package transporthttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	attendancev1 "github.com/tuxnode/dahua-attendance-backend/api/attendance/v1"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/parser"
)

const (
	DefaultMaxBodyBytes      = 8 << 20
	DefaultPath              = "/"
	DeviceEventsPath         = "/api/v1/device/events"
	AttendanceRecordsPath    = "/api/v1/attendance/records"
	DailyAttendancePath      = "/api/v1/attendance/daily"
	MonthlyAttendancePath    = "/api/v1/attendance/monthly"
	AttendanceSummaryPath    = "/api/v1/attendance/summary"
	AttendanceExceptionsPath = "/api/v1/attendance/exceptions"
	AttendanceSettingsPath   = "/api/v1/attendance/settings"
	AttendanceShiftsPath     = "/api/v1/attendance/shifts"
	AttendanceShiftPath      = "/api/v1/attendance/shifts/:id"
	AttendanceCalendarDaysPath = "/api/v1/attendance/calendar-days"
	AttendanceCalendarDayPath  = "/api/v1/attendance/calendar-days/:date"
	AttendanceSchedulesPath    = "/api/v1/attendance/schedules"
	AttendanceSchedulePath     = "/api/v1/attendance/schedules/:id"
	AttendanceWeeklySchedulesPath = "/api/v1/attendance/weekly-schedules"
	AttendanceWeeklySchedulePath  = "/api/v1/attendance/weekly-schedules/:id"

	queryDateLayout  = "2006-01-02"
	queryMonthLayout = "2006-01"
)

type EventConsumer interface {
	HandleDevicePayload(ctx context.Context, payload *parser.ParsedPayload) error
}

type AttendanceQueryService interface {
	ListAttendanceRecords(ctx context.Context, query domain.AttendanceRecordQuery) ([]domain.AttendanceRecord, error)
}

type DailyAttendanceQueryService interface {
	ListDailyAttendance(ctx context.Context, query domain.DailyAttendanceQuery) ([]domain.DailyAttendance, error)
}

type AttendanceStatsService interface {
	ListMonthlyAttendance(ctx context.Context, query domain.MonthlyAttendanceQuery) ([]domain.MonthlyAttendance, error)
	GetAttendanceSummary(ctx context.Context, query domain.AttendanceSummaryQuery) (domain.AttendanceSummary, error)
	ListAttendanceExceptions(ctx context.Context, query domain.AttendanceExceptionQuery) ([]domain.DailyAttendance, error)
}

type AttendanceRuleManagementService interface {
	GetAttendanceSettings(ctx context.Context) (domain.AttendanceSettings, error)
	SaveAttendanceSettings(ctx context.Context, settings domain.AttendanceSettings) (domain.AttendanceSettings, error)
	ListAttendanceShifts(ctx context.Context, query domain.AttendanceShiftQuery) ([]domain.AttendanceShift, error)
	SaveAttendanceShift(ctx context.Context, shift domain.AttendanceShift) (domain.AttendanceShift, error)
	DeleteAttendanceShift(ctx context.Context, id string) error
	ListAttendanceCalendarDays(ctx context.Context, query domain.AttendanceCalendarDayQuery) ([]domain.AttendanceCalendarDay, error)
	SaveAttendanceCalendarDay(ctx context.Context, day domain.AttendanceCalendarDay) (domain.AttendanceCalendarDay, error)
	DeleteAttendanceCalendarDay(ctx context.Context, date time.Time) error
	ListAttendanceSchedules(ctx context.Context, query domain.AttendanceScheduleQuery) ([]domain.ManagedAttendanceSchedule, error)
	SaveAttendanceSchedule(ctx context.Context, schedule domain.ManagedAttendanceSchedule) (domain.ManagedAttendanceSchedule, error)
	DeleteAttendanceSchedule(ctx context.Context, id int64) error
	ListAttendanceWeeklySchedules(ctx context.Context, query domain.AttendanceWeeklyScheduleQuery) ([]domain.ManagedAttendanceWeeklySchedule, error)
	SaveAttendanceWeeklySchedule(ctx context.Context, schedule domain.ManagedAttendanceWeeklySchedule) (domain.ManagedAttendanceWeeklySchedule, error)
	DeleteAttendanceWeeklySchedule(ctx context.Context, id int64) error
}

type AttendanceService interface {
	EventConsumer
	AttendanceQueryService
	DailyAttendanceQueryService
	AttendanceStatsService
	AttendanceRuleManagementService
}

type Handler struct {
	service      AttendanceService
	logger       *slog.Logger
	maxBodyBytes int64
}

type Option func(*Handler)

func NewHandler(service AttendanceService, opts ...Option) *Handler {
	handler := &Handler{
		service:      service,
		logger:       slog.Default(),
		maxBodyBytes: DefaultMaxBodyBytes,
	}

	for _, opt := range opts {
		opt(handler)
	}

	return handler
}

func NewRouter(service AttendanceService, opts ...Option) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)

	handler := NewHandler(service, opts...)
	router := gin.New()
	router.Use(gin.Recovery())
	router.HandleMethodNotAllowed = true

	router.GET("/healthz", handler.HandleHealthz)
	router.POST(DefaultPath, handler.HandleDeviceEvents)
	router.POST(DeviceEventsPath, handler.HandleDeviceEvents)
	router.GET(AttendanceRecordsPath, handler.HandleAttendanceRecords)
	router.GET(DailyAttendancePath, handler.HandleDailyAttendance)
	router.GET(MonthlyAttendancePath, handler.HandleMonthlyAttendance)
	router.GET(AttendanceSummaryPath, handler.HandleAttendanceSummary)
	router.GET(AttendanceExceptionsPath, handler.HandleAttendanceExceptions)
	router.GET(AttendanceSettingsPath, handler.HandleAttendanceSettings)
	router.PUT(AttendanceSettingsPath, handler.HandleSaveAttendanceSettings)
	router.GET(AttendanceShiftsPath, handler.HandleAttendanceShifts)
	router.POST(AttendanceShiftsPath, handler.HandleSaveAttendanceShift)
	router.PUT(AttendanceShiftPath, handler.HandleSaveAttendanceShift)
	router.DELETE(AttendanceShiftPath, handler.HandleDeleteAttendanceShift)
	router.GET(AttendanceCalendarDaysPath, handler.HandleAttendanceCalendarDays)
	router.POST(AttendanceCalendarDaysPath, handler.HandleSaveAttendanceCalendarDay)
	router.PUT(AttendanceCalendarDayPath, handler.HandleSaveAttendanceCalendarDay)
	router.DELETE(AttendanceCalendarDayPath, handler.HandleDeleteAttendanceCalendarDay)
	router.GET(AttendanceSchedulesPath, handler.HandleAttendanceSchedules)
	router.POST(AttendanceSchedulesPath, handler.HandleSaveAttendanceSchedule)
	router.PUT(AttendanceSchedulePath, handler.HandleSaveAttendanceSchedule)
	router.DELETE(AttendanceSchedulePath, handler.HandleDeleteAttendanceSchedule)
	router.GET(AttendanceWeeklySchedulesPath, handler.HandleAttendanceWeeklySchedules)
	router.POST(AttendanceWeeklySchedulesPath, handler.HandleSaveAttendanceWeeklySchedule)
	router.PUT(AttendanceWeeklySchedulePath, handler.HandleSaveAttendanceWeeklySchedule)
	router.DELETE(AttendanceWeeklySchedulePath, handler.HandleDeleteAttendanceWeeklySchedule)

	return router
}

func WithLogger(logger *slog.Logger) Option {
	return func(handler *Handler) {
		if logger != nil {
			handler.logger = logger
		}
	}
}

func WithMaxBodyBytes(maxBodyBytes int64) Option {
	return func(handler *Handler) {
		if maxBodyBytes > 0 {
			handler.maxBodyBytes = maxBodyBytes
		}
	}
}

func (h *Handler) HandleHealthz(c *gin.Context) {
	c.String(http.StatusOK, "ok\n")
}

func (h *Handler) HandleDeviceEvents(c *gin.Context) {
	body := http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBodyBytes)
	defer body.Close()

	payload, err := parser.Parse(
		c.GetHeader("Content-Type"),
		c.GetHeader("Content-Encoding"),
		body,
	)
	if err != nil {
		h.logger.WarnContext(
			c.Request.Context(),
			"failed to parse device payload",
			"remote_addr", c.Request.RemoteAddr,
			"content_type", c.GetHeader("Content-Type"),
			"content_encoding", c.GetHeader("Content-Encoding"),
			"error", err,
		)
		writeDeviceSuccess(c)
		return
	}

	if h.service != nil {
		if err := h.service.HandleDevicePayload(c.Request.Context(), payload); err != nil {
			h.logger.ErrorContext(
				c.Request.Context(),
				"failed to handle device payload",
				"remote_addr", c.Request.RemoteAddr,
				"events", len(payload.Events),
				"images", len(payload.Images),
				"error", err,
			)
			writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to handle device payload")
			return
		}
	}

	h.logger.InfoContext(
		c.Request.Context(),
		"device payload accepted",
		"remote_addr", c.Request.RemoteAddr,
		"events", len(payload.Events),
		"images", len(payload.Images),
	)

	writeDeviceSuccess(c)
}

func (h *Handler) HandleAttendanceRecords(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseAttendanceRecordQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	records, err := h.service.ListAttendanceRecords(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to list attendance records")
		return
	}

	c.JSON(http.StatusOK, attendancev1.ListAttendanceRecordsResponse{
		Records: toAttendanceRecordDTOs(records),
	})
}

func (h *Handler) HandleDailyAttendance(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseDailyAttendanceQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	records, err := h.service.ListDailyAttendance(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to list daily attendance")
		return
	}

	c.JSON(http.StatusOK, attendancev1.ListDailyAttendanceResponse{
		Records: toDailyAttendanceDTOs(records),
	})
}

func (h *Handler) HandleMonthlyAttendance(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseMonthlyAttendanceQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	records, err := h.service.ListMonthlyAttendance(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to list monthly attendance")
		return
	}

	c.JSON(http.StatusOK, attendancev1.ListMonthlyAttendanceResponse{
		Records: toMonthlyAttendanceDTOs(records),
	})
}

func (h *Handler) HandleAttendanceSummary(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseAttendanceSummaryQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	summary, err := h.service.GetAttendanceSummary(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to get attendance summary")
		return
	}

	c.JSON(http.StatusOK, attendancev1.GetAttendanceSummaryResponse{
		Summary: toAttendanceSummaryDTO(summary),
	})
}

func (h *Handler) HandleAttendanceExceptions(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseAttendanceExceptionQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	records, err := h.service.ListAttendanceExceptions(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to list attendance exceptions")
		return
	}

	c.JSON(http.StatusOK, attendancev1.ListAttendanceExceptionsResponse{
		Records: toDailyAttendanceDTOs(records),
	})
}

func (h *Handler) HandleAttendanceSettings(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	settings, err := h.service.GetAttendanceSettings(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to get attendance settings")
		return
	}

	c.JSON(http.StatusOK, attendancev1.GetAttendanceSettingsResponse{
		Settings: toAttendanceSettingsDTO(settings),
	})
}

func (h *Handler) HandleSaveAttendanceSettings(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	var request attendancev1.SaveAttendanceSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	settings, err := attendanceSettingsFromRequest(request)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	saved, err := h.service.SaveAttendanceSettings(c.Request.Context(), settings)
	if err != nil {
		writeManagementError(c, err, "failed to save attendance settings")
		return
	}

	c.JSON(http.StatusOK, attendancev1.SaveAttendanceSettingsResponse{
		Settings: toAttendanceSettingsDTO(saved),
	})
}

func (h *Handler) HandleAttendanceShifts(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseAttendanceShiftQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	shifts, err := h.service.ListAttendanceShifts(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to list attendance shifts")
		return
	}

	c.JSON(http.StatusOK, attendancev1.ListAttendanceShiftsResponse{
		Records: toAttendanceShiftDTOs(shifts),
	})
}

func (h *Handler) HandleSaveAttendanceShift(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	var request attendancev1.SaveAttendanceShiftRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	shift, err := attendanceShiftFromRequest(request, c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	saved, err := h.service.SaveAttendanceShift(c.Request.Context(), shift)
	if err != nil {
		writeManagementError(c, err, "failed to save attendance shift")
		return
	}

	c.JSON(http.StatusOK, attendancev1.SaveAttendanceShiftResponse{
		Record: toAttendanceShiftDTO(saved),
	})
}

func (h *Handler) HandleDeleteAttendanceShift(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		writeError(c, http.StatusBadRequest, "invalid_request", "id cannot be empty")
		return
	}
	if err := h.service.DeleteAttendanceShift(c.Request.Context(), id); err != nil {
		writeManagementError(c, err, "failed to delete attendance shift")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleAttendanceCalendarDays(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseAttendanceCalendarDayQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	days, err := h.service.ListAttendanceCalendarDays(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to list attendance calendar days")
		return
	}

	c.JSON(http.StatusOK, attendancev1.ListAttendanceCalendarDaysResponse{
		Records: toAttendanceCalendarDayDTOs(days),
	})
}

func (h *Handler) HandleSaveAttendanceCalendarDay(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	var request attendancev1.SaveAttendanceCalendarDayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	day, err := attendanceCalendarDayFromRequest(request, c.Param("date"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	saved, err := h.service.SaveAttendanceCalendarDay(c.Request.Context(), day)
	if err != nil {
		writeManagementError(c, err, "failed to save attendance calendar day")
		return
	}

	c.JSON(http.StatusOK, attendancev1.SaveAttendanceCalendarDayResponse{
		Record: toAttendanceCalendarDayDTO(saved),
	})
}

func (h *Handler) HandleDeleteAttendanceCalendarDay(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	date, err := parseDateValue("date", c.Param("date"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := h.service.DeleteAttendanceCalendarDay(c.Request.Context(), date); err != nil {
		writeManagementError(c, err, "failed to delete attendance calendar day")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleAttendanceSchedules(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseAttendanceScheduleQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	schedules, err := h.service.ListAttendanceSchedules(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to list attendance schedules")
		return
	}

	c.JSON(http.StatusOK, attendancev1.ListAttendanceSchedulesResponse{
		Records: toAttendanceScheduleDTOs(schedules),
	})
}

func (h *Handler) HandleSaveAttendanceSchedule(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	var request attendancev1.SaveAttendanceScheduleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	schedule, err := attendanceScheduleFromRequest(request, c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	saved, err := h.service.SaveAttendanceSchedule(c.Request.Context(), schedule)
	if err != nil {
		writeManagementError(c, err, "failed to save attendance schedule")
		return
	}

	c.JSON(http.StatusOK, attendancev1.SaveAttendanceScheduleResponse{
		Record: toAttendanceScheduleDTO(saved),
	})
}

func (h *Handler) HandleDeleteAttendanceSchedule(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	id, err := parseIDParam(c, "id")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := h.service.DeleteAttendanceSchedule(c.Request.Context(), id); err != nil {
		writeManagementError(c, err, "failed to delete attendance schedule")
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) HandleAttendanceWeeklySchedules(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	query, err := parseAttendanceWeeklyScheduleQuery(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	schedules, err := h.service.ListAttendanceWeeklySchedules(c.Request.Context(), query)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "failed to list attendance weekly schedules")
		return
	}

	c.JSON(http.StatusOK, attendancev1.ListAttendanceWeeklySchedulesResponse{
		Records: toAttendanceWeeklyScheduleDTOs(schedules),
	})
}

func (h *Handler) HandleSaveAttendanceWeeklySchedule(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	var request attendancev1.SaveAttendanceWeeklyScheduleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", "request body must be valid JSON")
		return
	}

	schedule, err := attendanceWeeklyScheduleFromRequest(request, c.Param("id"))
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	saved, err := h.service.SaveAttendanceWeeklySchedule(c.Request.Context(), schedule)
	if err != nil {
		writeManagementError(c, err, "failed to save attendance weekly schedule")
		return
	}

	c.JSON(http.StatusOK, attendancev1.SaveAttendanceWeeklyScheduleResponse{
		Record: toAttendanceWeeklyScheduleDTO(saved),
	})
}

func (h *Handler) HandleDeleteAttendanceWeeklySchedule(c *gin.Context) {
	if h.service == nil {
		writeError(c, http.StatusInternalServerError, "internal_server_error", "attendance service is not configured")
		return
	}

	id, err := parseIDParam(c, "id")
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	if err := h.service.DeleteAttendanceWeeklySchedule(c.Request.Context(), id); err != nil {
		writeManagementError(c, err, "failed to delete attendance weekly schedule")
		return
	}

	c.Status(http.StatusNoContent)
}

func writeDeviceSuccess(c *gin.Context) {
	c.JSON(http.StatusOK, domain.Response{
		Code:    0,
		Message: "success",
	})
}

func writeError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"code":    code,
		"message": message,
	})
}

func writeManagementError(c *gin.Context, err error, message string) {
	if isValidationError(err) {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeError(c, http.StatusInternalServerError, "internal_server_error", message)
}

func isValidationError(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.Contains(message, "cannot be empty") ||
		strings.Contains(message, "must be") ||
		strings.Contains(message, "must not be") ||
		strings.Contains(message, "invalid attendance") ||
		strings.Contains(message, "unsupported attendance")
}

func parseAttendanceRecordQuery(c *gin.Context) (domain.AttendanceRecordQuery, error) {
	startTime, err := parseUnixQuery(c, "start_time")
	if err != nil {
		return domain.AttendanceRecordQuery{}, err
	}
	endTime, err := parseUnixQuery(c, "end_time")
	if err != nil {
		return domain.AttendanceRecordQuery{}, err
	}
	if !startTime.IsZero() && !endTime.IsZero() && endTime.Before(startTime) {
		return domain.AttendanceRecordQuery{}, errors.New("end_time must not be before start_time")
	}

	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return domain.AttendanceRecordQuery{}, err
	}
	offset, err := parseIntQuery(c, "offset")
	if err != nil {
		return domain.AttendanceRecordQuery{}, err
	}

	return domain.AttendanceRecordQuery{
		UserID:    strings.TrimSpace(c.Query("user_id")),
		DeviceSN:  strings.TrimSpace(c.Query("device_sn")),
		StartTime: startTime,
		EndTime:   endTime,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func parseDailyAttendanceQuery(c *gin.Context) (domain.DailyAttendanceQuery, error) {
	startDate, endDate, err := parseDateSelection(c)
	if err != nil {
		return domain.DailyAttendanceQuery{}, err
	}

	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return domain.DailyAttendanceQuery{}, err
	}
	offset, err := parseIntQuery(c, "offset")
	if err != nil {
		return domain.DailyAttendanceQuery{}, err
	}

	return domain.DailyAttendanceQuery{
		UserID:    strings.TrimSpace(c.Query("user_id")),
		DeviceSN:  strings.TrimSpace(c.Query("device_sn")),
		StartDate: startDate,
		EndDate:   endDate,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func parseMonthlyAttendanceQuery(c *gin.Context) (domain.MonthlyAttendanceQuery, error) {
	month, err := parseMonthQuery(c, "month")
	if err != nil {
		return domain.MonthlyAttendanceQuery{}, err
	}

	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return domain.MonthlyAttendanceQuery{}, err
	}
	offset, err := parseIntQuery(c, "offset")
	if err != nil {
		return domain.MonthlyAttendanceQuery{}, err
	}

	return domain.MonthlyAttendanceQuery{
		UserID:   strings.TrimSpace(c.Query("user_id")),
		DeviceSN: strings.TrimSpace(c.Query("device_sn")),
		Month:    month,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

func parseAttendanceSummaryQuery(c *gin.Context) (domain.AttendanceSummaryQuery, error) {
	startDate, endDate, err := parseDateSelection(c)
	if err != nil {
		return domain.AttendanceSummaryQuery{}, err
	}

	return domain.AttendanceSummaryQuery{
		UserID:    strings.TrimSpace(c.Query("user_id")),
		DeviceSN:  strings.TrimSpace(c.Query("device_sn")),
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}

func parseAttendanceExceptionQuery(c *gin.Context) (domain.AttendanceExceptionQuery, error) {
	startDate, endDate, err := parseDateSelection(c)
	if err != nil {
		return domain.AttendanceExceptionQuery{}, err
	}

	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return domain.AttendanceExceptionQuery{}, err
	}
	offset, err := parseIntQuery(c, "offset")
	if err != nil {
		return domain.AttendanceExceptionQuery{}, err
	}

	return domain.AttendanceExceptionQuery{
		UserID:    strings.TrimSpace(c.Query("user_id")),
		DeviceSN:  strings.TrimSpace(c.Query("device_sn")),
		StartDate: startDate,
		EndDate:   endDate,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func parseAttendanceShiftQuery(c *gin.Context) (domain.AttendanceShiftQuery, error) {
	includeDisabled, err := parseBoolQuery(c, "include_disabled")
	if err != nil {
		return domain.AttendanceShiftQuery{}, err
	}
	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return domain.AttendanceShiftQuery{}, err
	}
	offset, err := parseIntQuery(c, "offset")
	if err != nil {
		return domain.AttendanceShiftQuery{}, err
	}

	return domain.AttendanceShiftQuery{
		IncludeDisabled: includeDisabled,
		Limit:           limit,
		Offset:          offset,
	}, nil
}

func parseAttendanceCalendarDayQuery(c *gin.Context) (domain.AttendanceCalendarDayQuery, error) {
	startDate, endDate, err := parseDateSelection(c)
	if err != nil {
		return domain.AttendanceCalendarDayQuery{}, err
	}

	dayType := domain.CalendarDayType(strings.TrimSpace(c.Query("day_type")))
	if dayType != "" && !validCalendarDayType(dayType) {
		return domain.AttendanceCalendarDayQuery{}, fmt.Errorf("unsupported day_type %q", dayType)
	}

	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return domain.AttendanceCalendarDayQuery{}, err
	}
	offset, err := parseIntQuery(c, "offset")
	if err != nil {
		return domain.AttendanceCalendarDayQuery{}, err
	}

	return domain.AttendanceCalendarDayQuery{
		StartDate: startDate,
		EndDate:   endDate,
		DayType:   dayType,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func parseAttendanceScheduleQuery(c *gin.Context) (domain.AttendanceScheduleQuery, error) {
	startDate, endDate, err := parseDateSelection(c)
	if err != nil {
		return domain.AttendanceScheduleQuery{}, err
	}

	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return domain.AttendanceScheduleQuery{}, err
	}
	offset, err := parseIntQuery(c, "offset")
	if err != nil {
		return domain.AttendanceScheduleQuery{}, err
	}

	return domain.AttendanceScheduleQuery{
		UserID:    strings.TrimSpace(c.Query("user_id")),
		DeviceSN:  strings.TrimSpace(c.Query("device_sn")),
		StartDate: startDate,
		EndDate:   endDate,
		Limit:     limit,
		Offset:    offset,
	}, nil
}

func parseAttendanceWeeklyScheduleQuery(c *gin.Context) (domain.AttendanceWeeklyScheduleQuery, error) {
	var weekdayPtr *time.Weekday
	weekdayValue := strings.TrimSpace(c.Query("weekday"))
	if weekdayValue != "" {
		weekday, err := parseWeekdayValue("weekday", weekdayValue)
		if err != nil {
			return domain.AttendanceWeeklyScheduleQuery{}, err
		}
		weekdayPtr = &weekday
	}

	limit, err := parseIntQuery(c, "limit")
	if err != nil {
		return domain.AttendanceWeeklyScheduleQuery{}, err
	}
	offset, err := parseIntQuery(c, "offset")
	if err != nil {
		return domain.AttendanceWeeklyScheduleQuery{}, err
	}

	return domain.AttendanceWeeklyScheduleQuery{
		UserID:   strings.TrimSpace(c.Query("user_id")),
		DeviceSN: strings.TrimSpace(c.Query("device_sn")),
		Weekday:  weekdayPtr,
		Limit:    limit,
		Offset:   offset,
	}, nil
}

func parseUnixQuery(c *gin.Context, key string) (time.Time, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return time.Time{}, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return time.Time{}, fmt.Errorf("%s must be a positive unix timestamp", key)
	}

	return time.Unix(parsed, 0), nil
}

func parseDateSelection(c *gin.Context) (time.Time, time.Time, error) {
	dateValue := strings.TrimSpace(c.Query("date"))
	startDateValue := strings.TrimSpace(c.Query("start_date"))
	endDateValue := strings.TrimSpace(c.Query("end_date"))

	if dateValue != "" {
		if startDateValue != "" || endDateValue != "" {
			return time.Time{}, time.Time{}, errors.New("date cannot be combined with start_date or end_date")
		}

		date, err := parseDateValue("date", dateValue)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}

		return date, date, nil
	}

	startDate, err := parseDateQuery(c, "start_date")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	endDate, err := parseDateQuery(c, "end_date")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !startDate.IsZero() && !endDate.IsZero() && endDate.Before(startDate) {
		return time.Time{}, time.Time{}, errors.New("end_date must not be before start_date")
	}

	return startDate, endDate, nil
}

func parseDateQuery(c *gin.Context, key string) (time.Time, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return time.Time{}, nil
	}

	return parseDateValue(key, value)
}

func parseDateValue(key string, value string) (time.Time, error) {
	parsed, err := time.ParseInLocation(queryDateLayout, value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM-DD format", key)
	}

	return parsed, nil
}

func parseMonthQuery(c *gin.Context, key string) (time.Time, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return time.Time{}, nil
	}

	parsed, err := time.ParseInLocation(queryMonthLayout, value, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must use YYYY-MM format", key)
	}

	return parsed, nil
}

func parseIntQuery(c *gin.Context, key string) (int, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}

	return parsed, nil
}

func parseBoolQuery(c *gin.Context, key string) (bool, error) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}

	return parsed, nil
}

func parseIDParam(c *gin.Context, key string) (int64, error) {
	value := strings.TrimSpace(c.Param(key))
	if value == "" {
		return 0, fmt.Errorf("%s cannot be empty", key)
	}

	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}

	return id, nil
}

func attendanceSettingsFromRequest(request attendancev1.SaveAttendanceSettingsRequest) (domain.AttendanceSettings, error) {
	weekendDays := make([]time.Weekday, 0, len(request.WeekendDays))
	for _, value := range request.WeekendDays {
		weekday, err := parseWeekdayValue("weekend_days", value)
		if err != nil {
			return domain.AttendanceSettings{}, err
		}
		weekendDays = append(weekendDays, weekday)
	}

	return domain.AttendanceSettings{
		Timezone:       strings.TrimSpace(request.Timezone),
		DefaultShiftID: strings.TrimSpace(request.DefaultShiftID),
		WeekendDays:    weekendDays,
	}, nil
}

func attendanceShiftFromRequest(request attendancev1.SaveAttendanceShiftRequest, pathID string) (domain.AttendanceShift, error) {
	id := strings.TrimSpace(request.ID)
	pathID = strings.TrimSpace(pathID)
	if pathID != "" {
		if id != "" && id != pathID {
			return domain.AttendanceShift{}, errors.New("path id must match request id")
		}
		id = pathID
	}

	start, err := parseClockTimeValue("start_time", request.StartTime)
	if err != nil {
		return domain.AttendanceShift{}, err
	}
	end, err := parseClockTimeValue("end_time", request.EndTime)
	if err != nil {
		return domain.AttendanceShift{}, err
	}

	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}

	return domain.AttendanceShift{
		ID:              id,
		Name:            strings.TrimSpace(request.Name),
		Start:           start,
		End:             end,
		LateGrace:       time.Duration(request.LateGraceMinutes) * time.Minute,
		EarlyLeaveGrace: time.Duration(request.EarlyLeaveGraceMinutes) * time.Minute,
		Flexible:        time.Duration(request.FlexibleMinutes) * time.Minute,
		Enabled:         enabled,
	}, nil
}

func attendanceCalendarDayFromRequest(request attendancev1.SaveAttendanceCalendarDayRequest, pathDate string) (domain.AttendanceCalendarDay, error) {
	dateValue := strings.TrimSpace(request.Date)
	pathDate = strings.TrimSpace(pathDate)
	if pathDate != "" {
		if dateValue != "" && dateValue != pathDate {
			return domain.AttendanceCalendarDay{}, errors.New("path date must match request date")
		}
		dateValue = pathDate
	}
	if dateValue == "" {
		return domain.AttendanceCalendarDay{}, errors.New("date cannot be empty")
	}

	date, err := parseDateValue("date", dateValue)
	if err != nil {
		return domain.AttendanceCalendarDay{}, err
	}

	dayType := domain.CalendarDayType(strings.TrimSpace(request.DayType))
	if !validCalendarDayType(dayType) {
		return domain.AttendanceCalendarDay{}, fmt.Errorf("unsupported day_type %q", dayType)
	}

	return domain.AttendanceCalendarDay{
		Date:    date,
		DayType: dayType,
		Name:    strings.TrimSpace(request.Name),
	}, nil
}

func attendanceScheduleFromRequest(request attendancev1.SaveAttendanceScheduleRequest, pathID string) (domain.ManagedAttendanceSchedule, error) {
	id, err := mergeRequestID(request.ID, pathID)
	if err != nil {
		return domain.ManagedAttendanceSchedule{}, err
	}
	if strings.TrimSpace(request.Date) == "" {
		return domain.ManagedAttendanceSchedule{}, errors.New("date cannot be empty")
	}
	date, err := parseDateValue("date", request.Date)
	if err != nil {
		return domain.ManagedAttendanceSchedule{}, err
	}

	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}

	return domain.ManagedAttendanceSchedule{
		ID:       id,
		UserID:   strings.TrimSpace(request.UserID),
		DeviceSN: strings.TrimSpace(request.DeviceSN),
		Date:     date,
		ShiftID:  strings.TrimSpace(request.ShiftID),
		Rest:     request.Rest,
		Reason:   strings.TrimSpace(request.Reason),
		Enabled:  enabled,
	}, nil
}

func attendanceWeeklyScheduleFromRequest(request attendancev1.SaveAttendanceWeeklyScheduleRequest, pathID string) (domain.ManagedAttendanceWeeklySchedule, error) {
	id, err := mergeRequestID(request.ID, pathID)
	if err != nil {
		return domain.ManagedAttendanceWeeklySchedule{}, err
	}

	weekday, err := parseWeekdayValue("weekday", request.Weekday)
	if err != nil {
		return domain.ManagedAttendanceWeeklySchedule{}, err
	}

	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}

	return domain.ManagedAttendanceWeeklySchedule{
		ID:       id,
		UserID:   strings.TrimSpace(request.UserID),
		DeviceSN: strings.TrimSpace(request.DeviceSN),
		Weekday:  weekday,
		ShiftID:  strings.TrimSpace(request.ShiftID),
		Rest:     request.Rest,
		Reason:   strings.TrimSpace(request.Reason),
		Enabled:  enabled,
	}, nil
}

func mergeRequestID(requestID int64, pathID string) (int64, error) {
	pathID = strings.TrimSpace(pathID)
	if pathID == "" {
		return requestID, nil
	}

	id, err := strconv.ParseInt(pathID, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("id must be a positive integer")
	}
	if requestID > 0 && requestID != id {
		return 0, errors.New("path id must match request id")
	}

	return id, nil
}

func parseClockTimeValue(key string, value string) (domain.ClockTime, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return domain.ClockTime{}, fmt.Errorf("%s cannot be empty", key)
	}

	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return domain.ClockTime{}, fmt.Errorf("%s must use HH:MM format", key)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return domain.ClockTime{}, fmt.Errorf("%s hour must be between 0 and 23", key)
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return domain.ClockTime{}, fmt.Errorf("%s minute must be between 0 and 59", key)
	}

	return domain.ClockTime{Hour: hour, Minute: minute}, nil
}

func parseWeekdayValue(key string, value string) (time.Weekday, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "sunday", "sun":
		return time.Sunday, nil
	case "1", "monday", "mon":
		return time.Monday, nil
	case "2", "tuesday", "tue":
		return time.Tuesday, nil
	case "3", "wednesday", "wed":
		return time.Wednesday, nil
	case "4", "thursday", "thu":
		return time.Thursday, nil
	case "5", "friday", "fri":
		return time.Friday, nil
	case "6", "saturday", "sat":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("%s must be one of sunday,monday,tuesday,wednesday,thursday,friday,saturday", key)
	}
}

func validCalendarDayType(dayType domain.CalendarDayType) bool {
	switch dayType {
	case domain.CalendarDayTypeHoliday, domain.CalendarDayTypeWorkday, domain.CalendarDayTypeRestDay:
		return true
	default:
		return false
	}
}

func toAttendanceSettingsDTO(settings domain.AttendanceSettings) attendancev1.AttendanceSettingsDTO {
	return attendancev1.AttendanceSettingsDTO{
		Timezone:       settings.Timezone,
		DefaultShiftID: settings.DefaultShiftID,
		WeekendDays:    weekdaysToStrings(settings.WeekendDays),
	}
}

func toAttendanceShiftDTOs(shifts []domain.AttendanceShift) []attendancev1.AttendanceShiftDTO {
	dtos := make([]attendancev1.AttendanceShiftDTO, 0, len(shifts))
	for _, shift := range shifts {
		dtos = append(dtos, toAttendanceShiftDTO(shift))
	}

	return dtos
}

func toAttendanceShiftDTO(shift domain.AttendanceShift) attendancev1.AttendanceShiftDTO {
	return attendancev1.AttendanceShiftDTO{
		ID:                     shift.ID,
		Name:                   shift.Name,
		StartTime:              clockTimeToString(shift.Start),
		EndTime:                clockTimeToString(shift.End),
		LateGraceMinutes:       int(shift.LateGrace.Minutes()),
		EarlyLeaveGraceMinutes: int(shift.EarlyLeaveGrace.Minutes()),
		FlexibleMinutes:        int(shift.Flexible.Minutes()),
		Enabled:                shift.Enabled,
	}
}

func toAttendanceCalendarDayDTOs(days []domain.AttendanceCalendarDay) []attendancev1.AttendanceCalendarDayDTO {
	dtos := make([]attendancev1.AttendanceCalendarDayDTO, 0, len(days))
	for _, day := range days {
		dtos = append(dtos, toAttendanceCalendarDayDTO(day))
	}

	return dtos
}

func toAttendanceCalendarDayDTO(day domain.AttendanceCalendarDay) attendancev1.AttendanceCalendarDayDTO {
	return attendancev1.AttendanceCalendarDayDTO{
		Date:    day.Date.Format(queryDateLayout),
		DayType: day.DayType.String(),
		Name:    day.Name,
	}
}

func toAttendanceScheduleDTOs(schedules []domain.ManagedAttendanceSchedule) []attendancev1.AttendanceScheduleDTO {
	dtos := make([]attendancev1.AttendanceScheduleDTO, 0, len(schedules))
	for _, schedule := range schedules {
		dtos = append(dtos, toAttendanceScheduleDTO(schedule))
	}

	return dtos
}

func toAttendanceScheduleDTO(schedule domain.ManagedAttendanceSchedule) attendancev1.AttendanceScheduleDTO {
	return attendancev1.AttendanceScheduleDTO{
		ID:       schedule.ID,
		UserID:   schedule.UserID,
		DeviceSN: schedule.DeviceSN,
		Date:     schedule.Date.Format(queryDateLayout),
		ShiftID:  schedule.ShiftID,
		Rest:     schedule.Rest,
		Reason:   schedule.Reason,
		Enabled:  schedule.Enabled,
	}
}

func toAttendanceWeeklyScheduleDTOs(schedules []domain.ManagedAttendanceWeeklySchedule) []attendancev1.AttendanceWeeklyScheduleDTO {
	dtos := make([]attendancev1.AttendanceWeeklyScheduleDTO, 0, len(schedules))
	for _, schedule := range schedules {
		dtos = append(dtos, toAttendanceWeeklyScheduleDTO(schedule))
	}

	return dtos
}

func toAttendanceWeeklyScheduleDTO(schedule domain.ManagedAttendanceWeeklySchedule) attendancev1.AttendanceWeeklyScheduleDTO {
	return attendancev1.AttendanceWeeklyScheduleDTO{
		ID:       schedule.ID,
		UserID:   schedule.UserID,
		DeviceSN: schedule.DeviceSN,
		Weekday:  weekdayToString(schedule.Weekday),
		ShiftID:  schedule.ShiftID,
		Rest:     schedule.Rest,
		Reason:   schedule.Reason,
		Enabled:  schedule.Enabled,
	}
}

func weekdaysToStrings(weekdays []time.Weekday) []string {
	values := make([]string, 0, len(weekdays))
	for _, weekday := range weekdays {
		values = append(values, weekdayToString(weekday))
	}

	return values
}

func weekdayToString(weekday time.Weekday) string {
	switch weekday {
	case time.Sunday:
		return "sunday"
	case time.Monday:
		return "monday"
	case time.Tuesday:
		return "tuesday"
	case time.Wednesday:
		return "wednesday"
	case time.Thursday:
		return "thursday"
	case time.Friday:
		return "friday"
	case time.Saturday:
		return "saturday"
	default:
		return ""
	}
}

func clockTimeToString(value domain.ClockTime) string {
	return fmt.Sprintf("%02d:%02d", value.Hour, value.Minute)
}

func toAttendanceRecordDTOs(records []domain.AttendanceRecord) []attendancev1.AttendanceRecordDTO {
	dtos := make([]attendancev1.AttendanceRecordDTO, 0, len(records))
	for _, record := range records {
		dtos = append(dtos, toAttendanceRecordDTO(record))
	}

	return dtos
}

func toAttendanceRecordDTO(record domain.AttendanceRecord) attendancev1.AttendanceRecordDTO {
	return attendancev1.AttendanceRecordDTO{
		UserID:        record.UserID,
		UserName:      record.CardName,
		DeviceSN:      record.DeviceSN,
		Direction:     string(record.Direction),
		Method:        int32(record.Method),
		MethodName:    record.Method.String(),
		Status:        record.Status,
		EventTime:     timeToUnixSeconds(record.EventTime),
		ReceivedAt:    timeToUnixSeconds(record.ReceivedAt),
		HasSnapshot:   record.ImageCount > 0,
		SnapshotCount: record.ImageCount,
	}
}

func toDailyAttendanceDTOs(records []domain.DailyAttendance) []attendancev1.DailyAttendanceDTO {
	dtos := make([]attendancev1.DailyAttendanceDTO, 0, len(records))
	for _, record := range records {
		dtos = append(dtos, toDailyAttendanceDTO(record))
	}

	return dtos
}

func toDailyAttendanceDTO(record domain.DailyAttendance) attendancev1.DailyAttendanceDTO {
	return attendancev1.DailyAttendanceDTO{
		Date:              record.Date.Format(queryDateLayout),
		UserID:            record.UserID,
		UserName:          record.UserName,
		DeviceSN:          record.DeviceSN,
		ShiftID:           record.ShiftID,
		ShiftName:         record.ShiftName,
		IsWorkday:         record.IsWorkday,
		NonWorkdayReason:  record.NonWorkdayReason,
		Status:            record.Status.String(),
		Exceptions:        dailyAttendanceExceptions(record.Exceptions),
		IsAbnormal:        record.IsAbnormal(),
		WorkStartAt:       timeToUnixSeconds(record.WorkStartAt),
		WorkEndAt:         timeToUnixSeconds(record.WorkEndAt),
		FirstEntryAt:      timeToUnixSeconds(record.FirstEntryAt),
		LastExitAt:        timeToUnixSeconds(record.LastExitAt),
		LateSeconds:       int64(record.LateDuration.Seconds()),
		EarlyLeaveSeconds: int64(record.EarlyLeaveDuration.Seconds()),
		RecordCount:       record.RecordCount,
		SnapshotCount:     record.SnapshotCount,
	}
}

func toMonthlyAttendanceDTOs(records []domain.MonthlyAttendance) []attendancev1.MonthlyAttendanceDTO {
	dtos := make([]attendancev1.MonthlyAttendanceDTO, 0, len(records))
	for _, record := range records {
		dtos = append(dtos, toMonthlyAttendanceDTO(record))
	}

	return dtos
}

func toMonthlyAttendanceDTO(record domain.MonthlyAttendance) attendancev1.MonthlyAttendanceDTO {
	return attendancev1.MonthlyAttendanceDTO{
		Month:    record.Month.Format(queryMonthLayout),
		UserID:   record.UserID,
		UserName: record.UserName,
		DeviceSN: record.DeviceSN,
		Stats:    toAttendanceStatsDTO(record.Stats),
	}
}

func toAttendanceSummaryDTO(summary domain.AttendanceSummary) attendancev1.AttendanceSummaryDTO {
	return attendancev1.AttendanceSummaryDTO{
		StartDate: summary.StartDate.Format(queryDateLayout),
		EndDate:   summary.EndDate.Format(queryDateLayout),
		UserCount: summary.UserCount,
		Stats:     toAttendanceStatsDTO(summary.Stats),
	}
}

func toAttendanceStatsDTO(stats domain.AttendanceStats) attendancev1.AttendanceStatsDTO {
	return attendancev1.AttendanceStatsDTO{
		TotalDays:              stats.TotalDays,
		WorkDays:               stats.WorkDays,
		RestDays:               stats.RestDays,
		NormalDays:             stats.NormalDays,
		AbnormalDays:           stats.AbnormalDays,
		LateDays:               stats.LateDays,
		EarlyLeaveDays:         stats.EarlyLeaveDays,
		LateAndEarlyLeaveDays:  stats.LateAndEarlyLeaveDays,
		MissingCheckInDays:     stats.MissingCheckInDays,
		MissingCheckOutDays:    stats.MissingCheckOutDays,
		AbsentDays:             stats.AbsentDays,
		RecordCount:            stats.RecordCount,
		SnapshotCount:          stats.SnapshotCount,
		TotalLateSeconds:       int64(stats.TotalLateDuration.Seconds()),
		TotalEarlyLeaveSeconds: int64(stats.TotalEarlyLeaveDuration.Seconds()),
	}
}

func dailyAttendanceExceptions(exceptions []domain.DailyAttendanceException) []string {
	values := make([]string, 0, len(exceptions))
	for _, exception := range exceptions {
		values = append(values, exception.String())
	}

	return values
}

func timeToUnixSeconds(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}

	return value.Unix()
}

type LoggingConsumer struct {
	logger *slog.Logger
}

func NewLoggingConsumer(logger *slog.Logger) *LoggingConsumer {
	if logger == nil {
		logger = slog.Default()
	}

	return &LoggingConsumer{logger: logger}
}

func (c *LoggingConsumer) HandleDevicePayload(ctx context.Context, payload *parser.ParsedPayload) error {
	if payload == nil {
		return errors.New("payload is nil")
	}

	now := time.Now()
	for _, event := range payload.Events {
		c.logger.InfoContext(
			ctx,
			"device event parsed",
			"code", event.Code,
			"action", event.Action,
			"index", event.Index,
			"data_source", event.DataSource,
			"received_at", now.Format(time.RFC3339),
		)
	}

	return nil
}
