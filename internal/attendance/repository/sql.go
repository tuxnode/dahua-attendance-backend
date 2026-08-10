package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
)

type SQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
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
	dedup_block_id,
	image_count,
	raw_event,
	received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	raw_event = VALUES(raw_event),
	received_at = VALUES(received_at)`

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
		dedupBlockID(record.BlockID),
		record.ImageCount,
		record.RawEvent,
		record.ReceivedAt,
	)
	if err != nil {
		return fmt.Errorf("repository: save attendance record: %w", err)
	}

	return nil
}

func dedupBlockID(blockID int64) any {
	if blockID <= 0 {
		return nil
	}

	return blockID
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

func (r *SQLRepository) ListAttendanceRecords(ctx context.Context, query domain.AttendanceRecordQuery) ([]domain.AttendanceRecord, error) {
	sqlQuery, args := buildListAttendanceRecordsQuery(query)

	rows, err := r.executor.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: list attendance records: %w", err)
	}
	defer rows.Close()

	records := make([]domain.AttendanceRecord, 0)
	for rows.Next() {
		record, err := scanAttendanceRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: iterate attendance records: %w", err)
	}

	return records, nil
}

func buildListAttendanceRecordsQuery(filter domain.AttendanceRecordQuery) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0, 6)

	builder.WriteString(`
SELECT
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
	received_at
FROM attendance_records
WHERE 1 = 1`)

	if filter.UserID != "" {
		builder.WriteString("\n  AND user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.DeviceSN != "" {
		builder.WriteString("\n  AND device_sn = ?")
		args = append(args, filter.DeviceSN)
	}
	if !filter.StartTime.IsZero() {
		builder.WriteString("\n  AND event_time >= ?")
		args = append(args, filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		builder.WriteString("\n  AND event_time <= ?")
		args = append(args, filter.EndTime)
	}

	builder.WriteString("\nORDER BY event_time DESC, id DESC")
	builder.WriteString("\nLIMIT ? OFFSET ?")
	args = append(args, filter.Limit, filter.Offset)

	return builder.String(), args
}

type attendanceRecordScanner interface {
	Scan(dest ...any) error
}

func scanAttendanceRecord(scanner attendanceRecordScanner) (domain.AttendanceRecord, error) {
	var record domain.AttendanceRecord
	var method int32
	var direction string
	var dataSource string

	if err := scanner.Scan(
		&record.DeviceSN,
		&record.UserID,
		&record.CardName,
		&record.CardNo,
		&method,
		&direction,
		&record.Status,
		&record.EventTime,
		&record.CreateTime,
		&record.UTC,
		&record.RealUTC,
		&dataSource,
		&record.Index,
		&record.Door,
		&record.ReaderID,
		&record.CardType,
		&record.UserType,
		&record.ErrorCode,
		&record.BlockID,
		&record.ImageCount,
		&record.ReceivedAt,
	); err != nil {
		return domain.AttendanceRecord{}, fmt.Errorf("repository: scan attendance record: %w", err)
	}

	record.Method = domain.AccessMethod(method)
	record.Direction = domain.AccessDirection(direction)
	record.DataSource = domain.DataSource(dataSource)

	return record, nil
}
