package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
)

type SQLExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func (r *SQLRepository) GetAttendanceSettings(ctx context.Context) (domain.AttendanceSettings, error) {
	const query = `
SELECT timezone, default_shift_id, weekend_days
FROM attendance_settings
WHERE id = 1`

	rows, err := r.executor.QueryContext(ctx, query)
	if err != nil {
		return domain.AttendanceSettings{}, fmt.Errorf("repository: get attendance settings: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return domain.AttendanceSettings{}, sql.ErrNoRows
	}

	var settings domain.AttendanceSettings
	var weekendDays string
	if err := rows.Scan(&settings.Timezone, &settings.DefaultShiftID, &weekendDays); err != nil {
		return domain.AttendanceSettings{}, fmt.Errorf("repository: scan attendance settings: %w", err)
	}
	if err := rows.Err(); err != nil {
		return domain.AttendanceSettings{}, fmt.Errorf("repository: iterate attendance settings: %w", err)
	}

	settings.WeekendDays = parseWeekdayList(weekendDays)
	return settings, nil
}

func (r *SQLRepository) SaveAttendanceSettings(ctx context.Context, settings domain.AttendanceSettings) error {
	const query = `
INSERT INTO attendance_settings (
	id,
	timezone,
	default_shift_id,
	weekend_days
) VALUES (1, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	timezone = VALUES(timezone),
	default_shift_id = VALUES(default_shift_id),
	weekend_days = VALUES(weekend_days)`

	_, err := r.executor.ExecContext(
		ctx,
		query,
		settings.Timezone,
		settings.DefaultShiftID,
		formatWeekdayList(settings.WeekendDays),
	)
	if err != nil {
		return fmt.Errorf("repository: save attendance settings: %w", err)
	}

	return nil
}

func (r *SQLRepository) ListAttendanceShifts(ctx context.Context, query domain.AttendanceShiftQuery) ([]domain.AttendanceShift, error) {
	sqlQuery, args := buildListAttendanceShiftsQuery(query)
	rows, err := r.executor.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: list attendance shifts: %w", err)
	}
	defer rows.Close()

	shifts := make([]domain.AttendanceShift, 0)
	for rows.Next() {
		shift, err := scanAttendanceShift(rows)
		if err != nil {
			return nil, err
		}
		shifts = append(shifts, shift)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: iterate attendance shifts: %w", err)
	}

	return shifts, nil
}

func buildListAttendanceShiftsQuery(filter domain.AttendanceShiftQuery) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0, 2)

	builder.WriteString(`
SELECT
	id,
	name,
	start_time,
	end_time,
	late_grace_minutes,
	early_leave_grace_minutes,
	flexible_minutes,
	enabled
FROM attendance_shifts
WHERE 1 = 1`)
	if !filter.IncludeDisabled {
		builder.WriteString("\n  AND enabled = TRUE")
	}
	builder.WriteString("\nORDER BY id ASC")
	if filter.Limit > 0 {
		builder.WriteString("\nLIMIT ? OFFSET ?")
		args = append(args, filter.Limit, filter.Offset)
	}

	return builder.String(), args
}

func scanAttendanceShift(scanner attendanceRecordScanner) (domain.AttendanceShift, error) {
	var shift domain.AttendanceShift
	var startTime string
	var endTime string
	var lateGraceMinutes int
	var earlyLeaveGraceMinutes int
	var flexibleMinutes int
	if err := scanner.Scan(
		&shift.ID,
		&shift.Name,
		&startTime,
		&endTime,
		&lateGraceMinutes,
		&earlyLeaveGraceMinutes,
		&flexibleMinutes,
		&shift.Enabled,
	); err != nil {
		return domain.AttendanceShift{}, fmt.Errorf("repository: scan attendance shift: %w", err)
	}

	shift.Start = parseClockTimeOrZero(startTime)
	shift.End = parseClockTimeOrZero(endTime)
	shift.LateGrace = time.Duration(lateGraceMinutes) * time.Minute
	shift.EarlyLeaveGrace = time.Duration(earlyLeaveGraceMinutes) * time.Minute
	shift.Flexible = time.Duration(flexibleMinutes) * time.Minute

	return shift, nil
}

func (r *SQLRepository) SaveAttendanceShift(ctx context.Context, shift domain.AttendanceShift) error {
	const query = `
INSERT INTO attendance_shifts (
	id,
	name,
	start_time,
	end_time,
	late_grace_minutes,
	early_leave_grace_minutes,
	flexible_minutes,
	enabled
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	name = VALUES(name),
	start_time = VALUES(start_time),
	end_time = VALUES(end_time),
	late_grace_minutes = VALUES(late_grace_minutes),
	early_leave_grace_minutes = VALUES(early_leave_grace_minutes),
	flexible_minutes = VALUES(flexible_minutes),
	enabled = VALUES(enabled)`

	_, err := r.executor.ExecContext(
		ctx,
		query,
		shift.ID,
		shift.Name,
		formatClockTime(shift.Start),
		formatClockTime(shift.End),
		int(shift.LateGrace.Minutes()),
		int(shift.EarlyLeaveGrace.Minutes()),
		int(shift.Flexible.Minutes()),
		shift.Enabled,
	)
	if err != nil {
		return fmt.Errorf("repository: save attendance shift: %w", err)
	}

	return nil
}

func (r *SQLRepository) DeleteAttendanceShift(ctx context.Context, id string) error {
	const query = `DELETE FROM attendance_shifts WHERE id = ?`
	if _, err := r.executor.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("repository: delete attendance shift: %w", err)
	}

	return nil
}

func (r *SQLRepository) ListAttendanceCalendarDays(ctx context.Context, query domain.AttendanceCalendarDayQuery) ([]domain.AttendanceCalendarDay, error) {
	sqlQuery, args := buildListAttendanceCalendarDaysQuery(query)
	rows, err := r.executor.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: list attendance calendar days: %w", err)
	}
	defer rows.Close()

	days := make([]domain.AttendanceCalendarDay, 0)
	for rows.Next() {
		day, err := scanAttendanceCalendarDay(rows)
		if err != nil {
			return nil, err
		}
		days = append(days, day)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: iterate attendance calendar days: %w", err)
	}

	return days, nil
}

func buildListAttendanceCalendarDaysQuery(filter domain.AttendanceCalendarDayQuery) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0, 5)

	builder.WriteString(`
SELECT calendar_date, day_type, name
FROM attendance_calendar_days
WHERE 1 = 1`)
	if !filter.StartDate.IsZero() {
		builder.WriteString("\n  AND calendar_date >= ?")
		args = append(args, filter.StartDate)
	}
	if !filter.EndDate.IsZero() {
		builder.WriteString("\n  AND calendar_date <= ?")
		args = append(args, filter.EndDate)
	}
	if filter.DayType != "" {
		builder.WriteString("\n  AND day_type = ?")
		args = append(args, filter.DayType.String())
	}
	builder.WriteString("\nORDER BY calendar_date ASC")
	if filter.Limit > 0 {
		builder.WriteString("\nLIMIT ? OFFSET ?")
		args = append(args, filter.Limit, filter.Offset)
	}

	return builder.String(), args
}

func scanAttendanceCalendarDay(scanner attendanceRecordScanner) (domain.AttendanceCalendarDay, error) {
	var day domain.AttendanceCalendarDay
	var dayType string
	if err := scanner.Scan(&day.Date, &dayType, &day.Name); err != nil {
		return domain.AttendanceCalendarDay{}, fmt.Errorf("repository: scan attendance calendar day: %w", err)
	}
	day.DayType = domain.CalendarDayType(dayType)

	return day, nil
}

func (r *SQLRepository) SaveAttendanceCalendarDay(ctx context.Context, day domain.AttendanceCalendarDay) error {
	const query = `
INSERT INTO attendance_calendar_days (
	calendar_date,
	day_type,
	name
) VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE
	day_type = VALUES(day_type),
	name = VALUES(name)`

	if _, err := r.executor.ExecContext(ctx, query, day.Date, day.DayType.String(), day.Name); err != nil {
		return fmt.Errorf("repository: save attendance calendar day: %w", err)
	}

	return nil
}

func (r *SQLRepository) DeleteAttendanceCalendarDay(ctx context.Context, date time.Time) error {
	const query = `DELETE FROM attendance_calendar_days WHERE calendar_date = ?`
	if _, err := r.executor.ExecContext(ctx, query, date); err != nil {
		return fmt.Errorf("repository: delete attendance calendar day: %w", err)
	}

	return nil
}

func (r *SQLRepository) ListAttendanceSchedules(ctx context.Context, query domain.AttendanceScheduleQuery) ([]domain.ManagedAttendanceSchedule, error) {
	sqlQuery, args := buildListAttendanceSchedulesQuery(query)
	rows, err := r.executor.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: list attendance schedules: %w", err)
	}
	defer rows.Close()

	schedules := make([]domain.ManagedAttendanceSchedule, 0)
	for rows.Next() {
		schedule, err := scanAttendanceSchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: iterate attendance schedules: %w", err)
	}

	return schedules, nil
}

func buildListAttendanceSchedulesQuery(filter domain.AttendanceScheduleQuery) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0, 6)

	builder.WriteString(`
SELECT
	id,
	user_id,
	device_sn,
	schedule_date,
	shift_id,
	rest,
	reason,
	enabled
FROM attendance_schedules
WHERE 1 = 1`)
	if !filter.IncludeDisabled {
		builder.WriteString("\n  AND enabled = TRUE")
	}
	if filter.UserID != "" {
		builder.WriteString("\n  AND user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.DeviceSN != "" {
		builder.WriteString("\n  AND device_sn = ?")
		args = append(args, filter.DeviceSN)
	}
	if !filter.StartDate.IsZero() {
		builder.WriteString("\n  AND schedule_date >= ?")
		args = append(args, filter.StartDate)
	}
	if !filter.EndDate.IsZero() {
		builder.WriteString("\n  AND schedule_date <= ?")
		args = append(args, filter.EndDate)
	}
	builder.WriteString("\nORDER BY schedule_date ASC, user_id ASC, device_sn ASC, id ASC")
	if filter.Limit > 0 {
		builder.WriteString("\nLIMIT ? OFFSET ?")
		args = append(args, filter.Limit, filter.Offset)
	}

	return builder.String(), args
}

func scanAttendanceSchedule(scanner attendanceRecordScanner) (domain.ManagedAttendanceSchedule, error) {
	var schedule domain.ManagedAttendanceSchedule
	if err := scanner.Scan(
		&schedule.ID,
		&schedule.UserID,
		&schedule.DeviceSN,
		&schedule.Date,
		&schedule.ShiftID,
		&schedule.Rest,
		&schedule.Reason,
		&schedule.Enabled,
	); err != nil {
		return domain.ManagedAttendanceSchedule{}, fmt.Errorf("repository: scan attendance schedule: %w", err)
	}

	return schedule, nil
}

func (r *SQLRepository) SaveAttendanceSchedule(ctx context.Context, schedule domain.ManagedAttendanceSchedule) (domain.ManagedAttendanceSchedule, error) {
	if schedule.ID > 0 {
		const updateQuery = `
UPDATE attendance_schedules
SET
	user_id = ?,
	device_sn = ?,
	schedule_date = ?,
	shift_id = ?,
	rest = ?,
	reason = ?,
	enabled = ?
WHERE id = ?`
		if _, err := r.executor.ExecContext(
			ctx,
			updateQuery,
			schedule.UserID,
			schedule.DeviceSN,
			schedule.Date,
			schedule.ShiftID,
			schedule.Rest,
			schedule.Reason,
			schedule.Enabled,
			schedule.ID,
		); err != nil {
			return domain.ManagedAttendanceSchedule{}, fmt.Errorf("repository: update attendance schedule: %w", err)
		}

		return schedule, nil
	}

	const insertQuery = `
INSERT INTO attendance_schedules (
	user_id,
	device_sn,
	schedule_date,
	shift_id,
	rest,
	reason,
	enabled
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	shift_id = VALUES(shift_id),
	rest = VALUES(rest),
	reason = VALUES(reason),
	enabled = VALUES(enabled),
	id = LAST_INSERT_ID(id)`
	result, err := r.executor.ExecContext(
		ctx,
		insertQuery,
		schedule.UserID,
		schedule.DeviceSN,
		schedule.Date,
		schedule.ShiftID,
		schedule.Rest,
		schedule.Reason,
		schedule.Enabled,
	)
	if err != nil {
		return domain.ManagedAttendanceSchedule{}, fmt.Errorf("repository: save attendance schedule: %w", err)
	}
	if id, err := result.LastInsertId(); err == nil && id > 0 {
		schedule.ID = id
	}

	return schedule, nil
}

func (r *SQLRepository) DeleteAttendanceSchedule(ctx context.Context, id int64) error {
	const query = `DELETE FROM attendance_schedules WHERE id = ?`
	if _, err := r.executor.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("repository: delete attendance schedule: %w", err)
	}

	return nil
}

func (r *SQLRepository) ListAttendanceWeeklySchedules(ctx context.Context, query domain.AttendanceWeeklyScheduleQuery) ([]domain.ManagedAttendanceWeeklySchedule, error) {
	sqlQuery, args := buildListAttendanceWeeklySchedulesQuery(query)
	rows, err := r.executor.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: list attendance weekly schedules: %w", err)
	}
	defer rows.Close()

	schedules := make([]domain.ManagedAttendanceWeeklySchedule, 0)
	for rows.Next() {
		schedule, err := scanAttendanceWeeklySchedule(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, schedule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: iterate attendance weekly schedules: %w", err)
	}

	return schedules, nil
}

func buildListAttendanceWeeklySchedulesQuery(filter domain.AttendanceWeeklyScheduleQuery) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0, 5)

	builder.WriteString(`
SELECT
	id,
	user_id,
	device_sn,
	weekday,
	shift_id,
	rest,
	reason,
	enabled
FROM attendance_weekly_schedules
WHERE 1 = 1`)
	if !filter.IncludeDisabled {
		builder.WriteString("\n  AND enabled = TRUE")
	}
	if filter.UserID != "" {
		builder.WriteString("\n  AND user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.DeviceSN != "" {
		builder.WriteString("\n  AND device_sn = ?")
		args = append(args, filter.DeviceSN)
	}
	if filter.Weekday != nil {
		builder.WriteString("\n  AND weekday = ?")
		args = append(args, int(*filter.Weekday))
	}
	builder.WriteString("\nORDER BY weekday ASC, user_id ASC, device_sn ASC, id ASC")
	if filter.Limit > 0 {
		builder.WriteString("\nLIMIT ? OFFSET ?")
		args = append(args, filter.Limit, filter.Offset)
	}

	return builder.String(), args
}

func scanAttendanceWeeklySchedule(scanner attendanceRecordScanner) (domain.ManagedAttendanceWeeklySchedule, error) {
	var schedule domain.ManagedAttendanceWeeklySchedule
	var weekday int
	if err := scanner.Scan(
		&schedule.ID,
		&schedule.UserID,
		&schedule.DeviceSN,
		&weekday,
		&schedule.ShiftID,
		&schedule.Rest,
		&schedule.Reason,
		&schedule.Enabled,
	); err != nil {
		return domain.ManagedAttendanceWeeklySchedule{}, fmt.Errorf("repository: scan attendance weekly schedule: %w", err)
	}
	schedule.Weekday = time.Weekday(weekday)

	return schedule, nil
}

func (r *SQLRepository) SaveAttendanceWeeklySchedule(ctx context.Context, schedule domain.ManagedAttendanceWeeklySchedule) (domain.ManagedAttendanceWeeklySchedule, error) {
	if schedule.ID > 0 {
		const updateQuery = `
UPDATE attendance_weekly_schedules
SET
	user_id = ?,
	device_sn = ?,
	weekday = ?,
	shift_id = ?,
	rest = ?,
	reason = ?,
	enabled = ?
WHERE id = ?`
		if _, err := r.executor.ExecContext(
			ctx,
			updateQuery,
			schedule.UserID,
			schedule.DeviceSN,
			int(schedule.Weekday),
			schedule.ShiftID,
			schedule.Rest,
			schedule.Reason,
			schedule.Enabled,
			schedule.ID,
		); err != nil {
			return domain.ManagedAttendanceWeeklySchedule{}, fmt.Errorf("repository: update attendance weekly schedule: %w", err)
		}

		return schedule, nil
	}

	const insertQuery = `
INSERT INTO attendance_weekly_schedules (
	user_id,
	device_sn,
	weekday,
	shift_id,
	rest,
	reason,
	enabled
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	shift_id = VALUES(shift_id),
	rest = VALUES(rest),
	reason = VALUES(reason),
	enabled = VALUES(enabled),
	id = LAST_INSERT_ID(id)`
	result, err := r.executor.ExecContext(
		ctx,
		insertQuery,
		schedule.UserID,
		schedule.DeviceSN,
		int(schedule.Weekday),
		schedule.ShiftID,
		schedule.Rest,
		schedule.Reason,
		schedule.Enabled,
	)
	if err != nil {
		return domain.ManagedAttendanceWeeklySchedule{}, fmt.Errorf("repository: save attendance weekly schedule: %w", err)
	}
	if id, err := result.LastInsertId(); err == nil && id > 0 {
		schedule.ID = id
	}

	return schedule, nil
}

func (r *SQLRepository) DeleteAttendanceWeeklySchedule(ctx context.Context, id int64) error {
	const query = `DELETE FROM attendance_weekly_schedules WHERE id = ?`
	if _, err := r.executor.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("repository: delete attendance weekly schedule: %w", err)
	}

	return nil
}

func (r *SQLRepository) SaveAttendanceCorrection(ctx context.Context, correction domain.AttendanceCorrection) (domain.AttendanceCorrection, error) {
	const query = `
INSERT INTO attendance_corrections (
	user_id,
	device_sn,
	attendance_date,
	correction_type,
	corrected_at,
	reason,
	status
) VALUES (?, ?, ?, ?, ?, ?, ?)`

	result, err := r.executor.ExecContext(
		ctx,
		query,
		correction.UserID,
		correction.DeviceSN,
		correction.Date,
		correction.Type.String(),
		correction.CorrectedAt,
		correction.Reason,
		correction.Status.String(),
	)
	if err != nil {
		return domain.AttendanceCorrection{}, fmt.Errorf("repository: save attendance correction: %w", err)
	}
	if id, err := result.LastInsertId(); err == nil && id > 0 {
		correction.ID = id
	}

	return correction, nil
}

func (r *SQLRepository) SaveMonthlyAttendanceResult(ctx context.Context, result domain.MonthlyAttendanceDailyResult) (domain.MonthlyAttendanceDailyResult, error) {
	const query = `
INSERT INTO monthly_attendance_results (
	attendance_month,
	attendance_date,
	user_id,
	user_name,
	device_sn,
	shift_id,
	shift_name,
	is_workday,
	non_workday_reason,
	status,
	exceptions,
	is_abnormal,
	corrected,
	correction_status,
	correction_reason,
	corrected_at,
	work_start_at,
	work_end_at,
	first_entry_at,
	last_exit_at,
	late_seconds,
	early_leave_seconds,
	record_count,
	snapshot_count,
	calculated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
	attendance_month = VALUES(attendance_month),
	user_name = VALUES(user_name),
	shift_id = VALUES(shift_id),
	shift_name = VALUES(shift_name),
	is_workday = VALUES(is_workday),
	non_workday_reason = VALUES(non_workday_reason),
	status = VALUES(status),
	exceptions = VALUES(exceptions),
	is_abnormal = VALUES(is_abnormal),
	corrected = VALUES(corrected),
	correction_status = VALUES(correction_status),
	correction_reason = VALUES(correction_reason),
	corrected_at = VALUES(corrected_at),
	work_start_at = VALUES(work_start_at),
	work_end_at = VALUES(work_end_at),
	first_entry_at = VALUES(first_entry_at),
	last_exit_at = VALUES(last_exit_at),
	late_seconds = VALUES(late_seconds),
	early_leave_seconds = VALUES(early_leave_seconds),
	record_count = VALUES(record_count),
	snapshot_count = VALUES(snapshot_count),
	calculated_at = VALUES(calculated_at),
	id = LAST_INSERT_ID(id)`

	dbResult, err := r.executor.ExecContext(
		ctx,
		query,
		result.Month,
		result.Date,
		result.UserID,
		result.UserName,
		result.DeviceSN,
		result.ShiftID,
		result.ShiftName,
		result.IsWorkday,
		result.NonWorkdayReason,
		result.Status.String(),
		formatAttendanceExceptions(result.Exceptions),
		result.IsAbnormal,
		result.Corrected,
		result.CorrectionStatus.String(),
		result.CorrectionReason,
		nullableTime(result.CorrectedAt),
		nullableTime(result.WorkStartAt),
		nullableTime(result.WorkEndAt),
		nullableTime(result.FirstEntryAt),
		nullableTime(result.LastExitAt),
		int64(result.LateDuration.Seconds()),
		int64(result.EarlyLeaveDuration.Seconds()),
		result.RecordCount,
		result.SnapshotCount,
		result.CalculatedAt,
	)
	if err != nil {
		return domain.MonthlyAttendanceDailyResult{}, fmt.Errorf("repository: save monthly attendance result: %w", err)
	}
	if id, err := dbResult.LastInsertId(); err == nil && id > 0 {
		result.ID = id
	}

	return result, nil
}

func (r *SQLRepository) ListMonthlyAttendanceResults(ctx context.Context, query domain.MonthlyAttendanceResultQuery) ([]domain.MonthlyAttendanceDailyResult, error) {
	sqlQuery, args := buildListMonthlyAttendanceResultsQuery(query)
	rows, err := r.executor.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("repository: list monthly attendance results: %w", err)
	}
	defer rows.Close()

	results := make([]domain.MonthlyAttendanceDailyResult, 0)
	for rows.Next() {
		result, err := scanMonthlyAttendanceResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: iterate monthly attendance results: %w", err)
	}

	return results, nil
}

func buildListMonthlyAttendanceResultsQuery(filter domain.MonthlyAttendanceResultQuery) (string, []any) {
	var builder strings.Builder
	args := make([]any, 0, 8)

	builder.WriteString(`
SELECT
	id,
	attendance_month,
	attendance_date,
	user_id,
	user_name,
	device_sn,
	shift_id,
	shift_name,
	is_workday,
	non_workday_reason,
	status,
	exceptions,
	is_abnormal,
	corrected,
	correction_status,
	correction_reason,
	corrected_at,
	work_start_at,
	work_end_at,
	first_entry_at,
	last_exit_at,
	late_seconds,
	early_leave_seconds,
	record_count,
	snapshot_count,
	calculated_at
FROM monthly_attendance_results
WHERE 1 = 1`)
	if filter.UserID != "" {
		builder.WriteString("\n  AND user_id = ?")
		args = append(args, filter.UserID)
	}
	if filter.DeviceSN != "" {
		builder.WriteString("\n  AND device_sn = ?")
		args = append(args, filter.DeviceSN)
	}
	if !filter.StartDate.IsZero() {
		builder.WriteString("\n  AND attendance_date >= ?")
		args = append(args, filter.StartDate)
	}
	if !filter.EndDate.IsZero() {
		builder.WriteString("\n  AND attendance_date <= ?")
		args = append(args, filter.EndDate)
	}
	if !filter.Month.IsZero() {
		builder.WriteString("\n  AND attendance_month = ?")
		args = append(args, filter.Month)
	}
	builder.WriteString("\nORDER BY attendance_date DESC, user_id ASC, device_sn ASC")
	if filter.Limit > 0 {
		builder.WriteString("\nLIMIT ? OFFSET ?")
		args = append(args, filter.Limit, filter.Offset)
	}

	return builder.String(), args
}

func scanMonthlyAttendanceResult(scanner attendanceRecordScanner) (domain.MonthlyAttendanceDailyResult, error) {
	var result domain.MonthlyAttendanceDailyResult
	var status string
	var exceptions string
	var correctionStatus string
	var correctedAt sql.NullTime
	var workStartAt sql.NullTime
	var workEndAt sql.NullTime
	var firstEntryAt sql.NullTime
	var lastExitAt sql.NullTime
	var lateSeconds int64
	var earlyLeaveSeconds int64
	if err := scanner.Scan(
		&result.ID,
		&result.Month,
		&result.Date,
		&result.UserID,
		&result.UserName,
		&result.DeviceSN,
		&result.ShiftID,
		&result.ShiftName,
		&result.IsWorkday,
		&result.NonWorkdayReason,
		&status,
		&exceptions,
		&result.IsAbnormal,
		&result.Corrected,
		&correctionStatus,
		&result.CorrectionReason,
		&correctedAt,
		&workStartAt,
		&workEndAt,
		&firstEntryAt,
		&lastExitAt,
		&lateSeconds,
		&earlyLeaveSeconds,
		&result.RecordCount,
		&result.SnapshotCount,
		&result.CalculatedAt,
	); err != nil {
		return domain.MonthlyAttendanceDailyResult{}, fmt.Errorf("repository: scan monthly attendance result: %w", err)
	}

	result.Status = domain.DailyAttendanceStatus(status)
	result.Exceptions = parseAttendanceExceptions(exceptions)
	result.CorrectionStatus = domain.AttendanceCorrectionStatus(correctionStatus)
	result.CorrectedAt = nullTimeValue(correctedAt)
	result.WorkStartAt = nullTimeValue(workStartAt)
	result.WorkEndAt = nullTimeValue(workEndAt)
	result.FirstEntryAt = nullTimeValue(firstEntryAt)
	result.LastExitAt = nullTimeValue(lastExitAt)
	result.LateDuration = time.Duration(lateSeconds) * time.Second
	result.EarlyLeaveDuration = time.Duration(earlyLeaveSeconds) * time.Second

	return result, nil
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

func parseWeekdayList(value string) []time.Weekday {
	items := strings.Split(value, ",")
	weekdays := make([]time.Weekday, 0, len(items))
	for _, item := range items {
		weekday, ok := parseWeekday(strings.TrimSpace(item))
		if ok {
			weekdays = append(weekdays, weekday)
		}
	}

	return weekdays
}

func formatWeekdayList(weekdays []time.Weekday) string {
	items := make([]string, 0, len(weekdays))
	for _, weekday := range weekdays {
		items = append(items, formatWeekday(weekday))
	}

	return strings.Join(items, ",")
}

func parseWeekday(value string) (time.Weekday, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "0", "sunday", "sun":
		return time.Sunday, true
	case "1", "monday", "mon":
		return time.Monday, true
	case "2", "tuesday", "tue":
		return time.Tuesday, true
	case "3", "wednesday", "wed":
		return time.Wednesday, true
	case "4", "thursday", "thu":
		return time.Thursday, true
	case "5", "friday", "fri":
		return time.Friday, true
	case "6", "saturday", "sat":
		return time.Saturday, true
	default:
		return 0, false
	}
}

func formatWeekday(weekday time.Weekday) string {
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

func parseClockTimeOrZero(value string) domain.ClockTime {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return domain.ClockTime{}
	}

	hour, ok := parseTwoDigitNumber(parts[0], 0, 23)
	if !ok {
		return domain.ClockTime{}
	}
	minute, ok := parseTwoDigitNumber(parts[1], 0, 59)
	if !ok {
		return domain.ClockTime{}
	}

	return domain.ClockTime{Hour: hour, Minute: minute}
}

func parseTwoDigitNumber(value string, min int, max int) (int, bool) {
	if value == "" {
		return 0, false
	}

	result := 0
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		result = result*10 + int(r-'0')
	}
	if result < min || result > max {
		return 0, false
	}

	return result, true
}

func formatClockTime(value domain.ClockTime) string {
	return fmt.Sprintf("%02d:%02d", value.Hour, value.Minute)
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}

	return value
}

func nullTimeValue(value sql.NullTime) time.Time {
	if !value.Valid {
		return time.Time{}
	}

	return value.Time
}

func formatAttendanceExceptions(exceptions []domain.DailyAttendanceException) string {
	values := make([]string, 0, len(exceptions))
	for _, exception := range exceptions {
		exceptionValue := strings.TrimSpace(exception.String())
		if exceptionValue != "" {
			values = append(values, exceptionValue)
		}
	}

	return strings.Join(values, ",")
}

func parseAttendanceExceptions(value string) []domain.DailyAttendanceException {
	items := strings.Split(value, ",")
	exceptions := make([]domain.DailyAttendanceException, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			exceptions = append(exceptions, domain.DailyAttendanceException(item))
		}
	}

	return exceptions
}
