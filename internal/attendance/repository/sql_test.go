package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/repository"
)

type fakeExecutor struct {
	query    string
	args     []any
	execErr  error
	queryErr error
}

func (e *fakeExecutor) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	e.query = query
	e.args = args
	if e.execErr != nil {
		return nil, e.execErr
	}
	return fakeResult(1), nil
}

func (e *fakeExecutor) QueryContext(_ context.Context, query string, args ...any) (*sql.Rows, error) {
	e.query = query
	e.args = args
	return nil, e.queryErr
}

type fakeResult int64

func (r fakeResult) LastInsertId() (int64, error) {
	return int64(r), nil
}

func (r fakeResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

func TestNewSQLRepositoryRejectsNilExecutor(t *testing.T) {
	_, err := repository.NewSQLRepository(nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSQLRepositorySavesAttendanceSettingsWithSettlementDay(t *testing.T) {
	executor := &fakeExecutor{}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	err = repo.SaveAttendanceSettings(context.Background(), domain.AttendanceSettings{
		Timezone:       "Asia/Shanghai",
		DefaultShiftID: "day",
		WeekendDays:    []time.Weekday{time.Saturday, time.Sunday},
		SettlementDay:  20,
	})
	if err != nil {
		t.Fatalf("save attendance settings: %v", err)
	}
	if !strings.Contains(executor.query, "settlement_day") {
		t.Fatalf("query should contain settlement_day: %s", executor.query)
	}
	if len(executor.args) != 4 {
		t.Fatalf("unexpected args length: %d", len(executor.args))
	}
	if executor.args[3] != 20 {
		t.Fatalf("unexpected settlement day arg: %v", executor.args[3])
	}
}

func TestSQLRepositorySavesAttendanceRecord(t *testing.T) {
	executor := &fakeExecutor{}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	err = repo.SaveAttendanceRecord(context.Background(), domain.AttendanceRecord{
		DeviceSN:   "REDACTED_DEVICE_SN",
		UserID:     "REDACTED_USER_ID",
		CardName:   "REDACTED_NAME",
		Method:     domain.AccessMethodFaceOpen,
		Direction:  domain.AccessDirectionEntry,
		Status:     1,
		EventTime:  fixedTime(),
		CreateTime: 1700000000,
		UTC:        1700000000,
		RealUTC:    1700000000,
		DataSource: domain.DataSourceOffline,
		BlockID:    10001,
		ImageCount: 1,
		RawEvent:   []byte(`{"Code":"AccessControl"}`),
		ReceivedAt: fixedTime(),
	})
	if err != nil {
		t.Fatalf("save attendance record: %v", err)
	}

	if !strings.Contains(executor.query, "INSERT INTO attendance_records") {
		t.Fatalf("unexpected query: %s", executor.query)
	}
	if !strings.Contains(executor.query, "ON DUPLICATE KEY UPDATE") {
		t.Fatalf("query should handle duplicate records: %s", executor.query)
	}
	if len(executor.args) != 23 {
		t.Fatalf("unexpected args length: %d", len(executor.args))
	}
	if executor.args[0] != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected first arg: %v", executor.args[0])
	}
	if executor.args[18] != int64(10001) {
		t.Fatalf("unexpected block id arg: %v", executor.args[18])
	}
	if executor.args[19] != int64(10001) {
		t.Fatalf("unexpected dedup block id arg: %v", executor.args[19])
	}
}

func TestSQLRepositoryDoesNotDeduplicateAttendanceRecordWithoutBlockID(t *testing.T) {
	executor := &fakeExecutor{}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	err = repo.SaveAttendanceRecord(context.Background(), domain.AttendanceRecord{
		DeviceSN:   "REDACTED_DEVICE_SN",
		Method:     domain.AccessMethodButton,
		Direction:  domain.AccessDirectionExit,
		Status:     1,
		EventTime:  fixedTime(),
		ReceivedAt: fixedTime(),
	})
	if err != nil {
		t.Fatalf("save attendance record: %v", err)
	}

	if len(executor.args) != 23 {
		t.Fatalf("unexpected args length: %d", len(executor.args))
	}
	if executor.args[18] != int64(0) {
		t.Fatalf("unexpected block id arg: %v", executor.args[18])
	}
	if executor.args[19] != nil {
		t.Fatalf("dedup block id should be nil for empty block id: %v", executor.args[19])
	}
}

func TestSQLRepositorySavesDoorStatusRecord(t *testing.T) {
	executor := &fakeExecutor{}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	err = repo.SaveDoorStatusRecord(context.Background(), domain.DoorStatusRecord{
		DeviceSN:   "REDACTED_DEVICE_SN",
		Status:     domain.DoorStateOpen,
		EventTime:  fixedTime(),
		UTC:        1700000120,
		RealUTC:    1700000120,
		RawEvent:   []byte(`{"Code":"DoorStatus"}`),
		ReceivedAt: fixedTime(),
	})
	if err != nil {
		t.Fatalf("save door status record: %v", err)
	}

	if !strings.Contains(executor.query, "INSERT INTO door_status_records") {
		t.Fatalf("unexpected query: %s", executor.query)
	}
	if len(executor.args) != 9 {
		t.Fatalf("unexpected args length: %d", len(executor.args))
	}
	if executor.args[1] != string(domain.DoorStateOpen) {
		t.Fatalf("unexpected status arg: %v", executor.args[1])
	}
}

func TestSQLRepositoryListsAttendanceRecordsWithFilters(t *testing.T) {
	executor := &fakeExecutor{queryErr: errors.New("query failed")}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	startTime := fixedTime().Add(-time.Hour)
	endTime := fixedTime()
	_, err = repo.ListAttendanceRecords(context.Background(), domain.AttendanceRecordQuery{
		AttendancePersonFilter: domain.AttendancePersonFilter{
			UserID:   "REDACTED_USER_ID",
			UserName: "REDACTED_NAME",
			DeviceSN: "REDACTED_DEVICE_SN",
		},
		TimeRangeFilter: domain.TimeRangeFilter{
			StartTime: startTime,
			EndTime:   endTime,
		},
		Pagination: domain.Pagination{
			Limit:  50,
			Offset: 100,
		},
	})
	if err == nil {
		t.Fatal("expected query error")
	}

	for _, want := range []string{
		"FROM attendance_records",
		"AND user_id = ?",
		"AND card_name = ?",
		"AND device_sn = ?",
		"AND event_time >= ?",
		"AND event_time <= ?",
		"ORDER BY event_time DESC, id DESC",
		"LIMIT ? OFFSET ?",
	} {
		if !strings.Contains(executor.query, want) {
			t.Fatalf("query should contain %q: %s", want, executor.query)
		}
	}
	if strings.Contains(executor.query, "raw_event") {
		t.Fatalf("query should not expose raw_event: %s", executor.query)
	}
	if len(executor.args) != 7 {
		t.Fatalf("unexpected args length: %d", len(executor.args))
	}
	if executor.args[0] != "REDACTED_USER_ID" {
		t.Fatalf("unexpected user id arg: %v", executor.args[0])
	}
	if executor.args[1] != "REDACTED_NAME" {
		t.Fatalf("unexpected user name arg: %v", executor.args[1])
	}
	if executor.args[2] != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn arg: %v", executor.args[2])
	}
	if executor.args[3] != startTime {
		t.Fatalf("unexpected start time arg: %v", executor.args[3])
	}
	if executor.args[4] != endTime {
		t.Fatalf("unexpected end time arg: %v", executor.args[4])
	}
	if executor.args[5] != 50 {
		t.Fatalf("unexpected limit arg: %v", executor.args[5])
	}
	if executor.args[6] != 100 {
		t.Fatalf("unexpected offset arg: %v", executor.args[6])
	}
}

func TestSQLRepositoryWrapsExecutorError(t *testing.T) {
	executor := &fakeExecutor{execErr: errors.New("database failed")}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	err = repo.SaveDoorStatusRecord(context.Background(), domain.DoorStatusRecord{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSQLRepositorySavesAttendanceCorrection(t *testing.T) {
	executor := &fakeExecutor{}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	correction, err := repo.SaveAttendanceCorrection(context.Background(), domain.AttendanceCorrection{
		UserID:      "REDACTED_USER_ID",
		DeviceSN:    "REDACTED_DEVICE_SN",
		Date:        fixedTime(),
		Type:        domain.AttendanceCorrectionTypeCheckOut,
		CorrectedAt: fixedTime().Add(time.Hour),
		Reason:      "manual correction",
		Status:      domain.AttendanceCorrectionStatusApplied,
	})
	if err != nil {
		t.Fatalf("save attendance correction: %v", err)
	}
	if correction.ID != 1 {
		t.Fatalf("unexpected correction id: %d", correction.ID)
	}
	if !strings.Contains(executor.query, "INSERT INTO attendance_corrections") {
		t.Fatalf("unexpected query: %s", executor.query)
	}
	if len(executor.args) != 7 {
		t.Fatalf("unexpected args length: %d", len(executor.args))
	}
	if executor.args[0] != "REDACTED_USER_ID" {
		t.Fatalf("unexpected user id arg: %v", executor.args[0])
	}
	if executor.args[3] != "check_out" {
		t.Fatalf("unexpected correction type arg: %v", executor.args[3])
	}
}

func TestSQLRepositorySavesMonthlyAttendanceResult(t *testing.T) {
	executor := &fakeExecutor{}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	result, err := repo.SaveMonthlyAttendanceResult(context.Background(), domain.MonthlyAttendanceDailyResult{
		Month:            time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Date:             fixedTime(),
		UserID:           "REDACTED_USER_ID",
		UserName:         "REDACTED_NAME",
		DeviceSN:         "REDACTED_DEVICE_SN",
		ShiftID:          "day",
		ShiftName:        "Day Shift",
		IsWorkday:        true,
		Status:           domain.DailyAttendanceStatusCorrected,
		IsAbnormal:       false,
		Corrected:        true,
		CorrectionStatus: domain.AttendanceCorrectionStatusApplied,
		CorrectionType:   domain.AttendanceCorrectionTypeLeave,
		CorrectionReason: "manual correction",
		CorrectedAt:      fixedTime(),
		Exceptions: []domain.DailyAttendanceException{
			domain.DailyAttendanceExceptionMissingCheckOut,
		},
		WorkStartAt:        fixedTime(),
		WorkEndAt:          fixedTime().Add(8 * time.Hour),
		FirstEntryAt:       fixedTime(),
		LastExitAt:         fixedTime().Add(8 * time.Hour),
		LateDuration:       2 * time.Minute,
		EarlyLeaveDuration: 3 * time.Minute,
		RecordCount:        1,
		SnapshotCount:      1,
		CalculatedAt:       fixedTime(),
	})
	if err != nil {
		t.Fatalf("save monthly attendance result: %v", err)
	}
	if result.ID != 1 {
		t.Fatalf("unexpected result id: %d", result.ID)
	}
	if !strings.Contains(executor.query, "INSERT INTO monthly_attendance_results") {
		t.Fatalf("unexpected query: %s", executor.query)
	}
	if !strings.Contains(executor.query, "ON DUPLICATE KEY UPDATE") {
		t.Fatalf("query should upsert monthly result: %s", executor.query)
	}
	if len(executor.args) != 26 {
		t.Fatalf("unexpected args length: %d", len(executor.args))
	}
	if executor.args[9] != "corrected" {
		t.Fatalf("unexpected status arg: %v", executor.args[9])
	}
	if executor.args[10] != "missing_check_out" {
		t.Fatalf("unexpected exceptions arg: %v", executor.args[10])
	}
	if executor.args[14] != "leave" {
		t.Fatalf("unexpected correction type arg: %v", executor.args[14])
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 8, 10, 16, 45, 0, 0, time.UTC)
}
