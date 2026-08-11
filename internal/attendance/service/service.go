package service

import (
	"context"
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
	defaultWorkStartHour  = 9
	defaultWorkEndHour    = 18
)

type Service struct {
	repository repository.Repository
	logger     *slog.Logger
	now        func() time.Time
}

type Option func(*Service)

func New(repository repository.Repository, opts ...Option) *Service {
	service := &Service{
		repository: repository,
		logger:     slog.Default(),
		now:        time.Now,
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

	records, err := s.listDailyAttendanceRecords(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("service: list daily attendance records: %w", err)
	}

	return buildDailyAttendance(normalized, records), nil
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

	if query.Limit <= 0 {
		query.Limit = defaultQueryLimit
	}
	if query.Limit > maxQueryLimit {
		query.Limit = maxQueryLimit
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	return query, nil
}

func normalizeDailyAttendanceQuery(query domain.DailyAttendanceQuery, now time.Time) (domain.DailyAttendanceQuery, error) {
	query.UserID = strings.TrimSpace(query.UserID)
	query.DeviceSN = strings.TrimSpace(query.DeviceSN)

	switch {
	case query.StartDate.IsZero() && query.EndDate.IsZero():
		query.StartDate = startOfDay(now)
		query.EndDate = query.StartDate
	case query.StartDate.IsZero():
		query.EndDate = startOfDay(query.EndDate)
		query.StartDate = query.EndDate
	case query.EndDate.IsZero():
		query.StartDate = startOfDay(query.StartDate)
		query.EndDate = query.StartDate
	default:
		query.StartDate = startOfDay(query.StartDate)
		query.EndDate = startOfDay(query.EndDate)
	}

	if query.EndDate.Before(query.StartDate) {
		return domain.DailyAttendanceQuery{}, errors.New("service: end date must not be before start date")
	}
	if dateRangeDays(query.StartDate, query.EndDate) > maxDailyDateRangeDays {
		return domain.DailyAttendanceQuery{}, fmt.Errorf("service: date range cannot exceed %d days", maxDailyDateRangeDays)
	}

	if query.Limit <= 0 {
		query.Limit = defaultQueryLimit
	}
	if query.Limit > maxQueryLimit {
		query.Limit = maxQueryLimit
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	return query, nil
}

func (s *Service) listDailyAttendanceRecords(ctx context.Context, query domain.DailyAttendanceQuery) ([]domain.AttendanceRecord, error) {
	recordQuery := domain.AttendanceRecordQuery{
		UserID:    query.UserID,
		DeviceSN:  query.DeviceSN,
		StartTime: query.StartDate,
		EndTime:   endOfDay(query.EndDate),
		Limit:     maxQueryLimit,
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

type dailyAttendanceKey struct {
	date   string
	userID string
}

type dailyAttendanceAggregate struct {
	date          time.Time
	userID        string
	userName      string
	deviceSN      string
	workStartAt   time.Time
	workEndAt     time.Time
	firstEntryAt  time.Time
	lastExitAt    time.Time
	recordCount   int
	snapshotCount int
}

func buildDailyAttendance(query domain.DailyAttendanceQuery, records []domain.AttendanceRecord) []domain.DailyAttendance {
	aggregates := make(map[dailyAttendanceKey]*dailyAttendanceAggregate)

	if query.UserID != "" {
		for date := query.StartDate; !date.After(query.EndDate); date = date.AddDate(0, 0, 1) {
			key := dailyAttendanceKey{
				date:   dailyAttendanceDateKey(date),
				userID: query.UserID,
			}
			aggregates[key] = newDailyAttendanceAggregate(date, query.UserID, "", query.DeviceSN)
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
		if query.DeviceSN != "" && strings.TrimSpace(record.DeviceSN) != query.DeviceSN {
			continue
		}

		date := startOfDay(record.EventTime)
		if date.Before(query.StartDate) || date.After(query.EndDate) {
			continue
		}

		key := dailyAttendanceKey{
			date:   dailyAttendanceDateKey(date),
			userID: userID,
		}
		aggregate := aggregates[key]
		if aggregate == nil {
			aggregate = newDailyAttendanceAggregate(date, userID, record.CardName, record.DeviceSN)
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

	return paginateDailyAttendance(dailies, query.Limit, query.Offset)
}

func newDailyAttendanceAggregate(date time.Time, userID string, userName string, deviceSN string) *dailyAttendanceAggregate {
	date = startOfDay(date)

	return &dailyAttendanceAggregate{
		date:        date,
		userID:      userID,
		userName:    userName,
		deviceSN:    deviceSN,
		workStartAt: time.Date(date.Year(), date.Month(), date.Day(), defaultWorkStartHour, 0, 0, 0, date.Location()),
		workEndAt:   time.Date(date.Year(), date.Month(), date.Day(), defaultWorkEndHour, 0, 0, 0, date.Location()),
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
	if aggregate.firstEntryAt.After(aggregate.workStartAt) {
		lateDuration = aggregate.firstEntryAt.Sub(aggregate.workStartAt)
		exceptions = append(exceptions, domain.DailyAttendanceExceptionLate)
	}

	earlyLeaveDuration := time.Duration(0)
	if aggregate.lastExitAt.Before(aggregate.workEndAt) {
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

func dailyAttendanceDateKey(date time.Time) string {
	return startOfDay(date).Format("2006-01-02")
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
