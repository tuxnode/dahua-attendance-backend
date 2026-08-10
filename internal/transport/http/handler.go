package transporthttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/parser"
)

const (
	DefaultMaxBodyBytes = 8 << 20
	DefaultPath         = "/"
	DeviceEventsPath    = "/api/v1/device/events"
)

type EventConsumer interface {
	HandleDevicePayload(ctx context.Context, payload *parser.ParsedPayload) error
}

type Handler struct {
	consumer     EventConsumer
	logger       *slog.Logger
	maxBodyBytes int64
}

type Option func(*Handler)

func NewHandler(consumer EventConsumer, opts ...Option) *Handler {
	handler := &Handler{
		consumer:     consumer,
		logger:       slog.Default(),
		maxBodyBytes: DefaultMaxBodyBytes,
	}

	for _, opt := range opts {
		opt(handler)
	}

	return handler
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

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(DefaultPath, h.HandleDeviceEvents)
	mux.HandleFunc(DeviceEventsPath, h.HandleDeviceEvents)
}

func (h *Handler) HandleDeviceEvents(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != DefaultPath && r.URL.Path != DeviceEventsPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body := http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	defer body.Close()

	payload, err := parser.Parse(
		r.Header.Get("Content-Type"),
		r.Header.Get("Content-Encoding"),
		body,
	)
	if err != nil {
		h.logger.WarnContext(
			r.Context(),
			"failed to parse device payload",
			"remote_addr", r.RemoteAddr,
			"content_type", r.Header.Get("Content-Type"),
			"content_encoding", r.Header.Get("Content-Encoding"),
			"error", err,
		)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if h.consumer != nil {
		if err := h.consumer.HandleDevicePayload(r.Context(), payload); err != nil {
			h.logger.ErrorContext(
				r.Context(),
				"failed to handle device payload",
				"remote_addr", r.RemoteAddr,
				"events", len(payload.Events),
				"images", len(payload.Images),
				"error", err,
			)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	h.logger.InfoContext(
		r.Context(),
		"device payload accepted",
		"remote_addr", r.RemoteAddr,
		"events", len(payload.Events),
		"images", len(payload.Images),
	)

	writeDeviceSuccess(w)
}

func writeDeviceSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(w).Encode(domain.Response{
		Code:    0,
		Message: "success",
	})
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
