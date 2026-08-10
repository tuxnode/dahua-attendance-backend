package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/parser"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/repository"
)

type Service struct {
	repository repository.Repository
	logger     *slog.Logger
	now        func() time.Time
}

type Option func(*Service)

func New(repository repository.Repository, opts ...Option) *Service {
	service := &Service{
		repository: repository,
		logger:     slog.Default(),
		now:        time.Now,
	}

	for _, opt := range opts {
		opt(service)
	}

	return service
}

func WithLogger(logger *slog.Logger) Option {
	return func(service *Service) {
		if logger != nil {
			service.logger = logger
		}
	}
}

func WithNow(now func() time.Time) Option {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

func (s *Service) HandleDevicePayload(ctx context.Context, payload *parser.ParsedPayload) error {
	if payload == nil {
		return errors.New("service: payload is nil")
	}
	if s.repository == nil {
		return errors.New("service: repository is nil")
	}

	for index, envelope := range payload.Events {
		if err := s.handleEvent(ctx, envelope); err != nil {
			return fmt.Errorf("service: handle event %d: %w", index, err)
		}
	}

	return nil
}

func (s *Service) handleEvent(ctx context.Context, envelope domain.EventEnvelope) error {
	switch envelope.Code {
	case domain.EventCodeAccessControl:
		return s.handleAccessControl(ctx, envelope)
	case domain.EventCodeDoorStatus:
		return s.handleDoorStatus(ctx, envelope)
	default:
		return fmt.Errorf("unsupported event code %q", envelope.Code)
	}
}

func (s *Service) handleAccessControl(ctx context.Context, envelope domain.EventEnvelope) error {
	var event domain.AccessControlEvent
	if err := json.Unmarshal(envelope.Data, &event); err != nil {
		return fmt.Errorf("decode access control data: %w", err)
	}

	record := domain.AttendanceRecord{
		DeviceSN:   event.SN,
		UserID:     event.UserID,
		CardName:   event.CardName,
		CardNo:     event.CardNo,
		Method:     event.Method,
		Direction:  event.Type,
		Status:     event.Status,
		EventTime:  eventTime(event.CreateTime, event.RealUTC, event.UTC, s.now),
		CreateTime: event.CreateTime,
		UTC:        event.UTC,
		RealUTC:    event.RealUTC,
		DataSource: envelope.DataSource,
		Index:      envelope.Index,
		Door:       event.Door,
		ReaderID:   event.ReaderID,
		CardType:   event.CardType,
		UserType:   event.UserType,
		ErrorCode:  event.ErrorCode,
		BlockID:    event.BlockID,
		ImageCount: len(event.ImageInfo),
		RawEvent:   cloneBytes(envelope.Data),
		ReceivedAt: s.now(),
	}

	if err := s.repository.SaveAttendanceRecord(ctx, record); err != nil {
		return err
	}

	s.logger.InfoContext(
		ctx,
		"attendance record saved",
		"method", record.Method,
		"direction", record.Direction,
		"status", record.Status,
		"image_count", record.ImageCount,
	)

	return nil
}

func (s *Service) handleDoorStatus(ctx context.Context, envelope domain.EventEnvelope) error {
	var event domain.DoorStatusEvent
	if err := json.Unmarshal(envelope.Data, &event); err != nil {
		return fmt.Errorf("decode door status data: %w", err)
	}

	record := domain.DoorStatusRecord{
		DeviceSN:   event.SN,
		Status:     event.Status,
		EventTime:  eventTime(0, event.RealUTC, event.UTC, s.now),
		UTC:        event.UTC,
		RealUTC:    event.RealUTC,
		DataSource: envelope.DataSource,
		Index:      envelope.Index,
		RawEvent:   cloneBytes(envelope.Data),
		ReceivedAt: s.now(),
	}

	if err := s.repository.SaveDoorStatusRecord(ctx, record); err != nil {
		return err
	}

	s.logger.InfoContext(
		ctx,
		"door status record saved",
		"status", record.Status,
		"index", record.Index,
	)

	return nil
}

func eventTime(createTime int64, realUTC int64, utc int64, now func() time.Time) time.Time {
	for _, unixSeconds := range []int64{createTime, realUTC, utc} {
		if unixSeconds > 0 {
			return time.Unix(unixSeconds, 0)
		}
	}

	return now()
}

func cloneBytes(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}

	cloned := make([]byte, len(data))
	copy(cloned, data)
	return cloned
}
