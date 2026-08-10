package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
)

type SQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type SQLRepository struct {
	executor SQLExecutor
}

func NewSQLRepository(executor SQLExecutor) (*SQLRepository, error) {
	if executor == nil {
		return nil, errors.New("repository: sql executor is nil")
	}

	return &SQLRepository{executor: executor}, nil
}

func (r *SQLRepository) SaveAttendanceRecord(ctx context.Context, record domain.AttendanceRecord) error {
	const query = `
INSERT INTO attendance_records (
	device_sn,
	user_id,
	card_name,
	card_no,
	method,
	direction,
	status,
	event_time,
	create_time,
	utc,
	real_utc,
	data_source,
	channel_index,
	door,
	reader_id,
	card_type,
	user_type,
	error_code,
	block_id,
	image_count,
	raw_event,
	received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.executor.ExecContext(
		ctx,
		query,
		record.DeviceSN,
		record.UserID,
		record.CardName,
		record.CardNo,
		int32(record.Method),
		string(record.Direction),
		record.Status,
		record.EventTime,
		record.CreateTime,
		record.UTC,
		record.RealUTC,
		string(record.DataSource),
		record.Index,
		record.Door,
		record.ReaderID,
		record.CardType,
		record.UserType,
		record.ErrorCode,
		record.BlockID,
		record.ImageCount,
		record.RawEvent,
		record.ReceivedAt,
	)
	if err != nil {
		return fmt.Errorf("repository: save attendance record: %w", err)
	}

	return nil
}

func (r *SQLRepository) SaveDoorStatusRecord(ctx context.Context, record domain.DoorStatusRecord) error {
	const query = `
INSERT INTO door_status_records (
	device_sn,
	status,
	event_time,
	utc,
	real_utc,
	data_source,
	channel_index,
	raw_event,
	received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.executor.ExecContext(
		ctx,
		query,
		record.DeviceSN,
		string(record.Status),
		record.EventTime,
		record.UTC,
		record.RealUTC,
		string(record.DataSource),
		record.Index,
		record.RawEvent,
		record.ReceivedAt,
	)
	if err != nil {
		return fmt.Errorf("repository: save door status record: %w", err)
	}

	return nil
}
