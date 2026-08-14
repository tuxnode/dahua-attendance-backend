package service_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/parser"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/service"
)

type fakeRepository struct {
	attendanceRecords []domain.AttendanceRecord
	doorStatusRecords []domain.DoorStatusRecord
	settings          domain.AttendanceSettings
	shifts            []domain.AttendanceShift
	calendarDays      []domain.AttendanceCalendarDay
	schedules         []domain.ManagedAttendanceSchedule
	weeklySchedules   []domain.ManagedAttendanceWeeklySchedule
	corrections       []domain.AttendanceCorrection
	monthlyResults    []domain.MonthlyAttendanceDailyResult
	query             domain.AttendanceRecordQuery
	calendarDayQuery  domain.AttendanceCalendarDayQuery
	scheduleQuery     domain.AttendanceScheduleQuery
	err               error
	settingsErr       error
}

func (r *fakeRepository) SaveAttendanceRecord(_ context.Context, record domain.AttendanceRecord) error {
	if r.err != nil {
		return r.err
	}
	r.attendanceRecords = append(r.attendanceRecords, record)
	return nil
}

func (r *fakeRepository) SaveDoorStatusRecord(_ context.Context, record domain.DoorStatusRecord) error {
	if r.err != nil {
		return r.err
	}
	r.doorStatusRecords = append(r.doorStatusRecords, record)
	return nil
}

func (r *fakeRepository) ListAttendanceRecords(_ context.Context, query domain.AttendanceRecordQuery) ([]domain.AttendanceRecord, error) {
	if r.err != nil {
		return nil, r.err
	}
	r.query = query
	return append([]domain.AttendanceRecord(nil), r.attendanceRecords...), nil
}

func (r *fakeRepository) GetAttendanceSettings(_ context.Context) (domain.AttendanceSettings, error) {
	if r.settingsErr != nil {
		return domain.AttendanceSettings{}, r.settingsErr
	}
	if r.settings.Timezone == "" && r.settings.DefaultShiftID == "" && len(r.settings.WeekendDays) == 0 && r.settings.SettlementDay == 0 {
		return domain.AttendanceSettings{}, sql.ErrNoRows
	}
	return r.settings, nil
}

func (r *fakeRepository) SaveAttendanceSettings(_ context.Context, _ domain.AttendanceSettings) error {
	return nil
}

func (r *fakeRepository) ListAttendanceShifts(_ context.Context, _ domain.AttendanceShiftQuery) ([]domain.AttendanceShift, error) {
	return append([]domain.AttendanceShift(nil), r.shifts...), nil
}

func (r *fakeRepository) SaveAttendanceShift(_ context.Context, _ domain.AttendanceShift) error {
	return nil
}

func (r *fakeRepository) DeleteAttendanceShift(_ context.Context, _ string) error {
	return nil
}

func (r *fakeRepository) ListAttendanceCalendarDays(_ context.Context, query domain.AttendanceCalendarDayQuery) ([]domain.AttendanceCalendarDay, error) {
	r.calendarDayQuery = query
	return append([]domain.AttendanceCalendarDay(nil), r.calendarDays...), nil
}

func (r *fakeRepository) SaveAttendanceCalendarDay(_ context.Context, _ domain.AttendanceCalendarDay) error {
	return nil
}

func (r *fakeRepository) DeleteAttendanceCalendarDay(_ context.Context, _ time.Time) error {
	return nil
}

func (r *fakeRepository) ListAttendanceSchedules(_ context.Context, query domain.AttendanceScheduleQuery) ([]domain.ManagedAttendanceSchedule, error) {
	r.scheduleQuery = query
	return append([]domain.ManagedAttendanceSchedule(nil), r.schedules...), nil
}

func (r *fakeRepository) SaveAttendanceSchedule(_ context.Context, schedule domain.ManagedAttendanceSchedule) (domain.ManagedAttendanceSchedule, error) {
	return schedule, nil
}

func (r *fakeRepository) DeleteAttendanceSchedule(_ context.Context, _ int64) error {
	return nil
}

func (r *fakeRepository) ListAttendanceWeeklySchedules(_ context.Context, _ domain.AttendanceWeeklyScheduleQuery) ([]domain.ManagedAttendanceWeeklySchedule, error) {
	return append([]domain.ManagedAttendanceWeeklySchedule(nil), r.weeklySchedules...), nil
}

func (r *fakeRepository) SaveAttendanceWeeklySchedule(_ context.Context, schedule domain.ManagedAttendanceWeeklySchedule) (domain.ManagedAttendanceWeeklySchedule, error) {
	return schedule, nil
}

func (r *fakeRepository) DeleteAttendanceWeeklySchedule(_ context.Context, _ int64) error {
	return nil
}

func (r *fakeRepository) SaveAttendanceCorrection(_ context.Context, correction domain.AttendanceCorrection) (domain.AttendanceCorrection, error) {
	if r.err != nil {
		return domain.AttendanceCorrection{}, r.err
	}
	if correction.ID == 0 {
		correction.ID = int64(len(r.corrections) + 1)
	}
	r.corrections = append(r.corrections, correction)
	return correction, nil
}

func (r *fakeRepository) SaveMonthlyAttendanceResult(_ context.Context, result domain.MonthlyAttendanceDailyResult) (domain.MonthlyAttendanceDailyResult, error) {
	if r.err != nil {
		return domain.MonthlyAttendanceDailyResult{}, r.err
	}
	if result.ID == 0 {
		result.ID = int64(len(r.monthlyResults) + 1)
	}
	r.monthlyResults = append(r.monthlyResults, result)
	return result, nil
}

func (r *fakeRepository) ListMonthlyAttendanceResults(_ context.Context, query domain.MonthlyAttendanceResultQuery) ([]domain.MonthlyAttendanceDailyResult, error) {
	if r.err != nil {
		return nil, r.err
	}

	results := make([]domain.MonthlyAttendanceDailyResult, 0, len(r.monthlyResults))
	for _, result := range r.monthlyResults {
		if !query.StartDate.IsZero() && result.Date.Before(query.StartDate) {
			continue
		}
		if !query.EndDate.IsZero() && result.Date.After(query.EndDate) {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

func TestHandleDevicePayloadWritesAttendanceRecord(t *testing.T) {
	repo := &fakeRepository{}
	now := fixedNow()
	svc := service.New(repo, service.WithLogger(discardLogger()), service.WithNow(func() time.Time {
		return now
	}))

	err := svc.HandleDevicePayload(context.Background(), &parser.ParsedPayload{
		Events: []domain.EventEnvelope{
			accessControlEnvelope(t),
		},
	})
	if err != nil {
		t.Fatalf("handle payload: %v", err)
	}

	if len(repo.attendanceRecords) != 1 {
		t.Fatalf("unexpected attendance records length: %d", len(repo.attendanceRecords))
	}

	record := repo.attendanceRecords[0]
	if record.DeviceSN != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn: %s", record.DeviceSN)
	}
	if record.UserID != "REDACTED_USER_ID" {
		t.Fatalf("unexpected user id: %s", record.UserID)
	}
	if record.Method != domain.AccessMethodFaceOpen {
		t.Fatalf("unexpected method: %s", record.Method)
	}
	if record.Direction != domain.AccessDirectionEntry {
		t.Fatalf("unexpected direction: %s", record.Direction)
	}
	if record.DataSource != domain.DataSourceOffline {
		t.Fatalf("unexpected data source: %s", record.DataSource)
	}
	if record.ImageCount != 1 {
		t.Fatalf("unexpected image count: %d", record.ImageCount)
	}
	if record.EventTime.Unix() != 1700000000 {
		t.Fatalf("unexpected event time: %s", record.EventTime)
	}
	if record.ReceivedAt != now {
		t.Fatalf("unexpected received at: %s", record.ReceivedAt)
	}
}

func TestHandleDevicePayloadWritesDoorStatusRecord(t *testing.T) {
	repo := &fakeRepository{}
	svc := service.New(repo, service.WithLogger(discardLogger()), service.WithNow(fixedNow))

	err := svc.HandleDevicePayload(context.Background(), &parser.ParsedPayload{
		Events: []domain.EventEnvelope{
			doorStatusEnvelope(t),
		},
	})
	if err != nil {
		t.Fatalf("handle payload: %v", err)
	}

	if len(repo.doorStatusRecords) != 1 {
		t.Fatalf("unexpected door status records length: %d", len(repo.doorStatusRecords))
	}

	record := repo.doorStatusRecords[0]
	if record.DeviceSN != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn: %s", record.DeviceSN)
	}
	if record.Status != domain.DoorStateOpen {
		t.Fatalf("unexpected status: %s", record.Status)
	}
	if record.EventTime.Unix() != 1700000120 {
		t.Fatalf("unexpected event time: %s", record.EventTime)
	}
}

func TestHandleDevicePayloadReturnsRepositoryError(t *testing.T) {
	svc := service.New(
		&fakeRepository{err: errors.New("write failed")},
		service.WithLogger(discardLogger()),
		service.WithNow(fixedNow),
	)

	err := svc.HandleDevicePayload(context.Background(), &parser.ParsedPayload{
		Events: []domain.EventEnvelope{
			accessControlEnvelope(t),
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHandleDevicePayloadLogsDoNotIncludeSensitiveFields(t *testing.T) {
	repo := &fakeRepository{}
	var logs bytes.Buffer

	svc := service.New(
		repo,
		service.WithLogger(slog.New(slog.NewJSONHandler(&logs, nil))),
		service.WithNow(fixedNow),
	)

	err := svc.HandleDevicePayload(context.Background(), &parser.ParsedPayload{
		Events: []domain.EventEnvelope{
			accessControlEnvelope(t),
			doorStatusEnvelope(t),
		},
	})
	if err != nil {
		t.Fatalf("handle payload: %v", err)
	}

	output := logs.String()
	for _, forbidden := range []string{
		"REDACTED_DEVICE_SN",
		"REDACTED_USER_ID",
		"REDACTED_NAME",
	} {
		if bytes.Contains([]byte(output), []byte(forbidden)) {
			t.Fatalf("log output contains sensitive field %q: %s", forbidden, output)
		}
	}
}

func TestHandleDevicePayloadRejectsNilPayload(t *testing.T) {
	svc := service.New(&fakeRepository{}, service.WithLogger(discardLogger()))

	err := svc.HandleDevicePayload(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSaveAttendanceSettingsRejectsInvalidSettlementDay(t *testing.T) {
	svc := service.New(&fakeRepository{}, service.WithLogger(discardLogger()))

	_, err := svc.SaveAttendanceSettings(context.Background(), domain.AttendanceSettings{
		Timezone:       "Asia/Shanghai",
		DefaultShiftID: "day",
		WeekendDays:    []time.Weekday{time.Saturday, time.Sunday},
		SettlementDay:  29,
	})
	if err == nil {
		t.Fatal("expected invalid settlement day error")
	}
}

func TestListAttendanceRecordsNormalizesPagination(t *testing.T) {
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			{UserID: "REDACTED_USER_ID"},
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	records, err := svc.ListAttendanceRecords(context.Background(), domain.AttendanceRecordQuery{
		Pagination: domain.Pagination{Limit: -1, Offset: -1},
	})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("unexpected records length: %d", len(records))
	}
	if repo.query.Limit != 100 {
		t.Fatalf("unexpected limit: %d", repo.query.Limit)
	}
	if repo.query.Offset != 0 {
		t.Fatalf("unexpected offset: %d", repo.query.Offset)
	}
}

func TestListAttendanceRecordsClampsLargeLimit(t *testing.T) {
	repo := &fakeRepository{}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	_, err := svc.ListAttendanceRecords(context.Background(), domain.AttendanceRecordQuery{Pagination: domain.Pagination{Limit: 1000}})
	if err != nil {
		t.Fatalf("list records: %v", err)
	}

	if repo.query.Limit != 500 {
		t.Fatalf("unexpected limit: %d", repo.query.Limit)
	}
}

func TestListAttendanceRecordsRejectsInvalidTimeRange(t *testing.T) {
	svc := service.New(&fakeRepository{}, service.WithLogger(discardLogger()))

	_, err := svc.ListAttendanceRecords(context.Background(), domain.AttendanceRecordQuery{
		TimeRangeFilter: domain.TimeRangeFilter{StartTime: fixedNow(), EndTime: fixedNow().Add(-time.Second)},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListAttendanceRecordsReturnsRepositoryError(t *testing.T) {
	svc := service.New(&fakeRepository{err: errors.New("query failed")}, service.WithLogger(discardLogger()))

	_, err := svc.ListAttendanceRecords(context.Background(), domain.AttendanceRecordQuery{Pagination: domain.Pagination{Limit: 10}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListDailyAttendanceReturnsNormal(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(8*time.Hour+50*time.Minute), domain.AccessDirectionEntry),
			{
				UserID:     "REDACTED_USER_ID",
				CardName:   "REDACTED_NAME",
				DeviceSN:   "REDACTED_DEVICE_SN",
				Direction:  domain.AccessDirectionExit,
				Status:     1,
				EventTime:  date.Add(18*time.Hour + 10*time.Minute),
				ImageCount: 1,
			},
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 1 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}

	daily := dailies[0]
	if daily.Status != domain.DailyAttendanceStatusNormal {
		t.Fatalf("unexpected status: %s", daily.Status)
	}
	if !daily.FirstEntryAt.Equal(date.Add(8*time.Hour + 50*time.Minute)) {
		t.Fatalf("unexpected first entry: %s", daily.FirstEntryAt)
	}
	if !daily.LastExitAt.Equal(date.Add(18*time.Hour + 10*time.Minute)) {
		t.Fatalf("unexpected last exit: %s", daily.LastExitAt)
	}
	if daily.RecordCount != 2 {
		t.Fatalf("unexpected record count: %d", daily.RecordCount)
	}
	if daily.SnapshotCount != 1 {
		t.Fatalf("unexpected snapshot count: %d", daily.SnapshotCount)
	}
	if repo.query.StartTime != date {
		t.Fatalf("unexpected repository start time: %s", repo.query.StartTime)
	}
	if !repo.query.EndTime.Equal(date.AddDate(0, 0, 2).Add(-time.Nanosecond)) {
		t.Fatalf("unexpected repository end time: %s", repo.query.EndTime)
	}
}

func TestListDailyAttendanceReturnsLateAndIgnoresEarlyExit(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(9*time.Hour+10*time.Minute), domain.AccessDirectionEntry),
			dailyRecord(date.Add(17*time.Hour+30*time.Minute), domain.AccessDirectionExit),
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 1 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}

	daily := dailies[0]
	if daily.Status != domain.DailyAttendanceStatusLate {
		t.Fatalf("unexpected status: %s", daily.Status)
	}
	if daily.LateDuration != 10*time.Minute {
		t.Fatalf("unexpected late duration: %s", daily.LateDuration)
	}
	if daily.EarlyLeaveDuration != 0 {
		t.Fatalf("unexpected early leave duration: %s", daily.EarlyLeaveDuration)
	}
	if !daily.HasException(domain.DailyAttendanceExceptionLate) {
		t.Fatal("expected late exception")
	}
	if daily.HasException(domain.DailyAttendanceExceptionEarlyLeave) {
		t.Fatal("did not expect early leave exception")
	}
}

func TestListDailyAttendanceReturnsNormalWithEntryOnly(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(8*time.Hour+55*time.Minute), domain.AccessDirectionEntry),
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 1 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}
	if dailies[0].Status != domain.DailyAttendanceStatusNormal {
		t.Fatalf("unexpected status: %s", dailies[0].Status)
	}
	if dailies[0].HasException(domain.DailyAttendanceExceptionMissingCheckOut) {
		t.Fatal("did not expect missing check-out exception")
	}
}

func TestListDailyAttendanceReturnsAbsentWithoutEntry(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(18*time.Hour), domain.AccessDirectionExit),
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 1 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}
	if dailies[0].Status != domain.DailyAttendanceStatusAbsent {
		t.Fatalf("unexpected status: %s", dailies[0].Status)
	}
	if !dailies[0].HasException(domain.DailyAttendanceExceptionAbsent) {
		t.Fatal("expected absent exception")
	}
	if dailies[0].LastExitAt.IsZero() {
		t.Fatal("expected raw exit time to remain available")
	}
}

func TestListDailyAttendanceReturnsAbsentForQueriedUser(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	svc := service.New(&fakeRepository{}, service.WithLogger(discardLogger()))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 1 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}
	if dailies[0].Status != domain.DailyAttendanceStatusAbsent {
		t.Fatalf("unexpected status: %s", dailies[0].Status)
	}
	if !dailies[0].HasException(domain.DailyAttendanceExceptionAbsent) {
		t.Fatal("expected absent exception")
	}
}

func TestListDailyAttendanceSkipsUserlessRecords(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			{
				DeviceSN:  "REDACTED_DEVICE_SN",
				Direction: domain.AccessDirectionEntry,
				Status:    1,
				EventTime: date.Add(9 * time.Hour),
			},
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		DateRangeFilter: domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 0 {
		t.Fatalf("expected no daily attendance, got %d", len(dailies))
	}
}

func TestListDailyAttendanceRejectsInvalidQuery(t *testing.T) {
	svc := service.New(&fakeRepository{}, service.WithLogger(discardLogger()))
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

	_, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		DateRangeFilter: domain.DateRangeFilter{
			StartDate: start,
			EndDate:   start.AddDate(0, 0, -1),
		},
	})
	if err == nil {
		t.Fatal("expected invalid date range error")
	}

	_, err = svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		DateRangeFilter: domain.DateRangeFilter{
			StartDate: start,
			EndDate:   start.AddDate(0, 0, 31),
		},
	})
	if err == nil {
		t.Fatal("expected oversized date range error")
	}
}

func TestListDailyAttendanceUsesWeekendHolidayAndWorkdayRules(t *testing.T) {
	start := time.Date(2026, 8, 8, 0, 0, 0, 0, time.UTC)
	rules := testAttendanceRules()
	rules.WeekendDays = map[time.Weekday]bool{
		time.Saturday: true,
		time.Sunday:   true,
	}
	rules.Workdays = map[string]bool{
		"2026-08-08": true,
	}
	rules.Holidays = map[string]string{
		"2026-08-10": "holiday",
	}

	svc := service.New(&fakeRepository{}, service.WithLogger(discardLogger()), service.WithAttendanceRules(rules))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: start, EndDate: start.AddDate(0, 0, 2)},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 3 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}
	if dailies[0].Status != domain.DailyAttendanceStatusRestDay || dailies[0].NonWorkdayReason != "holiday" {
		t.Fatalf("unexpected holiday daily: %+v", dailies[0])
	}
	if dailies[1].Status != domain.DailyAttendanceStatusRestDay || dailies[1].NonWorkdayReason != "weekend" {
		t.Fatalf("unexpected weekend daily: %+v", dailies[1])
	}
	if dailies[2].Status != domain.DailyAttendanceStatusAbsent || !dailies[2].IsWorkday {
		t.Fatalf("unexpected workday daily: %+v", dailies[2])
	}
}

func TestListDailyAttendanceUsesRepositoryRules(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		settings: domain.AttendanceSettings{
			Timezone:       "UTC",
			DefaultShiftID: "day",
			WeekendDays:    []time.Weekday{time.Saturday, time.Sunday},
		},
		shifts: []domain.AttendanceShift{
			{
				ID:      "day",
				Name:    "Day Shift",
				Start:   domain.ClockTime{Hour: 9},
				End:     domain.ClockTime{Hour: 18},
				Enabled: true,
			},
			{
				ID:      "night",
				Name:    "Night Shift",
				Start:   domain.ClockTime{Hour: 21},
				End:     domain.ClockTime{Hour: 6},
				Enabled: true,
			},
		},
		calendarDays: []domain.AttendanceCalendarDay{
			{
				Date:    date.AddDate(0, 0, 1),
				DayType: domain.CalendarDayTypeHoliday,
				Name:    "company_holiday",
			},
		},
		schedules: []domain.ManagedAttendanceSchedule{
			{
				UserID:  "REDACTED_USER_ID",
				Date:    date,
				ShiftID: "night",
				Enabled: true,
			},
		},
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(21*time.Hour), domain.AccessDirectionEntry),
			dailyRecord(date.AddDate(0, 0, 1).Add(6*time.Hour), domain.AccessDirectionExit),
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date.AddDate(0, 0, 1)},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 2 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}
	if repo.calendarDayQuery.StartDate.Format("2006-01-02") != "2026-08-10" ||
		repo.calendarDayQuery.EndDate.Format("2006-01-02") != "2026-08-11" {
		t.Fatalf("unexpected calendar day query: %+v", repo.calendarDayQuery)
	}
	if repo.scheduleQuery.StartDate.Format("2006-01-02") != "2026-08-10" ||
		repo.scheduleQuery.EndDate.Format("2006-01-02") != "2026-08-11" {
		t.Fatalf("unexpected schedule query: %+v", repo.scheduleQuery)
	}
	if dailies[0].Date.Format("2006-01-02") != "2026-08-11" ||
		dailies[0].Status != domain.DailyAttendanceStatusRestDay ||
		dailies[0].NonWorkdayReason != "company_holiday" {
		t.Fatalf("unexpected holiday daily: %+v", dailies[0])
	}
	if dailies[1].Date.Format("2006-01-02") != "2026-08-10" ||
		dailies[1].ShiftID != "night" ||
		dailies[1].Status != domain.DailyAttendanceStatusNormal {
		t.Fatalf("unexpected scheduled daily: %+v", dailies[1])
	}
}

func TestListDailyAttendanceUsesFlexibleAndGraceMinutes(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	rules := testAttendanceRules()
	shift := rules.Shifts["day"]
	shift.LateGrace = 5 * time.Minute
	shift.EarlyLeaveGrace = 5 * time.Minute
	shift.Flexible = 10 * time.Minute
	rules.Shifts["day"] = shift

	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(9*time.Hour+14*time.Minute), domain.AccessDirectionEntry),
			dailyRecord(date.Add(17*time.Hour+56*time.Minute), domain.AccessDirectionExit),
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()), service.WithAttendanceRules(rules))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 1 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}
	if dailies[0].Status != domain.DailyAttendanceStatusNormal {
		t.Fatalf("unexpected status: %+v", dailies[0])
	}
}

func TestSaveAttendanceCorrectionMarksMonthlyResultCorrected(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	now := date.Add(24 * time.Hour)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(9*time.Hour), domain.AccessDirectionEntry),
		},
	}
	svc := service.New(
		repo,
		service.WithLogger(discardLogger()),
		service.WithNow(func() time.Time { return now }),
		service.WithAttendanceRules(testAttendanceRules()),
	)

	correction, err := svc.SaveAttendanceCorrection(context.Background(), domain.AttendanceCorrection{
		UserID:      "REDACTED_USER_ID",
		DeviceSN:    "REDACTED_DEVICE_SN",
		Date:        date,
		Type:        domain.AttendanceCorrectionTypeCheckOut,
		CorrectedAt: date.Add(18 * time.Hour),
		Reason:      "forgot check out",
	})
	if err != nil {
		t.Fatalf("save correction: %v", err)
	}
	if correction.ID == 0 {
		t.Fatal("expected correction id")
	}
	if len(repo.corrections) != 1 {
		t.Fatalf("unexpected corrections length: %d", len(repo.corrections))
	}
	if len(repo.monthlyResults) != 1 {
		t.Fatalf("unexpected monthly results length: %d", len(repo.monthlyResults))
	}

	result := repo.monthlyResults[0]
	if result.Status != domain.DailyAttendanceStatusCorrected {
		t.Fatalf("unexpected result status: %s", result.Status)
	}
	if result.IsAbnormal {
		t.Fatal("expected result to be non-abnormal after correction")
	}
	if !result.Corrected {
		t.Fatal("expected result to be corrected")
	}
	if result.CorrectionType != domain.AttendanceCorrectionTypeCheckOut {
		t.Fatalf("unexpected correction type: %s", result.CorrectionType)
	}
	if result.CorrectionReason != "forgot check out" {
		t.Fatalf("unexpected correction reason: %s", result.CorrectionReason)
	}
	if result.LastExitAt.Unix() != date.Add(18*time.Hour).Unix() {
		t.Fatalf("unexpected last exit: %s", result.LastExitAt)
	}

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 1 {
		t.Fatalf("unexpected dailies length: %d", len(dailies))
	}
	if dailies[0].Status != domain.DailyAttendanceStatusCorrected || dailies[0].IsAbnormal() {
		t.Fatalf("unexpected corrected daily: %+v", dailies[0])
	}
	if !dailies[0].Corrected {
		t.Fatalf("expected corrected daily: %+v", dailies[0])
	}
	if dailies[0].CorrectionType != domain.AttendanceCorrectionTypeCheckOut {
		t.Fatalf("unexpected daily correction type: %s", dailies[0].CorrectionType)
	}
}

func TestSaveAttendanceCorrectionSupportsLeaveAndBusinessTrip(t *testing.T) {
	for _, correctionType := range []domain.AttendanceCorrectionType{
		domain.AttendanceCorrectionTypeLeave,
		domain.AttendanceCorrectionTypeBusinessTrip,
	} {
		t.Run(correctionType.String(), func(t *testing.T) {
			date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
			repo := &fakeRepository{}
			svc := service.New(
				repo,
				service.WithLogger(discardLogger()),
				service.WithNow(func() time.Time { return date.Add(24 * time.Hour) }),
				service.WithAttendanceRules(testAttendanceRules()),
			)

			_, err := svc.SaveAttendanceCorrection(context.Background(), domain.AttendanceCorrection{
				UserID:      "REDACTED_USER_ID",
				DeviceSN:    "REDACTED_DEVICE_SN",
				Date:        date,
				Type:        correctionType,
				CorrectedAt: date.Add(12 * time.Hour),
				Reason:      "attendance exception",
			})
			if err != nil {
				t.Fatalf("save correction: %v", err)
			}
			if len(repo.monthlyResults) != 1 {
				t.Fatalf("unexpected monthly results length: %d", len(repo.monthlyResults))
			}
			if repo.monthlyResults[0].CorrectionType != correctionType {
				t.Fatalf("unexpected stored correction type: %s", repo.monthlyResults[0].CorrectionType)
			}
			if repo.monthlyResults[0].IsAbnormal {
				t.Fatal("leave or business trip should not be abnormal")
			}
		})
	}
}

func TestSaveAttendanceCorrectionUsesSettlementMonth(t *testing.T) {
	date := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		settings: domain.AttendanceSettings{
			Timezone:       "UTC",
			DefaultShiftID: "day",
			WeekendDays:    []time.Weekday{time.Saturday, time.Sunday},
			SettlementDay:  20,
		},
	}
	svc := service.New(
		repo,
		service.WithLogger(discardLogger()),
		service.WithNow(func() time.Time { return date.Add(24 * time.Hour) }),
		service.WithAttendanceRules(testAttendanceRules()),
	)

	_, err := svc.SaveAttendanceCorrection(context.Background(), domain.AttendanceCorrection{
		UserID:      "REDACTED_USER_ID",
		DeviceSN:    "REDACTED_DEVICE_SN",
		Date:        date,
		Type:        domain.AttendanceCorrectionTypeLeave,
		CorrectedAt: date.Add(12 * time.Hour),
		Reason:      "annual leave",
	})
	if err != nil {
		t.Fatalf("save correction: %v", err)
	}
	if len(repo.monthlyResults) != 1 {
		t.Fatalf("unexpected monthly results length: %d", len(repo.monthlyResults))
	}
	if repo.monthlyResults[0].Month.Format("2006-01") != "2026-08" {
		t.Fatalf("unexpected monthly result month: %s", repo.monthlyResults[0].Month)
	}
}

func TestListDailyAttendanceUsesScheduledNightShiftAcrossDays(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	rules := testAttendanceRules()
	rules.Shifts["night"] = domain.AttendanceShift{
		ID:              "night",
		Name:            "Night Shift",
		Start:           domain.ClockTime{Hour: 21},
		End:             domain.ClockTime{Hour: 6},
		LateGrace:       5 * time.Minute,
		EarlyLeaveGrace: 5 * time.Minute,
		Enabled:         true,
	}
	rules.Schedules = []domain.AttendanceSchedule{
		{
			UserID:  "REDACTED_USER_ID",
			Date:    date,
			ShiftID: "night",
		},
	}
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(21*time.Hour+3*time.Minute), domain.AccessDirectionEntry),
			dailyRecord(date.AddDate(0, 0, 1).Add(5*time.Hour+58*time.Minute), domain.AccessDirectionExit),
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()), service.WithAttendanceRules(rules))

	dailies, err := svc.ListDailyAttendance(context.Background(), domain.DailyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date},
	})
	if err != nil {
		t.Fatalf("list daily attendance: %v", err)
	}
	if len(dailies) != 1 {
		t.Fatalf("unexpected daily attendance length: %d", len(dailies))
	}
	if dailies[0].Status != domain.DailyAttendanceStatusNormal {
		t.Fatalf("unexpected status: %+v", dailies[0])
	}
	if dailies[0].ShiftID != "night" {
		t.Fatalf("unexpected shift id: %s", dailies[0].ShiftID)
	}
	if !dailies[0].LastExitAt.Equal(date.AddDate(0, 0, 1).Add(5*time.Hour + 58*time.Minute)) {
		t.Fatalf("unexpected last exit: %s", dailies[0].LastExitAt)
	}
}

func TestListMonthlyAttendanceAggregatesUserStats(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(month.AddDate(0, 0, 9).Add(8*time.Hour+50*time.Minute), domain.AccessDirectionEntry),
			dailyRecord(month.AddDate(0, 0, 9).Add(18*time.Hour+10*time.Minute), domain.AccessDirectionExit),
			dailyRecord(month.AddDate(0, 0, 10).Add(9*time.Hour+10*time.Minute), domain.AccessDirectionEntry),
			dailyRecord(month.AddDate(0, 0, 10).Add(17*time.Hour+30*time.Minute), domain.AccessDirectionExit),
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	records, err := svc.ListMonthlyAttendance(context.Background(), domain.MonthlyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		Month:                  month,
	})
	if err != nil {
		t.Fatalf("list monthly attendance: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("unexpected monthly records length: %d", len(records))
	}

	stats := records[0].Stats
	if stats.TotalDays != 31 {
		t.Fatalf("unexpected total days: %d", stats.TotalDays)
	}
	if stats.NormalDays != 1 {
		t.Fatalf("unexpected normal days: %d", stats.NormalDays)
	}
	if stats.AbnormalDays != 30 {
		t.Fatalf("unexpected abnormal days: %d", stats.AbnormalDays)
	}
	if stats.LateDays != 1 || stats.EarlyLeaveDays != 0 || stats.LateAndEarlyLeaveDays != 0 {
		t.Fatalf("unexpected exception stats: %+v", stats)
	}
	if stats.AbsentDays != 29 {
		t.Fatalf("unexpected absent days: %d", stats.AbsentDays)
	}
	if stats.TotalLateDuration != 10*time.Minute {
		t.Fatalf("unexpected total late duration: %s", stats.TotalLateDuration)
	}
	if stats.TotalEarlyLeaveDuration != 0 {
		t.Fatalf("unexpected total early leave duration: %s", stats.TotalEarlyLeaveDuration)
	}
}

func TestListMonthlyAttendanceUsesSettlementDay(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		settings: domain.AttendanceSettings{
			Timezone:       "UTC",
			DefaultShiftID: "day",
			WeekendDays:    []time.Weekday{time.Saturday, time.Sunday},
			SettlementDay:  20,
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	records, err := svc.ListMonthlyAttendance(context.Background(), domain.MonthlyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		Month:                  month,
	})
	if err != nil {
		t.Fatalf("list monthly attendance: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("unexpected monthly records length: %d", len(records))
	}

	record := records[0]
	if record.PeriodStart.Format("2006-01-02") != "2026-07-21" {
		t.Fatalf("unexpected period start: %s", record.PeriodStart)
	}
	if record.PeriodEnd.Format("2006-01-02") != "2026-08-20" {
		t.Fatalf("unexpected period end: %s", record.PeriodEnd)
	}
	if record.SettlementDay != 20 {
		t.Fatalf("unexpected settlement day: %d", record.SettlementDay)
	}
	if len(record.Days) != 31 {
		t.Fatalf("unexpected monthly days length: %d", len(record.Days))
	}
	if record.Days[0].Date.Format("2006-01-02") != "2026-07-21" ||
		record.Days[len(record.Days)-1].Date.Format("2006-01-02") != "2026-08-20" {
		t.Fatalf("unexpected monthly day range: first=%s last=%s", record.Days[0].Date, record.Days[len(record.Days)-1].Date)
	}
}

func TestListMonthlyAttendanceDefaultsToCurrentSettlementPeriod(t *testing.T) {
	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		settings: domain.AttendanceSettings{
			Timezone:       "UTC",
			DefaultShiftID: "day",
			WeekendDays:    []time.Weekday{time.Saturday, time.Sunday},
			SettlementDay:  20,
		},
	}
	svc := service.New(
		repo,
		service.WithLogger(discardLogger()),
		service.WithNow(func() time.Time { return now }),
	)

	records, err := svc.ListMonthlyAttendance(context.Background(), domain.MonthlyAttendanceQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
	})
	if err != nil {
		t.Fatalf("list monthly attendance: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("unexpected monthly records length: %d", len(records))
	}

	record := records[0]
	if record.Month.Format("2006-01") != "2026-09" {
		t.Fatalf("unexpected default settlement month: %s", record.Month)
	}
	if record.PeriodStart.Format("2006-01-02") != "2026-08-21" {
		t.Fatalf("unexpected period start: %s", record.PeriodStart)
	}
	if record.PeriodEnd.Format("2006-01-02") != "2026-09-20" {
		t.Fatalf("unexpected period end: %s", record.PeriodEnd)
	}
}

func TestGetAttendanceSummaryAggregatesRange(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(8*time.Hour+50*time.Minute), domain.AccessDirectionEntry),
			dailyRecord(date.Add(18*time.Hour+10*time.Minute), domain.AccessDirectionExit),
			{
				UserID:    "REDACTED_USER_ID_2",
				CardName:  "REDACTED_NAME_2",
				DeviceSN:  "REDACTED_DEVICE_SN",
				Direction: domain.AccessDirectionEntry,
				Status:    1,
				EventTime: date.AddDate(0, 0, 1).Add(9*time.Hour + 10*time.Minute),
			},
			{
				UserID:    "REDACTED_USER_ID_2",
				CardName:  "REDACTED_NAME_2",
				DeviceSN:  "REDACTED_DEVICE_SN",
				Direction: domain.AccessDirectionExit,
				Status:    1,
				EventTime: date.AddDate(0, 0, 1).Add(18 * time.Hour),
			},
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	summary, err := svc.GetAttendanceSummary(context.Background(), domain.AttendanceSummaryQuery{
		DateRangeFilter: domain.DateRangeFilter{StartDate: date, EndDate: date.AddDate(0, 0, 1)},
	})
	if err != nil {
		t.Fatalf("get attendance summary: %v", err)
	}
	if summary.UserCount != 2 {
		t.Fatalf("unexpected user count: %d", summary.UserCount)
	}
	if summary.Stats.TotalDays != 2 {
		t.Fatalf("unexpected total days: %d", summary.Stats.TotalDays)
	}
	if summary.Stats.NormalDays != 1 || summary.Stats.LateDays != 1 {
		t.Fatalf("unexpected summary stats: %+v", summary.Stats)
	}
}

func TestListAttendanceExceptionsFiltersAndPaginates(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			dailyRecord(date.Add(8*time.Hour+50*time.Minute), domain.AccessDirectionEntry),
			dailyRecord(date.Add(18*time.Hour+10*time.Minute), domain.AccessDirectionExit),
			dailyRecord(date.AddDate(0, 0, 1).Add(9*time.Hour+10*time.Minute), domain.AccessDirectionEntry),
			dailyRecord(date.AddDate(0, 0, 1).Add(18*time.Hour), domain.AccessDirectionExit),
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	exceptions, err := svc.ListAttendanceExceptions(context.Background(), domain.AttendanceExceptionQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{UserID: "REDACTED_USER_ID"},
		DateRangeFilter:        domain.DateRangeFilter{StartDate: date, EndDate: date.AddDate(0, 0, 2)},
		Pagination:             domain.Pagination{Limit: 1, Offset: 1},
	})
	if err != nil {
		t.Fatalf("list attendance exceptions: %v", err)
	}
	if len(exceptions) != 1 {
		t.Fatalf("unexpected exceptions length: %d", len(exceptions))
	}
	if exceptions[0].Status != domain.DailyAttendanceStatusLate {
		t.Fatalf("unexpected status: %s", exceptions[0].Status)
	}
}

func dailyRecord(eventTime time.Time, direction domain.AccessDirection) domain.AttendanceRecord {
	return domain.AttendanceRecord{
		UserID:    "REDACTED_USER_ID",
		CardName:  "REDACTED_NAME",
		DeviceSN:  "REDACTED_DEVICE_SN",
		Direction: direction,
		Status:    1,
		EventTime: eventTime,
	}
}

func testAttendanceRules() domain.AttendanceRules {
	return domain.AttendanceRules{
		Location:       time.UTC,
		DefaultShiftID: "day",
		WeekendDays:    map[time.Weekday]bool{},
		Holidays:       map[string]string{},
		Workdays:       map[string]bool{},
		Shifts: map[string]domain.AttendanceShift{
			"day": {
				ID:      "day",
				Name:    "Day Shift",
				Start:   domain.ClockTime{Hour: 9},
				End:     domain.ClockTime{Hour: 18},
				Enabled: true,
			},
		},
	}
}

func accessControlEnvelope(t *testing.T) domain.EventEnvelope {
	t.Helper()

	return domain.EventEnvelope{
		Action:     domain.EventActionPulse,
		Code:       domain.EventCodeAccessControl,
		Index:      0,
		DataSource: domain.DataSourceOffline,
		Data:       mustRawMessage(t, accessControlData()),
	}
}

func doorStatusEnvelope(t *testing.T) domain.EventEnvelope {
	t.Helper()

	return domain.EventEnvelope{
		Action: domain.EventActionPulse,
		Code:   domain.EventCodeDoorStatus,
		Index:  0,
		Data:   mustRawMessage(t, doorStatusData()),
	}
}

func mustRawMessage(t *testing.T, value string) json.RawMessage {
	t.Helper()

	var raw json.RawMessage
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		t.Fatalf("unmarshal raw message: %v", err)
	}
	return raw
}

func accessControlData() string {
	return `{
		"BlockId": 10001,
		"CardName": "REDACTED_NAME",
		"CardNo": "",
		"CardType": 0,
		"CreateTime": 1700000000,
		"Door": 0,
		"ErrorCode": 0,
		"ImageInfo": [
			{
				"Height": 640,
				"Length": 15344,
				"Offset": 0,
				"Width": 384
			}
		],
		"Method": 15,
		"ReaderID": "1",
		"RealUTC": 1700000000,
		"SN": "REDACTED_DEVICE_SN",
		"Status": 1,
		"Type": "Entry",
		"UTC": 1700000000,
		"UserID": "REDACTED_USER_ID",
		"UserType": 0
	}`
}

func doorStatusData() string {
	return `{
		"RealUTC": 1700000120,
		"SN": "REDACTED_DEVICE_SN",
		"Status": "Open",
		"UTC": 1700000120
	}`
}

func fixedNow() time.Time {
	return time.Date(2026, 8, 10, 16, 40, 0, 0, time.UTC)
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
