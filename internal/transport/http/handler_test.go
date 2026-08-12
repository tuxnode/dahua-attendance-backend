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
	payload            *parser.ParsedPayload
	records            []domain.AttendanceRecord
	dailyRecords       []domain.DailyAttendance
	monthlyRecords     []domain.MonthlyAttendance
	summary            domain.AttendanceSummary
	settings           domain.AttendanceSettings
	shifts             []domain.AttendanceShift
	calendarDays       []domain.AttendanceCalendarDay
	schedules          []domain.ManagedAttendanceSchedule
	weeklySchedules    []domain.ManagedAttendanceWeeklySchedule
	query              domain.AttendanceRecordQuery
	dailyQuery         domain.DailyAttendanceQuery
	monthlyQuery       domain.MonthlyAttendanceQuery
	summaryQuery       domain.AttendanceSummaryQuery
	exceptionQuery     domain.AttendanceExceptionQuery
	shiftQuery         domain.AttendanceShiftQuery
	calendarDayQuery   domain.AttendanceCalendarDayQuery
	scheduleQuery      domain.AttendanceScheduleQuery
	weeklyScheduleQuery domain.AttendanceWeeklyScheduleQuery
	savedSettings      domain.AttendanceSettings
	savedShift         domain.AttendanceShift
	deletedShiftID     string
	savedCalendarDay   domain.AttendanceCalendarDay
	deletedCalendarDay time.Time
	savedSchedule      domain.ManagedAttendanceSchedule
	deletedScheduleID  int64
	savedWeeklySchedule domain.ManagedAttendanceWeeklySchedule
	deletedWeeklyScheduleID int64
	handleErr          error
	listErr            error
	listDailyErr       error
	listMonthlyErr     error
	summaryErr         error
	listExceptionErr   error
	settingsErr        error
	saveSettingsErr    error
	listShiftsErr      error
	saveShiftErr       error
	deleteShiftErr     error
	listCalendarDaysErr error
	saveCalendarDayErr error
	deleteCalendarDayErr error
	listSchedulesErr   error
	saveScheduleErr    error
	deleteScheduleErr  error
	listWeeklySchedulesErr error
	saveWeeklyScheduleErr error
	deleteWeeklyScheduleErr error
	handleDeviceCalls  int
	listRecordsCalls   int
	listDailyCalls     int
	listMonthlyCalls   int
	summaryCalls       int
	listExceptionCalls int
	getSettingsCalls   int
	saveSettingsCalls  int
	listShiftsCalls    int
	saveShiftCalls     int
	deleteShiftCalls   int
	listCalendarDaysCalls int
	saveCalendarDayCalls int
	deleteCalendarDayCalls int
	listSchedulesCalls int
	saveScheduleCalls  int
	deleteScheduleCalls int
	listWeeklySchedulesCalls int
	saveWeeklyScheduleCalls int
	deleteWeeklyScheduleCalls int
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

func (s *fakeService) ListMonthlyAttendance(_ context.Context, query domain.MonthlyAttendanceQuery) ([]domain.MonthlyAttendance, error) {
	s.listMonthlyCalls++
	s.monthlyQuery = query
	if s.listMonthlyErr != nil {
		return nil, s.listMonthlyErr
	}
	return append([]domain.MonthlyAttendance(nil), s.monthlyRecords...), nil
}

func (s *fakeService) GetAttendanceSummary(_ context.Context, query domain.AttendanceSummaryQuery) (domain.AttendanceSummary, error) {
	s.summaryCalls++
	s.summaryQuery = query
	if s.summaryErr != nil {
		return domain.AttendanceSummary{}, s.summaryErr
	}
	return s.summary, nil
}

func (s *fakeService) ListAttendanceExceptions(_ context.Context, query domain.AttendanceExceptionQuery) ([]domain.DailyAttendance, error) {
	s.listExceptionCalls++
	s.exceptionQuery = query
	if s.listExceptionErr != nil {
		return nil, s.listExceptionErr
	}
	return append([]domain.DailyAttendance(nil), s.dailyRecords...), nil
}

func (s *fakeService) GetAttendanceSettings(_ context.Context) (domain.AttendanceSettings, error) {
	s.getSettingsCalls++
	if s.settingsErr != nil {
		return domain.AttendanceSettings{}, s.settingsErr
	}
	return s.settings, nil
}

func (s *fakeService) SaveAttendanceSettings(_ context.Context, settings domain.AttendanceSettings) (domain.AttendanceSettings, error) {
	s.saveSettingsCalls++
	s.savedSettings = settings
	if s.saveSettingsErr != nil {
		return domain.AttendanceSettings{}, s.saveSettingsErr
	}
	return settings, nil
}

func (s *fakeService) ListAttendanceShifts(_ context.Context, query domain.AttendanceShiftQuery) ([]domain.AttendanceShift, error) {
	s.listShiftsCalls++
	s.shiftQuery = query
	if s.listShiftsErr != nil {
		return nil, s.listShiftsErr
	}
	return append([]domain.AttendanceShift(nil), s.shifts...), nil
}

func (s *fakeService) SaveAttendanceShift(_ context.Context, shift domain.AttendanceShift) (domain.AttendanceShift, error) {
	s.saveShiftCalls++
	s.savedShift = shift
	if s.saveShiftErr != nil {
		return domain.AttendanceShift{}, s.saveShiftErr
	}
	return shift, nil
}

func (s *fakeService) DeleteAttendanceShift(_ context.Context, id string) error {
	s.deleteShiftCalls++
	s.deletedShiftID = id
	return s.deleteShiftErr
}

func (s *fakeService) ListAttendanceCalendarDays(_ context.Context, query domain.AttendanceCalendarDayQuery) ([]domain.AttendanceCalendarDay, error) {
	s.listCalendarDaysCalls++
	s.calendarDayQuery = query
	if s.listCalendarDaysErr != nil {
		return nil, s.listCalendarDaysErr
	}
	return append([]domain.AttendanceCalendarDay(nil), s.calendarDays...), nil
}

func (s *fakeService) SaveAttendanceCalendarDay(_ context.Context, day domain.AttendanceCalendarDay) (domain.AttendanceCalendarDay, error) {
	s.saveCalendarDayCalls++
	s.savedCalendarDay = day
	if s.saveCalendarDayErr != nil {
		return domain.AttendanceCalendarDay{}, s.saveCalendarDayErr
	}
	return day, nil
}

func (s *fakeService) DeleteAttendanceCalendarDay(_ context.Context, date time.Time) error {
	s.deleteCalendarDayCalls++
	s.deletedCalendarDay = date
	return s.deleteCalendarDayErr
}

func (s *fakeService) ListAttendanceSchedules(_ context.Context, query domain.AttendanceScheduleQuery) ([]domain.ManagedAttendanceSchedule, error) {
	s.listSchedulesCalls++
	s.scheduleQuery = query
	if s.listSchedulesErr != nil {
		return nil, s.listSchedulesErr
	}
	return append([]domain.ManagedAttendanceSchedule(nil), s.schedules...), nil
}

func (s *fakeService) SaveAttendanceSchedule(_ context.Context, schedule domain.ManagedAttendanceSchedule) (domain.ManagedAttendanceSchedule, error) {
	s.saveScheduleCalls++
	s.savedSchedule = schedule
	if s.saveScheduleErr != nil {
		return domain.ManagedAttendanceSchedule{}, s.saveScheduleErr
	}
	if schedule.ID == 0 {
		schedule.ID = 1
	}
	return schedule, nil
}

func (s *fakeService) DeleteAttendanceSchedule(_ context.Context, id int64) error {
	s.deleteScheduleCalls++
	s.deletedScheduleID = id
	return s.deleteScheduleErr
}

func (s *fakeService) ListAttendanceWeeklySchedules(_ context.Context, query domain.AttendanceWeeklyScheduleQuery) ([]domain.ManagedAttendanceWeeklySchedule, error) {
	s.listWeeklySchedulesCalls++
	s.weeklyScheduleQuery = query
	if s.listWeeklySchedulesErr != nil {
		return nil, s.listWeeklySchedulesErr
	}
	return append([]domain.ManagedAttendanceWeeklySchedule(nil), s.weeklySchedules...), nil
}

func (s *fakeService) SaveAttendanceWeeklySchedule(_ context.Context, schedule domain.ManagedAttendanceWeeklySchedule) (domain.ManagedAttendanceWeeklySchedule, error) {
	s.saveWeeklyScheduleCalls++
	s.savedWeeklySchedule = schedule
	if s.saveWeeklyScheduleErr != nil {
		return domain.ManagedAttendanceWeeklySchedule{}, s.saveWeeklyScheduleErr
	}
	if schedule.ID == 0 {
		schedule.ID = 1
	}
	return schedule, nil
}

func (s *fakeService) DeleteAttendanceWeeklySchedule(_ context.Context, id int64) error {
	s.deleteWeeklyScheduleCalls++
	s.deletedWeeklyScheduleID = id
	return s.deleteWeeklyScheduleErr
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

func TestHandleMonthlyAttendanceReturnsRecords(t *testing.T) {
	month := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	service := &fakeService{
		monthlyRecords: []domain.MonthlyAttendance{
			{
				Month:    month,
				UserID:   "REDACTED_USER_ID",
				UserName: "REDACTED_NAME",
				DeviceSN: "REDACTED_DEVICE_SN",
				Stats: domain.AttendanceStats{
					TotalDays:               31,
					NormalDays:              20,
					AbnormalDays:            11,
					LateDays:                3,
					EarlyLeaveDays:          2,
					LateAndEarlyLeaveDays:   1,
					AbsentDays:              6,
					RecordCount:             50,
					SnapshotCount:           10,
					TotalLateDuration:       30 * time.Minute,
					TotalEarlyLeaveDuration: 20 * time.Minute,
				},
			},
		},
	}
	router := newTestRouter(service)

	request := httptest.NewRequest(http.MethodGet, transporthttp.MonthlyAttendancePath+"?user_id=REDACTED_USER_ID&device_sn=REDACTED_DEVICE_SN&month=2026-08&limit=20&offset=40", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}
	if service.monthlyQuery.UserID != "REDACTED_USER_ID" {
		t.Fatalf("unexpected user id filter: %s", service.monthlyQuery.UserID)
	}
	if service.monthlyQuery.DeviceSN != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn filter: %s", service.monthlyQuery.DeviceSN)
	}
	if service.monthlyQuery.Month.Format("2006-01") != "2026-08" {
		t.Fatalf("unexpected month: %s", service.monthlyQuery.Month)
	}
	if service.monthlyQuery.Limit != 20 || service.monthlyQuery.Offset != 40 {
		t.Fatalf("unexpected pagination: limit=%d offset=%d", service.monthlyQuery.Limit, service.monthlyQuery.Offset)
	}

	body := response.Body.String()
	for _, expected := range []string{
		`"month":"2026-08"`,
		`"total_days":31`,
		`"normal_days":20`,
		`"abnormal_days":11`,
		`"late_days":3`,
		`"total_late_seconds":1800`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %s: %s", expected, body)
		}
	}
}

func TestHandleMonthlyAttendanceRejectsInvalidMonth(t *testing.T) {
	router := newTestRouter(&fakeService{})

	request := httptest.NewRequest(http.MethodGet, transporthttp.MonthlyAttendancePath+"?month=2026/08", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleAttendanceSummaryReturnsSummary(t *testing.T) {
	startDate := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	endDate := time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	service := &fakeService{
		summary: domain.AttendanceSummary{
			StartDate: startDate,
			EndDate:   endDate,
			UserCount: 3,
			Stats: domain.AttendanceStats{
				TotalDays:     10,
				NormalDays:    7,
				AbnormalDays:  3,
				RecordCount:   20,
				SnapshotCount: 5,
			},
		},
	}
	router := newTestRouter(service)

	request := httptest.NewRequest(http.MethodGet, transporthttp.AttendanceSummaryPath+"?start_date=2026-08-01&end_date=2026-08-10&device_sn=REDACTED_DEVICE_SN", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}
	if service.summaryQuery.DeviceSN != "REDACTED_DEVICE_SN" {
		t.Fatalf("unexpected device sn filter: %s", service.summaryQuery.DeviceSN)
	}
	if service.summaryQuery.StartDate.Format("2006-01-02") != "2026-08-01" {
		t.Fatalf("unexpected start date: %s", service.summaryQuery.StartDate)
	}
	if service.summaryQuery.EndDate.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("unexpected end date: %s", service.summaryQuery.EndDate)
	}

	body := response.Body.String()
	for _, expected := range []string{
		`"start_date":"2026-08-01"`,
		`"end_date":"2026-08-10"`,
		`"user_count":3`,
		`"total_days":10`,
		`"normal_days":7`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %s: %s", expected, body)
		}
	}
}

func TestHandleAttendanceExceptionsReturnsRecords(t *testing.T) {
	date := time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	service := &fakeService{
		dailyRecords: []domain.DailyAttendance{
			{
				Date:         date,
				UserID:       "REDACTED_USER_ID",
				UserName:     "REDACTED_NAME",
				DeviceSN:     "REDACTED_DEVICE_SN",
				Status:       domain.DailyAttendanceStatusLate,
				Exceptions:   []domain.DailyAttendanceException{domain.DailyAttendanceExceptionLate},
				WorkStartAt:  date.Add(9 * time.Hour),
				FirstEntryAt: date.Add(9*time.Hour + 10*time.Minute),
				LateDuration: 10 * time.Minute,
				RecordCount:  2,
			},
		},
	}
	router := newTestRouter(service)

	request := httptest.NewRequest(http.MethodGet, transporthttp.AttendanceExceptionsPath+"?date=2026-08-10&limit=10&offset=5", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}
	if service.exceptionQuery.StartDate.Format("2006-01-02") != "2026-08-10" {
		t.Fatalf("unexpected start date: %s", service.exceptionQuery.StartDate)
	}
	if service.exceptionQuery.Limit != 10 || service.exceptionQuery.Offset != 5 {
		t.Fatalf("unexpected pagination: limit=%d offset=%d", service.exceptionQuery.Limit, service.exceptionQuery.Offset)
	}

	body := response.Body.String()
	for _, expected := range []string{
		`"status":"late"`,
		`"exceptions":["late"]`,
		`"late_seconds":600`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("response missing %s: %s", expected, body)
		}
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
