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
	query string
	args  []any
	err   error
}

func (e *fakeExecutor) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	e.query = query
	e.args = args
	if e.err != nil {
		return nil, e.err
	}
	return fakeResult(1), nil
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

func TestSQLRepositoryWrapsExecutorError(t *testing.T) {
	executor := &fakeExecutor{err: errors.New("database failed")}
	repo, err := repository.NewSQLRepository(executor)
	if err != nil {
		t.Fatalf("new repository: %v", err)
	}

	err = repo.SaveDoorStatusRecord(context.Background(), domain.DoorStatusRecord{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func fixedTime() time.Time {
	return time.Date(2026, 8, 10, 16, 45, 0, 0, time.UTC)
}
