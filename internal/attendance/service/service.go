package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/parser"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/repository"
)

const (
	defaultQueryLimit = 100
	maxQueryLimit     = 500

	maxDailyDateRangeDays = 31
)

type Service struct {
	repository repository.Repository
	logger     *slog.Logger
	now        func() time.Time
	rules      domain.AttendanceRules
}

type Option func(*Service)

func New(repository repository.Repository, opts ...Option) *Service {
	service := &Service{
		repository: repository,
		logger:     slog.Default(),
		now:        time.Now,
		rules:      domain.DefaultAttendanceRules(),
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

func WithLogger(logger *slog.Logger) Option {
	return func(service *Service) {
		if logger != nil {
			service.logger = logger
		}
	}
}

func WithNow(now func() time.Time) Option {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

func WithAttendanceRules(rules domain.AttendanceRules) Option {
	return func(service *Service) {
		service.rules = normalizeAttendanceRules(rules)
	}
}

func (s *Service) HandleDevicePayload(ctx context.Context, payload *parser.ParsedPayload) error {
	if payload == nil {
		return errors.New("service: payload is nil")
	}
	if s.repository == nil {
		return errors.New("service: repository is nil")
	}

	for index, envelope := range payload.Events {
		if err := s.handleEvent(ctx, envelope); err != nil {
			return fmt.Errorf("service: handle event %d: %w", index, err)
		}
	}

	return nil
}

func (s *Service) ListAttendanceRecords(ctx context.Context, query domain.AttendanceRecordQuery) ([]domain.AttendanceRecord, error) {
	if s.repository == nil {
		return nil, errors.New("service: repository is nil")
	}

	normalized, err := normalizeAttendanceRecordQuery(query)
	if err != nil {
		return nil, err
	}

	records, err := s.repository.ListAttendanceRecords(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("service: list attendance records: %w", err)
	}

	return records, nil
}

func (s *Service) ListDailyAttendance(ctx context.Context, query domain.DailyAttendanceQuery) ([]domain.DailyAttendance, error) {
	if s.repository == nil {
		return nil, errors.New("service: repository is nil")
	}

	normalized, err := normalizeDailyAttendanceQuery(query, s.now())
	if err != nil {
		return nil, err
	}

	dailies, err := s.listDailyAttendance(ctx, normalized)
	if err != nil {
		return nil, err
	}

	return paginateDailyAttendance(dailies, normalized.Limit, normalized.Offset), nil
}

func (s *Service) ListMonthlyAttendance(ctx context.Context, query domain.MonthlyAttendanceQuery) ([]domain.MonthlyAttendance, error) {
	if s.repository == nil {
		return nil, errors.New("service: repository is nil")
	}

	normalized := normalizeMonthlyAttendanceQuery(query, s.now())
	dailies, err := s.listDailyAttendance(ctx, domain.DailyAttendanceQuery{
		AttendancePersonFilter: normalized.AttendancePersonFilter,
		DateRangeFilter: domain.DateRangeFilter{
			StartDate: firstDayOfMonth(normalized.Month),
			EndDate:   lastDayOfMonth(normalized.Month),
		},
		Pagination: domain.Pagination{Limit: maxQueryLimit},
	})
	if err != nil {
		return nil, err
	}

	records := buildMonthlyAttendance(normalized, dailies)
	return paginateMonthlyAttendance(records, normalized.Limit, normalized.Offset), nil
}

func (s *Service) GetAttendanceSummary(ctx context.Context, query domain.AttendanceSummaryQuery) (domain.AttendanceSummary, error) {
	if s.repository == nil {
		return domain.AttendanceSummary{}, errors.New("service: repository is nil")
	}

	normalized, err := normalizeAttendanceSummaryQuery(query, s.now())
	if err != nil {
		return domain.AttendanceSummary{}, err
	}

	dailies, err := s.listDailyAttendance(ctx, domain.DailyAttendanceQuery{
		AttendancePersonFilter: normalized.AttendancePersonFilter,
		DateRangeFilter:        normalized.DateRangeFilter,
		Pagination:             domain.Pagination{Limit: maxQueryLimit},
	})
	if err != nil {
		return domain.AttendanceSummary{}, err
	}

	return buildAttendanceSummary(normalized, dailies), nil
}

func (s *Service) ListAttendanceExceptions(ctx context.Context, query domain.AttendanceExceptionQuery) ([]domain.DailyAttendance, error) {
	if s.repository == nil {
		return nil, errors.New("service: repository is nil")
	}

	normalized, err := normalizeAttendanceExceptionQuery(query, s.now())
	if err != nil {
		return nil, err
	}

	dailies, err := s.listDailyAttendance(ctx, domain.DailyAttendanceQuery{
		AttendancePersonFilter: normalized.AttendancePersonFilter,
		DateRangeFilter:        normalized.DateRangeFilter,
		Pagination:             domain.Pagination{Limit: maxQueryLimit},
	})
	if err != nil {
		return nil, err
	}

	exceptions := filterAttendanceExceptions(dailies)
	return paginateDailyAttendance(exceptions, normalized.Limit, normalized.Offset), nil
}

func (s *Service) handleEvent(ctx context.Context, envelope domain.EventEnvelope) error {
	switch envelope.Code {
	case domain.EventCodeAccessControl:
		return s.handleAccessControl(ctx, envelope)
	case domain.EventCodeDoorStatus:
		return s.handleDoorStatus(ctx, envelope)
	default:
		return fmt.Errorf("unsupported event code %q", envelope.Code)
	}
}

func normalizeAttendanceRecordQuery(query domain.AttendanceRecordQuery) (domain.AttendanceRecordQuery, error) {
	if !query.StartTime.IsZero() && !query.EndTime.IsZero() && query.EndTime.Before(query.StartTime) {
		return domain.AttendanceRecordQuery{}, errors.New("service: end time must not be before start time")
	}

	query.AttendancePersonFilter = normalizeAttendancePersonFilter(query.AttendancePersonFilter)
	query.Pagination = normalizePagination(query.Pagination)

	return query, nil
}

func normalizeDailyAttendanceQuery(query domain.DailyAttendanceQuery, now time.Time) (domain.DailyAttendanceQuery, error) {
	query.AttendancePersonFilter = normalizeAttendancePersonFilter(query.AttendancePersonFilter)

	switch {
	case query.StartDate.IsZero() && query.EndDate.IsZero():
		query.DateRangeFilter.StartDate = startOfDay(now)
		query.DateRangeFilter.EndDate = query.DateRangeFilter.StartDate
	case query.StartDate.IsZero():
		query.DateRangeFilter.EndDate = startOfDay(query.EndDate)
		query.DateRangeFilter.StartDate = query.DateRangeFilter.EndDate
	case query.EndDate.IsZero():
		query.DateRangeFilter.StartDate = startOfDay(query.StartDate)
		query.DateRangeFilter.EndDate = query.DateRangeFilter.StartDate
	default:
		query.DateRangeFilter.StartDate = startOfDay(query.StartDate)
		query.DateRangeFilter.EndDate = startOfDay(query.EndDate)
	}

	if query.EndDate.Before(query.StartDate) {
		return domain.DailyAttendanceQuery{}, errors.New("service: end date must not be before start date")
	}
	if dateRangeDays(query.StartDate, query.EndDate) > maxDailyDateRangeDays {
		return domain.DailyAttendanceQuery{}, fmt.Errorf("service: date range cannot exceed %d days", maxDailyDateRangeDays)
	}

	query.Pagination = normalizePagination(query.Pagination)

	return query, nil
}

func normalizeMonthlyAttendanceQuery(query domain.MonthlyAttendanceQuery, now time.Time) domain.MonthlyAttendanceQuery {
	query.AttendancePersonFilter = normalizeAttendancePersonFilter(query.AttendancePersonFilter)
	if query.Month.IsZero() {
		query.Month = firstDayOfMonth(now)
	} else {
		query.Month = firstDayOfMonth(query.Month)
	}
	query.Pagination = normalizePagination(query.Pagination)

	return query
}

func normalizeAttendanceSummaryQuery(query domain.AttendanceSummaryQuery, now time.Time) (domain.AttendanceSummaryQuery, error) {
	dailyQuery, err := normalizeDailyAttendanceQuery(domain.DailyAttendanceQuery{
		AttendancePersonFilter: query.AttendancePersonFilter,
		DateRangeFilter:        query.DateRangeFilter,
		Pagination:             domain.Pagination{Limit: maxQueryLimit},
	}, now)
	if err != nil {
		return domain.AttendanceSummaryQuery{}, err
	}

	return domain.AttendanceSummaryQuery{
		AttendancePersonFilter: dailyQuery.AttendancePersonFilter,
		DateRangeFilter:        dailyQuery.DateRangeFilter,
	}, nil
}

func normalizeAttendanceExceptionQuery(query domain.AttendanceExceptionQuery, now time.Time) (domain.AttendanceExceptionQuery, error) {
	dailyQuery, err := normalizeDailyAttendanceQuery(domain.DailyAttendanceQuery{
		AttendancePersonFilter: query.AttendancePersonFilter,
		DateRangeFilter:        query.DateRangeFilter,
		Pagination:             query.Pagination,
	}, now)
	if err != nil {
		return domain.AttendanceExceptionQuery{}, err
	}

	return domain.AttendanceExceptionQuery{
		AttendancePersonFilter: dailyQuery.AttendancePersonFilter,
		DateRangeFilter:        dailyQuery.DateRangeFilter,
		Pagination:             dailyQuery.Pagination,
	}, nil
}

func normalizeAttendancePersonFilter(filter domain.AttendancePersonFilter) domain.AttendancePersonFilter {
	filter.UserID = strings.TrimSpace(filter.UserID)
	filter.UserName = strings.TrimSpace(filter.UserName)
	filter.DeviceSN = strings.TrimSpace(filter.DeviceSN)

	return filter
}

func normalizePagination(pagination domain.Pagination) domain.Pagination {
	if pagination.Limit <= 0 {
		pagination.Limit = defaultQueryLimit
	}
	if pagination.Limit > maxQueryLimit {
		pagination.Limit = maxQueryLimit
	}
	if pagination.Offset < 0 {
		pagination.Offset = 0
	}

	return pagination
}

func (s *Service) listDailyAttendance(ctx context.Context, query domain.DailyAttendanceQuery) ([]domain.DailyAttendance, error) {
	records, err := s.listDailyAttendanceRecords(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("service: list daily attendance records: %w", err)
	}

	rules, err := s.loadAttendanceRules(ctx, query.StartDate, query.EndDate)
	if err != nil {
		return nil, err
	}

	dailies := buildDailyAttendance(query, records, rules)
	if err := s.applyMonthlyAttendanceResults(ctx, dailies, query); err != nil {
		return nil, err
	}

	return dailies, nil
}

func (s *Service) loadAttendanceRules(ctx context.Context, startDate time.Time, endDate time.Time) (domain.AttendanceRules, error) {
	settings, err := s.repository.GetAttendanceSettings(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return normalizeAttendanceRules(s.rules), nil
		}
		return domain.AttendanceRules{}, fmt.Errorf("service: get attendance settings: %w", err)
	}

	location, err := time.LoadLocation(settings.Timezone)
	if err != nil {
		return domain.AttendanceRules{}, fmt.Errorf("service: load attendance timezone %q: %w", settings.Timezone, err)
	}

	shifts, err := s.repository.ListAttendanceShifts(ctx, domain.AttendanceShiftQuery{})
	if err != nil {
		return domain.AttendanceRules{}, fmt.Errorf("service: list attendance shifts: %w", err)
	}
	calendarDays, err := s.repository.ListAttendanceCalendarDays(ctx, domain.AttendanceCalendarDayQuery{
		StartDate: startDate,
		EndDate:   endDate,
	})
	if err != nil {
		return domain.AttendanceRules{}, fmt.Errorf("service: list attendance calendar days: %w", err)
	}
	schedules, err := s.repository.ListAttendanceSchedules(ctx, domain.AttendanceScheduleQuery{
		StartDate: startDate,
		EndDate:   endDate,
	})
	if err != nil {
		return domain.AttendanceRules{}, fmt.Errorf("service: list attendance schedules: %w", err)
	}
	weeklySchedules, err := s.repository.ListAttendanceWeeklySchedules(ctx, domain.AttendanceWeeklyScheduleQuery{})
	if err != nil {
		return domain.AttendanceRules{}, fmt.Errorf("service: list attendance weekly schedules: %w", err)
	}

	rules := domain.AttendanceRules{
		Location:        location,
		DefaultShiftID:  settings.DefaultShiftID,
		WeekendDays:     weekdayMap(settings.WeekendDays),
		Holidays:        make(map[string]string),
		Workdays:        make(map[string]bool),
		CalendarDays:    make(map[string]domain.AttendanceCalendarDay, len(calendarDays)),
		Shifts:          make(map[string]domain.AttendanceShift, len(shifts)),
		Schedules:       make([]domain.AttendanceSchedule, 0, len(schedules)),
		WeeklySchedules: make([]domain.AttendanceWeeklySchedule, 0, len(weeklySchedules)),
	}
	for _, shift := range shifts {
		rules.Shifts[shift.ID] = shift
	}
	for _, day := range calendarDays {
		key := dailyAttendanceDateKey(day.Date)
		rules.CalendarDays[key] = day
		switch day.DayType {
		case domain.CalendarDayTypeWorkday:
			rules.Workdays[key] = true
		case domain.CalendarDayTypeHoliday, domain.CalendarDayTypeRestDay:
			rules.Holidays[key] = day.Name
		}
	}
	for _, schedule := range schedules {
		rules.Schedules = append(rules.Schedules, domain.AttendanceSchedule{
			UserID:   schedule.UserID,
			DeviceSN: schedule.DeviceSN,
			Date:     schedule.Date,
			ShiftID:  schedule.ShiftID,
			Rest:     schedule.Rest,
			Reason:   schedule.Reason,
		})
	}
	for _, schedule := range weeklySchedules {
		rules.WeeklySchedules = append(rules.WeeklySchedules, domain.AttendanceWeeklySchedule{
			UserID:   schedule.UserID,
			DeviceSN: schedule.DeviceSN,
			Weekday:  schedule.Weekday,
			ShiftID:  schedule.ShiftID,
			Rest:     schedule.Rest,
			Reason:   schedule.Reason,
		})
	}

	return normalizeAttendanceRules(rules), nil
}

func (s *Service) listDailyAttendanceRecords(ctx context.Context, query domain.DailyAttendanceQuery) ([]domain.AttendanceRecord, error) {
	recordQuery := domain.AttendanceRecordQuery{
		AttendancePersonFilter: query.AttendancePersonFilter,
		TimeRangeFilter: domain.TimeRangeFilter{
			StartTime: query.StartDate,
			EndTime:   endOfDay(query.EndDate).Add(24 * time.Hour),
		},
		Pagination: domain.Pagination{Limit: maxQueryLimit},
	}

	var records []domain.AttendanceRecord
	for {
		page, err := s.repository.ListAttendanceRecords(ctx, recordQuery)
		if err != nil {
			return nil, err
		}

		records = append(records, page...)
		if len(page) < recordQuery.Limit {
			break
		}

		recordQuery.Offset += recordQuery.Limit
	}

	return records, nil
}

func (s *Service) applyMonthlyAttendanceResults(ctx context.Context, dailies []domain.DailyAttendance, query domain.DailyAttendanceQuery) error {
	if len(dailies) == 0 {
		return nil
	}

	results, err := s.repository.ListMonthlyAttendanceResults(ctx, domain.MonthlyAttendanceResultQuery{
		AttendancePersonFilter: query.AttendancePersonFilter,
		DateRangeFilter: domain.DateRangeFilter{
			StartDate: query.StartDate,
			EndDate:   query.EndDate,
		},
	})
	if err != nil {
		return fmt.Errorf("service: list monthly attendance results: %w", err)
	}

	resultByKey := make(map[dailyAttendanceResultKey]domain.MonthlyAttendanceDailyResult, len(results))
	for _, result := range results {
		if !result.Corrected {
			continue
		}
		key := dailyAttendanceResultKey{
			date:     dailyAttendanceDateKey(result.Date),
			userID:   strings.TrimSpace(result.UserID),
			deviceSN: strings.TrimSpace(result.DeviceSN),
		}
		resultByKey[key] = result
	}
	for index := range dailies {
		key := dailyAttendanceResultKey{
			date:     dailyAttendanceDateKey(dailies[index].Date),
			userID:   strings.TrimSpace(dailies[index].UserID),
			deviceSN: strings.TrimSpace(dailies[index].DeviceSN),
		}
		result, ok := resultByKey[key]
		if !ok && key.deviceSN != "" {
			key.deviceSN = ""
			result, ok = resultByKey[key]
		}
		if ok {
			applyMonthlyResultToDailyAttendance(&dailies[index], result)
		}
	}

	return nil
}

func (s *Service) handleAccessControl(ctx context.Context, envelope domain.EventEnvelope) error {
	var event domain.AccessControlEvent
	if err := json.Unmarshal(envelope.Data, &event); err != nil {
		return fmt.Errorf("decode access control data: %w", err)
	}

	record := domain.AttendanceRecord{
		DeviceSN:   event.SN,
		UserID:     event.UserID,
		CardName:   event.CardName,
		CardNo:     event.CardNo,
		Method:     event.Method,
		Direction:  event.Type,
		Status:     event.Status,
		EventTime:  eventTime(event.CreateTime, event.RealUTC, event.UTC, s.now),
		CreateTime: event.CreateTime,
		UTC:        event.UTC,
		RealUTC:    event.RealUTC,
		DataSource: envelope.DataSource,
		Index:      envelope.Index,
		Door:       event.Door,
		ReaderID:   event.ReaderID,
		CardType:   event.CardType,
		UserType:   event.UserType,
		ErrorCode:  event.ErrorCode,
		BlockID:    event.BlockID,
		ImageCount: len(event.ImageInfo),
		RawEvent:   cloneBytes(envelope.Data),
		ReceivedAt: s.now(),
	}

	if err := s.repository.SaveAttendanceRecord(ctx, record); err != nil {
		return err
	}

	s.logger.InfoContext(
		ctx,
		"attendance record saved",
		"method", record.Method,
		"direction", record.Direction,
		"status", record.Status,
		"image_count", record.ImageCount,
	)

	return nil
}

func (s *Service) handleDoorStatus(ctx context.Context, envelope domain.EventEnvelope) error {
	var event domain.DoorStatusEvent
	if err := json.Unmarshal(envelope.Data, &event); err != nil {
		return fmt.Errorf("decode door status data: %w", err)
	}

	record := domain.DoorStatusRecord{
		DeviceSN:   event.SN,
		Status:     event.Status,
		EventTime:  eventTime(0, event.RealUTC, event.UTC, s.now),
		UTC:        event.UTC,
		RealUTC:    event.RealUTC,
		DataSource: envelope.DataSource,
		Index:      envelope.Index,
		RawEvent:   cloneBytes(envelope.Data),
		ReceivedAt: s.now(),
	}

	if err := s.repository.SaveDoorStatusRecord(ctx, record); err != nil {
		return err
	}

	s.logger.InfoContext(
		ctx,
		"door status record saved",
		"status", record.Status,
		"index", record.Index,
	)

	return nil
}

type monthlyAttendanceAggregate struct {
	month    time.Time
	userID   string
	userName string
	deviceSN string
	dailies  []domain.DailyAttendance
	stats    domain.AttendanceStats
}

type dailyAttendanceResultKey struct {
	date     string
	userID   string
	deviceSN string
}

type dailyAttendanceKey struct {
	date   string
	userID string
}

type dailyAttendanceAggregate struct {
	date             time.Time
	userID           string
	userName         string
	deviceSN         string
	shiftID          string
	shiftName        string
	isWorkday        bool
	nonWorkdayReason string
	workStartAt      time.Time
	workEndAt        time.Time
	lateGrace        time.Duration
	earlyLeaveGrace  time.Duration
	flexible         time.Duration
	firstEntryAt     time.Time
	lastExitAt       time.Time
	recordCount      int
	snapshotCount    int
}

func buildDailyAttendance(query domain.DailyAttendanceQuery, records []domain.AttendanceRecord, rules domain.AttendanceRules) []domain.DailyAttendance {
	rules = normalizeAttendanceRules(rules)
	startDate := attendanceDateInLocation(query.StartDate, rules.Location)
	endDate := attendanceDateInLocation(query.EndDate, rules.Location)
	aggregates := make(map[dailyAttendanceKey]*dailyAttendanceAggregate)

	if query.UserID != "" {
		for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
			key := dailyAttendanceKey{
				date:   dailyAttendanceDateKey(date),
				userID: query.UserID,
			}
			aggregates[key] = newDailyAttendanceAggregate(date, query.UserID, "", query.DeviceSN, resolveAttendanceDayRule(rules, date, query.UserID, query.DeviceSN))
		}
	}

	for _, record := range records {
		userID := strings.TrimSpace(record.UserID)
		if userID == "" || record.EventTime.IsZero() {
			continue
		}
		if query.UserID != "" && userID != query.UserID {
			continue
		}
		if query.UserName != "" && strings.TrimSpace(record.CardName) != query.UserName {
			continue
		}
		if query.DeviceSN != "" && strings.TrimSpace(record.DeviceSN) != query.DeviceSN {
			continue
		}

		date := attendanceDateForRecord(rules, startDate, endDate, record)
		if date.IsZero() {
			continue
		}

		key := dailyAttendanceKey{
			date:   dailyAttendanceDateKey(date),
			userID: userID,
		}
		aggregate := aggregates[key]
		if aggregate == nil {
			aggregate = newDailyAttendanceAggregate(date, userID, record.CardName, record.DeviceSN, resolveAttendanceDayRule(rules, date, userID, record.DeviceSN))
			aggregates[key] = aggregate
		}

		aggregate.addRecord(record)
	}

	dailies := make([]domain.DailyAttendance, 0, len(aggregates))
	for _, aggregate := range aggregates {
		dailies = append(dailies, aggregate.dailyAttendance())
	}

	sort.Slice(dailies, func(i, j int) bool {
		if !dailies[i].Date.Equal(dailies[j].Date) {
			return dailies[i].Date.After(dailies[j].Date)
		}

		return dailies[i].UserID < dailies[j].UserID
	})

	return dailies
}

func buildMonthlyAttendance(query domain.MonthlyAttendanceQuery, dailies []domain.DailyAttendance) []domain.MonthlyAttendance {
	aggregates := make(map[string]*monthlyAttendanceAggregate)

	for _, daily := range dailies {
		userID := strings.TrimSpace(daily.UserID)
		if userID == "" {
			continue
		}

		aggregate := aggregates[userID]
		if aggregate == nil {
			aggregate = &monthlyAttendanceAggregate{
				month:    firstDayOfMonth(query.Month),
				userID:   userID,
				userName: daily.UserName,
				deviceSN: daily.DeviceSN,
			}
			aggregates[userID] = aggregate
		}
		if aggregate.userName == "" {
			aggregate.userName = daily.UserName
		}
		if aggregate.deviceSN == "" {
			aggregate.deviceSN = daily.DeviceSN
		}
		aggregate.dailies = append(aggregate.dailies, daily)

		addDailyAttendanceStats(&aggregate.stats, daily)
	}

	records := make([]domain.MonthlyAttendance, 0, len(aggregates))
	for _, aggregate := range aggregates {
		records = append(records, domain.MonthlyAttendance{
			Month:    aggregate.month,
			UserID:   aggregate.userID,
			UserName: aggregate.userName,
			DeviceSN: aggregate.deviceSN,
			Days:     sortedMonthlyDailies(aggregate.dailies),
			Stats:    aggregate.stats,
		})
	}

	sort.Slice(records, func(i, j int) bool {
		return records[i].UserID < records[j].UserID
	})

	return records
}

func sortedMonthlyDailies(dailies []domain.DailyAttendance) []domain.DailyAttendance {
	records := append([]domain.DailyAttendance(nil), dailies...)
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date.Before(records[j].Date)
	})

	return records
}

func applyMonthlyResultToDailyAttendance(daily *domain.DailyAttendance, result domain.MonthlyAttendanceDailyResult) {
	if daily == nil {
		return
	}

	daily.Status = result.Status
	daily.Exceptions = append([]domain.DailyAttendanceException(nil), result.Exceptions...)
	daily.IsAbnormalOverride = boolPtr(result.IsAbnormal)
	daily.Corrected = result.Corrected
	daily.CorrectionStatus = result.CorrectionStatus
	daily.CorrectionType = result.CorrectionType
	daily.CorrectionReason = result.CorrectionReason
	daily.CorrectedAt = result.CorrectedAt
	if daily.UserName == "" {
		daily.UserName = result.UserName
	}
	if daily.DeviceSN == "" {
		daily.DeviceSN = result.DeviceSN
	}
	if daily.ShiftID == "" {
		daily.ShiftID = result.ShiftID
	}
	if daily.ShiftName == "" {
		daily.ShiftName = result.ShiftName
	}
	if !result.WorkStartAt.IsZero() {
		daily.WorkStartAt = result.WorkStartAt
	}
	if !result.WorkEndAt.IsZero() {
		daily.WorkEndAt = result.WorkEndAt
	}
	if !result.FirstEntryAt.IsZero() {
		daily.FirstEntryAt = result.FirstEntryAt
	}
	if !result.LastExitAt.IsZero() {
		daily.LastExitAt = result.LastExitAt
	}
	daily.LateDuration = result.LateDuration
	daily.EarlyLeaveDuration = result.EarlyLeaveDuration
}

func buildAttendanceSummary(query domain.AttendanceSummaryQuery, dailies []domain.DailyAttendance) domain.AttendanceSummary {
	users := make(map[string]struct{})
	summary := domain.AttendanceSummary{
		StartDate: query.StartDate,
		EndDate:   query.EndDate,
	}

	for _, daily := range dailies {
		if daily.UserID != "" {
			users[daily.UserID] = struct{}{}
		}
		addDailyAttendanceStats(&summary.Stats, daily)
	}

	summary.UserCount = len(users)
	return summary
}

func filterAttendanceExceptions(dailies []domain.DailyAttendance) []domain.DailyAttendance {
	exceptions := make([]domain.DailyAttendance, 0)
	for _, daily := range dailies {
		if daily.IsAbnormal() {
			exceptions = append(exceptions, daily)
		}
	}

	return exceptions
}

func addDailyAttendanceStats(stats *domain.AttendanceStats, daily domain.DailyAttendance) {
	stats.TotalDays++
	if daily.IsWorkday {
		stats.WorkDays++
	} else {
		stats.RestDays++
	}
	stats.RecordCount += daily.RecordCount
	stats.SnapshotCount += daily.SnapshotCount
	stats.TotalLateDuration += daily.LateDuration
	stats.TotalEarlyLeaveDuration += daily.EarlyLeaveDuration

	if daily.Status == domain.DailyAttendanceStatusNormal {
		stats.NormalDays++
	} else if daily.IsAbnormal() {
		stats.AbnormalDays++
	}

	if daily.Status == domain.DailyAttendanceStatusLateAndEarlyLeave {
		stats.LateAndEarlyLeaveDays++
	}
	if daily.HasException(domain.DailyAttendanceExceptionLate) {
		stats.LateDays++
	}
	if daily.HasException(domain.DailyAttendanceExceptionEarlyLeave) {
		stats.EarlyLeaveDays++
	}
	if daily.HasException(domain.DailyAttendanceExceptionMissingCheckIn) {
		stats.MissingCheckInDays++
	}
	if daily.HasException(domain.DailyAttendanceExceptionMissingCheckOut) {
		stats.MissingCheckOutDays++
	}
	if daily.HasException(domain.DailyAttendanceExceptionAbsent) {
		stats.AbsentDays++
	}
}

func normalizeAttendanceRules(rules domain.AttendanceRules) domain.AttendanceRules {
	defaultRules := domain.DefaultAttendanceRules()
	if rules.Location == nil {
		rules.Location = defaultRules.Location
	}
	if strings.TrimSpace(rules.DefaultShiftID) == "" {
		rules.DefaultShiftID = defaultRules.DefaultShiftID
	}
	if len(rules.Shifts) == 0 {
		rules.Shifts = defaultRules.Shifts
	}
	if rules.WeekendDays == nil {
		rules.WeekendDays = make(map[time.Weekday]bool)
	}
	if rules.Holidays == nil {
		rules.Holidays = make(map[string]string)
	}
	if rules.Workdays == nil {
		rules.Workdays = make(map[string]bool)
	}
	if rules.CalendarDays == nil {
		rules.CalendarDays = make(map[string]domain.AttendanceCalendarDay)
	}

	normalizedShifts := make(map[string]domain.AttendanceShift, len(rules.Shifts))
	for id, shift := range rules.Shifts {
		shift.ID = strings.TrimSpace(shift.ID)
		if shift.ID == "" {
			shift.ID = strings.TrimSpace(id)
		}
		if shift.ID == "" || !shift.Enabled {
			continue
		}
		if strings.TrimSpace(shift.Name) == "" {
			shift.Name = shift.ID
		}
		normalizedShifts[shift.ID] = shift
	}
	if len(normalizedShifts) == 0 {
		normalizedShifts = defaultRules.Shifts
	}
	rules.Shifts = normalizedShifts
	if _, ok := rules.Shifts[rules.DefaultShiftID]; !ok {
		for id := range rules.Shifts {
			rules.DefaultShiftID = id
			break
		}
	}

	return rules
}

func weekdayMap(weekdays []time.Weekday) map[time.Weekday]bool {
	values := make(map[time.Weekday]bool, len(weekdays))
	for _, weekday := range weekdays {
		values[weekday] = true
	}

	return values
}

func resolveAttendanceDayRule(rules domain.AttendanceRules, date time.Time, userID string, deviceSN string) domain.AttendanceDayRule {
	rules = normalizeAttendanceRules(rules)
	date = attendanceDateInLocation(date, rules.Location)
	userID = strings.TrimSpace(userID)
	deviceSN = strings.TrimSpace(deviceSN)

	if schedule, ok := resolveDateSchedule(rules.Schedules, date, userID, deviceSN); ok {
		return attendanceRuleFromSchedule(rules, date, schedule.ShiftID, schedule.Rest, schedule.Reason)
	}
	if schedule, ok := resolveWeeklySchedule(rules.WeeklySchedules, date, userID, deviceSN); ok {
		return attendanceRuleFromSchedule(rules, date, schedule.ShiftID, schedule.Rest, schedule.Reason)
	}

	dateKey := dailyAttendanceDateKey(date)
	if rules.Workdays[dateKey] {
		return domain.AttendanceDayRule{
			Date:      date,
			Shift:     resolveAttendanceShift(rules, rules.DefaultShiftID),
			IsWorkday: true,
		}
	}
	if reason, ok := rules.Holidays[dateKey]; ok {
		if reason == "" {
			reason = "holiday"
		}
		return domain.AttendanceDayRule{
			Date:             date,
			Shift:            resolveAttendanceShift(rules, rules.DefaultShiftID),
			IsWorkday:        false,
			NonWorkdayReason: reason,
		}
	}
	if rules.WeekendDays[date.Weekday()] {
		return domain.AttendanceDayRule{
			Date:             date,
			Shift:            resolveAttendanceShift(rules, rules.DefaultShiftID),
			IsWorkday:        false,
			NonWorkdayReason: "weekend",
		}
	}

	return domain.AttendanceDayRule{
		Date:      date,
		Shift:     resolveAttendanceShift(rules, rules.DefaultShiftID),
		IsWorkday: true,
	}
}

func attendanceDateForRecord(rules domain.AttendanceRules, startDate time.Time, endDate time.Time, record domain.AttendanceRecord) time.Time {
	rules = normalizeAttendanceRules(rules)
	startDate = attendanceDateInLocation(startDate, rules.Location)
	endDate = attendanceDateInLocation(endDate, rules.Location)
	eventTime := record.EventTime
	if rules.Location != nil {
		eventTime = eventTime.In(rules.Location)
	}
	eventDate := startOfDay(eventTime)
	previousDate := eventDate.AddDate(0, 0, -1)

	for _, date := range []time.Time{previousDate, eventDate} {
		if date.Before(startDate) || date.After(endDate) {
			continue
		}
		rule := resolveAttendanceDayRule(rules, date, record.UserID, record.DeviceSN)
		if !rule.IsWorkday {
			continue
		}
		workStartAt := rule.Shift.WorkStartAt(date)
		workEndAt := rule.Shift.WorkEndAt(date)
		if workEndAt.After(endOfDay(date)) && !eventTime.Before(workStartAt) && !eventTime.After(workEndAt) {
			return date
		}
	}

	if eventDate.Before(startDate) || eventDate.After(endDate) {
		return time.Time{}
	}

	return eventDate
}

func attendanceRuleFromSchedule(rules domain.AttendanceRules, date time.Time, shiftID string, rest bool, reason string) domain.AttendanceDayRule {
	if rest {
		if reason == "" {
			reason = "scheduled_rest"
		}
		return domain.AttendanceDayRule{
			Date:             date,
			Shift:            resolveAttendanceShift(rules, rules.DefaultShiftID),
			IsWorkday:        false,
			NonWorkdayReason: reason,
		}
	}

	return domain.AttendanceDayRule{
		Date:      date,
		Shift:     resolveAttendanceShift(rules, shiftID),
		IsWorkday: true,
	}
}

func resolveAttendanceShift(rules domain.AttendanceRules, shiftID string) domain.AttendanceShift {
	shiftID = strings.TrimSpace(shiftID)
	if shiftID != "" {
		if shift, ok := rules.Shifts[shiftID]; ok {
			return shift
		}
	}
	if shift, ok := rules.Shifts[rules.DefaultShiftID]; ok {
		return shift
	}

	return domain.DefaultAttendanceRules().Shifts[domain.DefaultAttendanceShiftID]
}

func resolveDateSchedule(schedules []domain.AttendanceSchedule, date time.Time, userID string, deviceSN string) (domain.AttendanceSchedule, bool) {
	var matched domain.AttendanceSchedule
	bestScore := -1
	dateKey := dailyAttendanceDateKey(date)
	for _, schedule := range schedules {
		if dailyAttendanceDateKey(schedule.Date) != dateKey {
			continue
		}
		score, ok := scheduleMatchScore(schedule.UserID, schedule.DeviceSN, userID, deviceSN)
		if ok && score > bestScore {
			matched = schedule
			bestScore = score
		}
	}

	return matched, bestScore >= 0
}

func resolveWeeklySchedule(schedules []domain.AttendanceWeeklySchedule, date time.Time, userID string, deviceSN string) (domain.AttendanceWeeklySchedule, bool) {
	var matched domain.AttendanceWeeklySchedule
	bestScore := -1
	for _, schedule := range schedules {
		if schedule.Weekday != date.Weekday() {
			continue
		}
		score, ok := scheduleMatchScore(schedule.UserID, schedule.DeviceSN, userID, deviceSN)
		if ok && score > bestScore {
			matched = schedule
			bestScore = score
		}
	}

	return matched, bestScore >= 0
}

func scheduleMatchScore(scheduleUserID string, scheduleDeviceSN string, userID string, deviceSN string) (int, bool) {
	score := 0
	scheduleUserID = strings.TrimSpace(scheduleUserID)
	scheduleDeviceSN = strings.TrimSpace(scheduleDeviceSN)
	if scheduleUserID != "" {
		if scheduleUserID != userID {
			return 0, false
		}
		score += 2
	}
	if scheduleDeviceSN != "" {
		if scheduleDeviceSN != deviceSN {
			return 0, false
		}
		score++
	}

	return score, true
}

func newDailyAttendanceAggregate(date time.Time, userID string, userName string, deviceSN string, rule domain.AttendanceDayRule) *dailyAttendanceAggregate {
	date = startOfDay(date)
	workStartAt := rule.Shift.WorkStartAt(date)
	workEndAt := rule.Shift.WorkEndAt(date)

	return &dailyAttendanceAggregate{
		date:             date,
		userID:           userID,
		userName:         userName,
		deviceSN:         deviceSN,
		shiftID:          rule.Shift.ID,
		shiftName:        rule.Shift.Name,
		isWorkday:        rule.IsWorkday,
		nonWorkdayReason: rule.NonWorkdayReason,
		workStartAt:      workStartAt,
		workEndAt:        workEndAt,
		lateGrace:        rule.Shift.LateGrace,
		earlyLeaveGrace:  rule.Shift.EarlyLeaveGrace,
		flexible:         rule.Shift.Flexible,
	}
}

func (a *dailyAttendanceAggregate) addRecord(record domain.AttendanceRecord) {
	if a.userName == "" {
		a.userName = strings.TrimSpace(record.CardName)
	}
	if a.deviceSN == "" {
		a.deviceSN = strings.TrimSpace(record.DeviceSN)
	}

	a.recordCount++
	a.snapshotCount += record.ImageCount

	if record.Status != 1 {
		return
	}

	switch record.Direction {
	case domain.AccessDirectionEntry:
		if a.firstEntryAt.IsZero() || record.EventTime.Before(a.firstEntryAt) {
			a.firstEntryAt = record.EventTime
		}
	case domain.AccessDirectionExit:
		if a.lastExitAt.IsZero() || record.EventTime.After(a.lastExitAt) {
			a.lastExitAt = record.EventTime
		}
	}
}

func (a *dailyAttendanceAggregate) dailyAttendance() domain.DailyAttendance {
	status, exceptions, lateDuration, earlyLeaveDuration := evaluateDailyAttendanceStatus(*a)

	return domain.DailyAttendance{
		Date:               a.date,
		UserID:             a.userID,
		UserName:           a.userName,
		DeviceSN:           a.deviceSN,
		ShiftID:            a.shiftID,
		ShiftName:          a.shiftName,
		IsWorkday:          a.isWorkday,
		NonWorkdayReason:   a.nonWorkdayReason,
		Status:             status,
		Exceptions:         exceptions,
		WorkStartAt:        a.workStartAt,
		WorkEndAt:          a.workEndAt,
		FirstEntryAt:       a.firstEntryAt,
		LastExitAt:         a.lastExitAt,
		LateDuration:       lateDuration,
		EarlyLeaveDuration: earlyLeaveDuration,
		RecordCount:        a.recordCount,
		SnapshotCount:      a.snapshotCount,
	}
}

func evaluateDailyAttendanceStatus(aggregate dailyAttendanceAggregate) (
	domain.DailyAttendanceStatus,
	[]domain.DailyAttendanceException,
	time.Duration,
	time.Duration,
) {
	if !aggregate.isWorkday {
		return domain.DailyAttendanceStatusRestDay, nil, 0, 0
	}

	if aggregate.firstEntryAt.IsZero() && aggregate.lastExitAt.IsZero() {
		return domain.DailyAttendanceStatusAbsent,
			[]domain.DailyAttendanceException{domain.DailyAttendanceExceptionAbsent},
			0,
			0
	}
	if aggregate.firstEntryAt.IsZero() {
		return domain.DailyAttendanceStatusMissingCheckIn,
			[]domain.DailyAttendanceException{domain.DailyAttendanceExceptionMissingCheckIn},
			0,
			0
	}
	if aggregate.lastExitAt.IsZero() {
		return domain.DailyAttendanceStatusMissingCheckOut,
			[]domain.DailyAttendanceException{domain.DailyAttendanceExceptionMissingCheckOut},
			0,
			0
	}

	exceptions := make([]domain.DailyAttendanceException, 0, 2)
	lateDuration := time.Duration(0)
	latestStart := aggregate.workStartAt.Add(aggregate.lateGrace).Add(aggregate.flexible)
	if aggregate.firstEntryAt.After(latestStart) {
		lateDuration = aggregate.firstEntryAt.Sub(aggregate.workStartAt)
		exceptions = append(exceptions, domain.DailyAttendanceExceptionLate)
	}

	earlyLeaveDuration := time.Duration(0)
	earliestEnd := aggregate.workEndAt.Add(-aggregate.earlyLeaveGrace)
	if aggregate.lastExitAt.Before(earliestEnd) {
		earlyLeaveDuration = aggregate.workEndAt.Sub(aggregate.lastExitAt)
		exceptions = append(exceptions, domain.DailyAttendanceExceptionEarlyLeave)
	}

	switch {
	case lateDuration > 0 && earlyLeaveDuration > 0:
		return domain.DailyAttendanceStatusLateAndEarlyLeave, exceptions, lateDuration, earlyLeaveDuration
	case lateDuration > 0:
		return domain.DailyAttendanceStatusLate, exceptions, lateDuration, earlyLeaveDuration
	case earlyLeaveDuration > 0:
		return domain.DailyAttendanceStatusEarlyLeave, exceptions, lateDuration, earlyLeaveDuration
	default:
		return domain.DailyAttendanceStatusNormal, nil, 0, 0
	}
}

func paginateDailyAttendance(dailies []domain.DailyAttendance, limit int, offset int) []domain.DailyAttendance {
	if offset >= len(dailies) {
		return []domain.DailyAttendance{}
	}

	end := offset + limit
	if end > len(dailies) {
		end = len(dailies)
	}

	return dailies[offset:end]
}

func paginateMonthlyAttendance(records []domain.MonthlyAttendance, limit int, offset int) []domain.MonthlyAttendance {
	if offset >= len(records) {
		return []domain.MonthlyAttendance{}
	}

	end := offset + limit
	if end > len(records) {
		end = len(records)
	}

	return records[offset:end]
}

func dailyAttendanceDateKey(date time.Time) string {
	return startOfDay(date).Format("2006-01-02")
}

func attendanceDateInLocation(date time.Time, location *time.Location) time.Time {
	if date.IsZero() {
		return time.Time{}
	}
	if location == nil {
		return startOfDay(date)
	}

	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
}

func firstDayOfMonth(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}

	value = value.In(value.Location())
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
}

func lastDayOfMonth(value time.Time) time.Time {
	return firstDayOfMonth(value).AddDate(0, 1, -1)
}

func startOfDay(value time.Time) time.Time {
	if value.IsZero() {
		return time.Time{}
	}

	value = value.In(value.Location())
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}

func endOfDay(value time.Time) time.Time {
	return startOfDay(value).AddDate(0, 0, 1).Add(-time.Nanosecond)
}

func dateRangeDays(start time.Time, end time.Time) int {
	days := 0
	for date := startOfDay(start); !date.After(end); date = date.AddDate(0, 0, 1) {
		days++
	}

	return days
}

func boolPtr(value bool) *bool {
	return &value
}

func eventTime(createTime int64, realUTC int64, utc int64, now func() time.Time) time.Time {
	for _, unixSeconds := range []int64{createTime, realUTC, utc} {
		if unixSeconds > 0 {
			return time.Unix(unixSeconds, 0)
		}
	}

	return now()
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	cloned := make([]byte, len(data))
	copy(cloned, data)
	return cloned
}
