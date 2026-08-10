package dubbo_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	attendancev1 "github.com/tuxnode/dahua-attendance-backend/api/attendance/v1"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/repository"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/service"
	"github.com/tuxnode/dahua-attendance-backend/internal/transport/dubbo"
)

type fakeRepository struct {
	attendanceRecords []domain.AttendanceRecord
	query             domain.AttendanceRecordQuery
	err               error
}

var _ repository.Repository = (*fakeRepository)(nil)

func (r *fakeRepository) SaveAttendanceRecord(context.Context, domain.AttendanceRecord) error {
	return nil
}

func (r *fakeRepository) SaveDoorStatusRecord(context.Context, domain.DoorStatusRecord) error {
	return nil
}

func (r *fakeRepository) ListAttendanceRecords(_ context.Context, query domain.AttendanceRecordQuery) ([]domain.AttendanceRecord, error) {
	if r.err != nil {
		return nil, r.err
	}

	r.query = query
	return append([]domain.AttendanceRecord(nil), r.attendanceRecords...), nil
}

func TestListAttendanceRecordsConvertsRequestAndResponse(t *testing.T) {
	eventTime := time.Unix(1700000000, 0)
	receivedAt := time.Unix(1700000100, 0)
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			{
				DeviceSN:   "REDACTED_DEVICE_SN",
				UserID:     "REDACTED_USER_ID",
				CardName:   "REDACTED_NAME",
				Method:     domain.AccessMethodFaceOpen,
				Direction:  domain.AccessDirectionEntry,
				Status:     1,
				EventTime:  eventTime,
				ImageCount: 1,
				ReceivedAt: receivedAt,
				RawEvent:   []byte(`{"sensitive":true}`),
			},
		},
	}
	provider := dubbo.NewAttendanceProvider(service.New(repo))

	resp, err := provider.ListAttendanceRecords(context.Background(), &attendancev1.ListAttendanceRecordsRequest{
		UserID:    "REDACTED_USER_ID",
		DeviceSN:  "REDACTED_DEVICE_SN",
		StartTime: 1700000000,
		EndTime:   1700000200,
		Limit:     20,
		Offset:    40,
	})
	if err != nil {
		t.Fatalf("list attendance records: %v", err)
	}

	if repo.query.UserID != "REDACTED_USER_ID" {
		t.Fatalf("unexpected user id filter: %s", repo.query.UserID)
	}
	if repo.query.DeviceSN != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn filter: %s", repo.query.DeviceSN)
	}
	if repo.query.StartTime.Unix() != 1700000000 {
		t.Fatalf("unexpected start time: %s", repo.query.StartTime)
	}
	if repo.query.EndTime.Unix() != 1700000200 {
		t.Fatalf("unexpected end time: %s", repo.query.EndTime)
	}
	if repo.query.Limit != 20 {
		t.Fatalf("unexpected limit: %d", repo.query.Limit)
	}
	if repo.query.Offset != 40 {
		t.Fatalf("unexpected offset: %d", repo.query.Offset)
	}

	if len(resp.Records) != 1 {
		t.Fatalf("unexpected record count: %d", len(resp.Records))
	}

	record := resp.Records[0]
	if record.UserID != "REDACTED_USER_ID" {
		t.Fatalf("unexpected user id: %s", record.UserID)
	}
	if record.UserName != "REDACTED_NAME" {
		t.Fatalf("unexpected user name: %s", record.UserName)
	}
	if record.DeviceSN != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn: %s", record.DeviceSN)
	}
	if record.Direction != "Entry" {
		t.Fatalf("unexpected direction: %s", record.Direction)
	}
	if record.Method != int32(domain.AccessMethodFaceOpen) {
		t.Fatalf("unexpected method: %d", record.Method)
	}
	if record.MethodName != "face_open" {
		t.Fatalf("unexpected method name: %s", record.MethodName)
	}
	if record.Status != 1 {
		t.Fatalf("unexpected status: %d", record.Status)
	}
	if record.EventTime != 1700000000 {
		t.Fatalf("unexpected event time: %d", record.EventTime)
	}
	if record.ReceivedAt != 1700000100 {
		t.Fatalf("unexpected received at: %d", record.ReceivedAt)
	}
	if !record.HasSnapshot {
		t.Fatal("expected snapshot flag")
	}
	if record.SnapshotCount != 1 {
		t.Fatalf("unexpected snapshot count: %d", record.SnapshotCount)
	}
}

func TestListAttendanceRecordsRejectsNilRequest(t *testing.T) {
	provider := dubbo.NewAttendanceProvider(service.New(&fakeRepository{}))

	_, err := provider.ListAttendanceRecords(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListAttendanceRecordsReturnsServiceError(t *testing.T) {
	provider := dubbo.NewAttendanceProvider(service.New(&fakeRepository{err: errors.New("query failed")}))

	_, err := provider.ListAttendanceRecords(context.Background(), &attendancev1.ListAttendanceRecordsRequest{Limit: 10})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "query failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewAttendanceProviderRejectsNilService(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()

	dubbo.NewAttendanceProvider(nil)
}

func TestAttendanceRecordDTODoesNotExposeRawEvent(t *testing.T) {
	if _, ok := reflect.TypeOf(attendancev1.AttendanceRecordDTO{}).FieldByName("RawEvent"); ok {
		t.Fatal("AttendanceRecordDTO must not expose raw event")
	}
}
