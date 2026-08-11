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

type AttendanceService interface {
	EventConsumer
	AttendanceQueryService
	DailyAttendanceQueryService
	AttendanceStatsService
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
