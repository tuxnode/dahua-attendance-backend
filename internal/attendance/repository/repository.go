package repository

import (
	"context"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
)

type Repository interface {
	SaveAttendanceRecord(ctx context.Context, record domain.AttendanceRecord) error
	SaveDoorStatusRecord(ctx context.Context, record domain.DoorStatusRecord) error
	ListAttendanceRecords(ctx context.Context, query domain.AttendanceRecordQuery) ([]domain.AttendanceRecord, error)
}
