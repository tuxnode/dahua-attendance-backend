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
	DefaultMaxBodyBytes   = 8 << 20
	DefaultPath           = "/"
	DeviceEventsPath      = "/api/v1/device/events"
	AttendanceRecordsPath = "/api/v1/attendance/records"
	DailyAttendancePath   = "/api/v1/attendance/daily"

	queryDateLayout = "2006-01-02"
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

type AttendanceService interface {
	EventConsumer
	AttendanceQueryService
	DailyAttendanceQueryService
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
	dateValue := strings.TrimSpace(c.Query("date"))
	startDateValue := strings.TrimSpace(c.Query("start_date"))
	endDateValue := strings.TrimSpace(c.Query("end_date"))

	var startDate time.Time
	var endDate time.Time
	var err error

	if dateValue != "" {
		if startDateValue != "" || endDateValue != "" {
			return domain.DailyAttendanceQuery{}, errors.New("date cannot be combined with start_date or end_date")
		}

		startDate, err = parseDateValue("date", dateValue)
		if err != nil {
			return domain.DailyAttendanceQuery{}, err
		}
		endDate = startDate
	} else {
		startDate, err = parseDateQuery(c, "start_date")
		if err != nil {
			return domain.DailyAttendanceQuery{}, err
		}
		endDate, err = parseDateQuery(c, "end_date")
		if err != nil {
			return domain.DailyAttendanceQuery{}, err
		}
		if !startDate.IsZero() && !endDate.IsZero() && endDate.Before(startDate) {
			return domain.DailyAttendanceQuery{}, errors.New("end_date must not be before start_date")
		}
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
