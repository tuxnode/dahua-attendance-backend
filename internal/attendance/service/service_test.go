package service_test

import (
	"bytes"
	"context"
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
	query             domain.AttendanceRecordQuery
	err               error
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

func TestListAttendanceRecordsNormalizesPagination(t *testing.T) {
	repo := &fakeRepository{
		attendanceRecords: []domain.AttendanceRecord{
			{UserID: "REDACTED_USER_ID"},
		},
	}
	svc := service.New(repo, service.WithLogger(discardLogger()))

	records, err := svc.ListAttendanceRecords(context.Background(), domain.AttendanceRecordQuery{
		Limit:  -1,
		Offset: -1,
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

	_, err := svc.ListAttendanceRecords(context.Background(), domain.AttendanceRecordQuery{Limit: 1000})
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
		StartTime: fixedNow(),
		EndTime:   fixedNow().Add(-time.Second),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListAttendanceRecordsReturnsRepositoryError(t *testing.T) {
	svc := service.New(&fakeRepository{err: errors.New("query failed")}, service.WithLogger(discardLogger()))

	_, err := svc.ListAttendanceRecords(context.Background(), domain.AttendanceRecordQuery{Limit: 10})
	if err == nil {
		t.Fatal("expected error")
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
