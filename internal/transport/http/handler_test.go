package transporthttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/parser"
	transporthttp "github.com/tuxnode/dahua-attendance-backend/internal/transport/http"
)

type fakeService struct {
	payload           *parser.ParsedPayload
	records           []domain.AttendanceRecord
	dailyRecords      []domain.DailyAttendance
	query             domain.AttendanceRecordQuery
	dailyQuery        domain.DailyAttendanceQuery
	handleErr         error
	listErr           error
	listDailyErr      error
	handleDeviceCalls int
	listRecordsCalls  int
	listDailyCalls    int
}

func (s *fakeService) HandleDevicePayload(_ context.Context, payload *parser.ParsedPayload) error {
	s.handleDeviceCalls++
	s.payload = payload
	return s.handleErr
}

func (s *fakeService) ListAttendanceRecords(_ context.Context, query domain.AttendanceRecordQuery) ([]domain.AttendanceRecord, error) {
	s.listRecordsCalls++
	s.query = query
	if s.listErr != nil {
		return nil, s.listErr
	}
	return append([]domain.AttendanceRecord(nil), s.records...), nil
}

func (s *fakeService) ListDailyAttendance(_ context.Context, query domain.DailyAttendanceQuery) ([]domain.DailyAttendance, error) {
	s.listDailyCalls++
	s.dailyQuery = query
	if s.listDailyErr != nil {
		return nil, s.listDailyErr
	}
	return append([]domain.DailyAttendance(nil), s.dailyRecords...), nil
}

func TestHandleDeviceEventsAcceptsRootJSON(t *testing.T) {
	service := &fakeService{}
	router := newTestRouter(service)

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assertSuccessResponse(t, response)
	if service.payload == nil {
		t.Fatal("consumer was not called")
	}
	if got := len(service.payload.Events); got != 1 {
		t.Fatalf("unexpected events length: %d", got)
	}
	if service.payload.Events[0].Code != domain.EventCodeDoorStatus {
		t.Fatalf("unexpected event code: %s", service.payload.Events[0].Code)
	}
}

func TestHandleDeviceEventsAcceptsDeviceEventsPath(t *testing.T) {
	router := newTestRouter(&fakeService{})

	request := httptest.NewRequest(http.MethodPost, transporthttp.DeviceEventsPath, strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assertSuccessResponse(t, response)
}

func TestHandleDeviceEventsAcceptsMultipartMixedReplace(t *testing.T) {
	router := newTestRouter(&fakeService{})

	contentType, body := multipartPayload(t)
	request := httptest.NewRequest(http.MethodPost, "/", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assertSuccessResponse(t, response)
}

func TestHandleDeviceEventsRejectsUnsupportedMethod(t *testing.T) {
	router := newTestRouter(&fakeService{})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleDeviceEventsRejectsUnknownPath(t *testing.T) {
	router := newTestRouter(&fakeService{})

	request := httptest.NewRequest(http.MethodPost, "/unknown", strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleDeviceEventsAcksInvalidPayloadWithoutConsumer(t *testing.T) {
	service := &fakeService{}
	router := newTestRouter(service)

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"Code": "Unknown", "Data": {}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assertSuccessResponse(t, response)
	if service.handleDeviceCalls != 0 {
		t.Fatalf("consumer should not be called, calls: %d", service.handleDeviceCalls)
	}
}

func TestHandleDeviceEventsAcksOversizedPayloadWithoutConsumer(t *testing.T) {
	service := &fakeService{}
	router := transporthttp.NewRouter(
		service,
		transporthttp.WithLogger(discardLogger()),
		transporthttp.WithMaxBodyBytes(8),
	)

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	assertSuccessResponse(t, response)
	if service.handleDeviceCalls != 0 {
		t.Fatalf("consumer should not be called, calls: %d", service.handleDeviceCalls)
	}
}

func TestHandleDeviceEventsReturnsServerErrorWhenConsumerFails(t *testing.T) {
	router := newTestRouter(&fakeService{handleErr: errors.New("store failed")})

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleHealthz(t *testing.T) {
	router := newTestRouter(&fakeService{})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if response.Body.String() != "ok\n" {
		t.Fatalf("unexpected body: %q", response.Body.String())
	}
}

func TestHandleAttendanceRecordsReturnsRecords(t *testing.T) {
	service := &fakeService{
		records: []domain.AttendanceRecord{
			{
				DeviceSN:   "REDACTED_DEVICE_SN",
				UserID:     "REDACTED_USER_ID",
				CardName:   "REDACTED_NAME",
				Method:     domain.AccessMethodFaceOpen,
				Direction:  domain.AccessDirectionEntry,
				Status:     1,
				EventTime:  time.Unix(1700000000, 0),
				ReceivedAt: time.Unix(1700000100, 0),
				ImageCount: 1,
				RawEvent:   []byte(`{"sensitive":true}`),
			},
		},
	}
	router := newTestRouter(service)

	request := httptest.NewRequest(http.MethodGet, transporthttp.AttendanceRecordsPath+"?user_id=REDACTED_USER_ID&device_sn=REDACTED_DEVICE_SN&start_time=1700000000&end_time=1700000200&limit=20&offset=40", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}
	if service.query.UserID != "REDACTED_USER_ID" {
		t.Fatalf("unexpected user id filter: %s", service.query.UserID)
	}
	if service.query.DeviceSN != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn filter: %s", service.query.DeviceSN)
	}
	if service.query.StartTime.Unix() != 1700000000 {
		t.Fatalf("unexpected start time: %s", service.query.StartTime)
	}
	if service.query.EndTime.Unix() != 1700000200 {
		t.Fatalf("unexpected end time: %s", service.query.EndTime)
	}
	if service.query.Limit != 20 {
		t.Fatalf("unexpected limit: %d", service.query.Limit)
	}
	if service.query.Offset != 40 {
		t.Fatalf("unexpected offset: %d", service.query.Offset)
	}
	if strings.Contains(response.Body.String(), "RawEvent") || strings.Contains(response.Body.String(), "sensitive") {
		t.Fatalf("response exposes raw event: %s", response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"method_name":"face_open"`) {
		t.Fatalf("unexpected response body: %s", response.Body.String())
	}
}

func TestHandleAttendanceRecordsRejectsInvalidTimeRange(t *testing.T) {
	router := newTestRouter(&fakeService{})

	request := httptest.NewRequest(http.MethodGet, transporthttp.AttendanceRecordsPath+"?start_time=1700000200&end_time=1700000000", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleAttendanceRecordsReturnsServerErrorWhenServiceFails(t *testing.T) {
	router := newTestRouter(&fakeService{listErr: errors.New("query failed")})

	request := httptest.NewRequest(http.MethodGet, transporthttp.AttendanceRecordsPath, nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleDailyAttendanceReturnsRecords(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	service := &fakeService{
		dailyRecords: []domain.DailyAttendance{
			{
				Date:               date,
				UserID:             "REDACTED_USER_ID",
				UserName:           "REDACTED_NAME",
				DeviceSN:           "REDACTED_DEVICE_SN",
				Status:             domain.DailyAttendanceStatusLateAndEarlyLeave,
				Exceptions:         []domain.DailyAttendanceException{domain.DailyAttendanceExceptionLate, domain.DailyAttendanceExceptionEarlyLeave},
				WorkStartAt:        date.Add(9 * time.Hour),
				WorkEndAt:          date.Add(18 * time.Hour),
				FirstEntryAt:       date.Add(9*time.Hour + 10*time.Minute),
				LastExitAt:         date.Add(17*time.Hour + 30*time.Minute),
				LateDuration:       10 * time.Minute,
				EarlyLeaveDuration: 30 * time.Minute,
				RecordCount:        2,
				SnapshotCount:      1,
			},
		},
	}
	router := newTestRouter(service)

	request := httptest.NewRequest(http.MethodGet, transporthttp.DailyAttendancePath+"?user_id=REDACTED_USER_ID&device_sn=REDACTED_DEVICE_SN&date=2026-08-10&limit=20&offset=40", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}
	if service.dailyQuery.UserID != "REDACTED_USER_ID" {
		t.Fatalf("unexpected user id filter: %s", service.dailyQuery.UserID)
	}
	if service.dailyQuery.DeviceSN != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn filter: %s", service.dailyQuery.DeviceSN)
	}
	if service.dailyQuery.StartDate.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("unexpected start date: %s", service.dailyQuery.StartDate)
	}
	if service.dailyQuery.EndDate.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("unexpected end date: %s", service.dailyQuery.EndDate)
	}
	if service.dailyQuery.Limit != 20 {
		t.Fatalf("unexpected limit: %d", service.dailyQuery.Limit)
	}
	if service.dailyQuery.Offset != 40 {
		t.Fatalf("unexpected offset: %d", service.dailyQuery.Offset)
	}

	body := response.Body.String()
	for _, expected := range []string{
		`"date":"2026-08-10"`,
		`"status":"late_and_early_leave"`,
		`"exceptions":["late","early_leave"]`,
		`"is_abnormal":true`,
		`"late_seconds":600`,
		`"early_leave_seconds":1800`,
		`"record_count":2`,
		`"snapshot_count":1`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %s: %s", expected, body)
		}
	}
}

func TestHandleDailyAttendanceAcceptsDateRange(t *testing.T) {
	service := &fakeService{}
	router := newTestRouter(service)

	request := httptest.NewRequest(http.MethodGet, transporthttp.DailyAttendancePath+"?start_date=2026-08-01&end_date=2026-08-10", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}
	if service.dailyQuery.StartDate.Format("2006-01-02") != "2026-08-01" {
		t.Fatalf("unexpected start date: %s", service.dailyQuery.StartDate)
	}
	if service.dailyQuery.EndDate.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("unexpected end date: %s", service.dailyQuery.EndDate)
	}
}

func TestHandleDailyAttendanceRejectsInvalidDate(t *testing.T) {
	router := newTestRouter(&fakeService{})

	request := httptest.NewRequest(http.MethodGet, transporthttp.DailyAttendancePath+"?date=2026/08/10", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleDailyAttendanceRejectsMixedDateAndRange(t *testing.T) {
	router := newTestRouter(&fakeService{})

	request := httptest.NewRequest(http.MethodGet, transporthttp.DailyAttendancePath+"?date=2026-08-10&start_date=2026-08-01", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleDailyAttendanceReturnsServerErrorWhenServiceFails(t *testing.T) {
	router := newTestRouter(&fakeService{listDailyErr: errors.New("query failed")})

	request := httptest.NewRequest(http.MethodGet, transporthttp.DailyAttendancePath, nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func newTestRouter(service *fakeService) *gin.Engine {
	return transporthttp.NewRouter(service, transporthttp.WithLogger(discardLogger()))
}

func assertSuccessResponse(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}

	var body domain.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Code != 0 || body.Message != "success" {
		t.Fatalf("unexpected response body: %+v", body)
	}
}

func multipartPayload(t *testing.T) (string, io.Reader) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	jsonPart, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"text/plain"},
	})
	if err != nil {
		t.Fatalf("create json part: %v", err)
	}
	if _, err := io.WriteString(jsonPart, batchAccessControlPayload()); err != nil {
		t.Fatalf("write json part: %v", err)
	}

	imagePart, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"image/jpeg"},
	})
	if err != nil {
		t.Fatalf("create image part: %v", err)
	}
	if _, err := imagePart.Write([]byte{0xff, 0xd8, 0xff, 0xdb}); err != nil {
		t.Fatalf("write image part: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	return "multipart/x-mixed-replace; boundary=" + writer.Boundary(), &body
}

func doorStatusPayload() string {
	return `{
		"Action": "Pulse",
		"Code": "DoorStatus",
		"Data": {
			"RealUTC": 1700000120,
			"SN": "REDACTED_DEVICE_SN",
			"Status": "Open",
			"UTC": 1700000120
		},
		"Index": 0
	}`
}

func batchAccessControlPayload() string {
	return `{
		"Events": [
			{
				"Code": "AccessControl",
				"Data": {
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
					"UserID": "REDACTED_USER_ID"
				},
				"DataSource": "Offline"
			}
		]
	}`
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
