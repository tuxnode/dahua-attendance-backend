package repository

import (
	"context"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
)

type Repository interface {
	SaveAttendanceRecord(ctx context.Context, record domain.AttendanceRecord) error
	SaveDoorStatusRecord(ctx context.Context, record domain.DoorStatusRecord) error
	ListAttendanceRecords(ctx context.Context, query domain.AttendanceRecordQuery) ([]domain.AttendanceRecord, error)
	GetAttendanceSettings(ctx context.Context) (domain.AttendanceSettings, error)
	SaveAttendanceSettings(ctx context.Context, settings domain.AttendanceSettings) error
	ListAttendanceShifts(ctx context.Context, query domain.AttendanceShiftQuery) ([]domain.AttendanceShift, error)
	SaveAttendanceShift(ctx context.Context, shift domain.AttendanceShift) error
	DeleteAttendanceShift(ctx context.Context, id string) error
	ListAttendanceCalendarDays(ctx context.Context, query domain.AttendanceCalendarDayQuery) ([]domain.AttendanceCalendarDay, error)
	SaveAttendanceCalendarDay(ctx context.Context, day domain.AttendanceCalendarDay) error
	DeleteAttendanceCalendarDay(ctx context.Context, date time.Time) error
	ListAttendanceSchedules(ctx context.Context, query domain.AttendanceScheduleQuery) ([]domain.ManagedAttendanceSchedule, error)
	SaveAttendanceSchedule(ctx context.Context, schedule domain.ManagedAttendanceSchedule) (domain.ManagedAttendanceSchedule, error)
	DeleteAttendanceSchedule(ctx context.Context, id int64) error
	ListAttendanceWeeklySchedules(ctx context.Context, query domain.AttendanceWeeklyScheduleQuery) ([]domain.ManagedAttendanceWeeklySchedule, error)
	SaveAttendanceWeeklySchedule(ctx context.Context, schedule domain.ManagedAttendanceWeeklySchedule) (domain.ManagedAttendanceWeeklySchedule, error)
	DeleteAttendanceWeeklySchedule(ctx context.Context, id int64) error
	SaveAttendanceCorrection(ctx context.Context, correction domain.AttendanceCorrection) (domain.AttendanceCorrection, error)
	SaveMonthlyAttendanceResult(ctx context.Context, result domain.MonthlyAttendanceDailyResult) (domain.MonthlyAttendanceDailyResult, error)
	ListMonthlyAttendanceResults(ctx context.Context, query domain.MonthlyAttendanceResultQuery) ([]domain.MonthlyAttendanceDailyResult, error)
}
