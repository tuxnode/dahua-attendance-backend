package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
)

func (s *Service) GetAttendanceSettings(ctx context.Context) (domain.AttendanceSettings, error) {
	if s.repository == nil {
		return domain.AttendanceSettings{}, errors.New("service: repository is nil")
	}

	settings, err := s.repository.GetAttendanceSettings(ctx)
	if err != nil {
		return domain.AttendanceSettings{}, fmt.Errorf("service: get attendance settings: %w", err)
	}

	return normalizeAttendanceSettings(settings), nil
}

func (s *Service) SaveAttendanceSettings(ctx context.Context, settings domain.AttendanceSettings) (domain.AttendanceSettings, error) {
	if s.repository == nil {
		return domain.AttendanceSettings{}, errors.New("service: repository is nil")
	}

	normalized := normalizeAttendanceSettings(settings)
	if _, err := time.LoadLocation(normalized.Timezone); err != nil {
		return domain.AttendanceSettings{}, fmt.Errorf("service: invalid attendance timezone %q: %w", normalized.Timezone, err)
	}
	if err := s.repository.SaveAttendanceSettings(ctx, normalized); err != nil {
		return domain.AttendanceSettings{}, fmt.Errorf("service: save attendance settings: %w", err)
	}

	return normalized, nil
}

func (s *Service) ListAttendanceShifts(ctx context.Context, query domain.AttendanceShiftQuery) ([]domain.AttendanceShift, error) {
	if s.repository == nil {
		return nil, errors.New("service: repository is nil")
	}

	normalized := normalizeAttendanceShiftQuery(query)
	shifts, err := s.repository.ListAttendanceShifts(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("service: list attendance shifts: %w", err)
	}

	return shifts, nil
}

func (s *Service) SaveAttendanceShift(ctx context.Context, shift domain.AttendanceShift) (domain.AttendanceShift, error) {
	if s.repository == nil {
		return domain.AttendanceShift{}, errors.New("service: repository is nil")
	}

	normalized, err := normalizeManagedAttendanceShift(shift)
	if err != nil {
		return domain.AttendanceShift{}, err
	}
	if err := s.repository.SaveAttendanceShift(ctx, normalized); err != nil {
		return domain.AttendanceShift{}, fmt.Errorf("service: save attendance shift: %w", err)
	}

	return normalized, nil
}

func (s *Service) DeleteAttendanceShift(ctx context.Context, id string) error {
	if s.repository == nil {
		return errors.New("service: repository is nil")
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("service: attendance shift id cannot be empty")
	}
	if err := s.repository.DeleteAttendanceShift(ctx, id); err != nil {
		return fmt.Errorf("service: delete attendance shift: %w", err)
	}

	return nil
}

func (s *Service) ListAttendanceCalendarDays(ctx context.Context, query domain.AttendanceCalendarDayQuery) ([]domain.AttendanceCalendarDay, error) {
	if s.repository == nil {
		return nil, errors.New("service: repository is nil")
	}

	normalized, err := normalizeAttendanceCalendarDayQuery(query)
	if err != nil {
		return nil, err
	}
	days, err := s.repository.ListAttendanceCalendarDays(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("service: list attendance calendar days: %w", err)
	}

	return days, nil
}

func (s *Service) SaveAttendanceCalendarDay(ctx context.Context, day domain.AttendanceCalendarDay) (domain.AttendanceCalendarDay, error) {
	if s.repository == nil {
		return domain.AttendanceCalendarDay{}, errors.New("service: repository is nil")
	}

	normalized, err := normalizeAttendanceCalendarDay(day)
	if err != nil {
		return domain.AttendanceCalendarDay{}, err
	}
	if err := s.repository.SaveAttendanceCalendarDay(ctx, normalized); err != nil {
		return domain.AttendanceCalendarDay{}, fmt.Errorf("service: save attendance calendar day: %w", err)
	}

	return normalized, nil
}

func (s *Service) DeleteAttendanceCalendarDay(ctx context.Context, date time.Time) error {
	if s.repository == nil {
		return errors.New("service: repository is nil")
	}

	date = startOfDay(date)
	if date.IsZero() {
		return errors.New("service: attendance calendar date cannot be empty")
	}
	if err := s.repository.DeleteAttendanceCalendarDay(ctx, date); err != nil {
		return fmt.Errorf("service: delete attendance calendar day: %w", err)
	}

	return nil
}

func (s *Service) ListAttendanceSchedules(ctx context.Context, query domain.AttendanceScheduleQuery) ([]domain.ManagedAttendanceSchedule, error) {
	if s.repository == nil {
		return nil, errors.New("service: repository is nil")
	}

	normalized, err := normalizeAttendanceScheduleQuery(query)
	if err != nil {
		return nil, err
	}
	schedules, err := s.repository.ListAttendanceSchedules(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("service: list attendance schedules: %w", err)
	}

	return schedules, nil
}

func (s *Service) SaveAttendanceSchedule(ctx context.Context, schedule domain.ManagedAttendanceSchedule) (domain.ManagedAttendanceSchedule, error) {
	if s.repository == nil {
		return domain.ManagedAttendanceSchedule{}, errors.New("service: repository is nil")
	}

	normalized, err := normalizeManagedAttendanceSchedule(schedule)
	if err != nil {
		return domain.ManagedAttendanceSchedule{}, err
	}
	saved, err := s.repository.SaveAttendanceSchedule(ctx, normalized)
	if err != nil {
		return domain.ManagedAttendanceSchedule{}, fmt.Errorf("service: save attendance schedule: %w", err)
	}

	return saved, nil
}

func (s *Service) DeleteAttendanceSchedule(ctx context.Context, id int64) error {
	if s.repository == nil {
		return errors.New("service: repository is nil")
	}
	if id <= 0 {
		return errors.New("service: attendance schedule id must be positive")
	}
	if err := s.repository.DeleteAttendanceSchedule(ctx, id); err != nil {
		return fmt.Errorf("service: delete attendance schedule: %w", err)
	}

	return nil
}

func (s *Service) ListAttendanceWeeklySchedules(ctx context.Context, query domain.AttendanceWeeklyScheduleQuery) ([]domain.ManagedAttendanceWeeklySchedule, error) {
	if s.repository == nil {
		return nil, errors.New("service: repository is nil")
	}

	normalized := normalizeAttendanceWeeklyScheduleQuery(query)
	schedules, err := s.repository.ListAttendanceWeeklySchedules(ctx, normalized)
	if err != nil {
		return nil, fmt.Errorf("service: list attendance weekly schedules: %w", err)
	}

	return schedules, nil
}

func (s *Service) SaveAttendanceWeeklySchedule(ctx context.Context, schedule domain.ManagedAttendanceWeeklySchedule) (domain.ManagedAttendanceWeeklySchedule, error) {
	if s.repository == nil {
		return domain.ManagedAttendanceWeeklySchedule{}, errors.New("service: repository is nil")
	}

	normalized, err := normalizeManagedAttendanceWeeklySchedule(schedule)
	if err != nil {
		return domain.ManagedAttendanceWeeklySchedule{}, err
	}
	saved, err := s.repository.SaveAttendanceWeeklySchedule(ctx, normalized)
	if err != nil {
		return domain.ManagedAttendanceWeeklySchedule{}, fmt.Errorf("service: save attendance weekly schedule: %w", err)
	}

	return saved, nil
}

func (s *Service) DeleteAttendanceWeeklySchedule(ctx context.Context, id int64) error {
	if s.repository == nil {
		return errors.New("service: repository is nil")
	}
	if id <= 0 {
		return errors.New("service: attendance weekly schedule id must be positive")
	}
	if err := s.repository.DeleteAttendanceWeeklySchedule(ctx, id); err != nil {
		return fmt.Errorf("service: delete attendance weekly schedule: %w", err)
	}

	return nil
}

func normalizeAttendanceSettings(settings domain.AttendanceSettings) domain.AttendanceSettings {
	settings.Timezone = strings.TrimSpace(settings.Timezone)
	if settings.Timezone == "" {
		settings.Timezone = "Asia/Shanghai"
	}
	settings.DefaultShiftID = strings.TrimSpace(settings.DefaultShiftID)
	if settings.DefaultShiftID == "" {
		settings.DefaultShiftID = domain.DefaultAttendanceShiftID
	}
	if len(settings.WeekendDays) == 0 {
		settings.WeekendDays = []time.Weekday{time.Saturday, time.Sunday}
	}

	return settings
}

func normalizeAttendanceShiftQuery(query domain.AttendanceShiftQuery) domain.AttendanceShiftQuery {
	if query.Limit < 0 {
		query.Limit = 0
	}
	if query.Limit > maxQueryLimit {
		query.Limit = maxQueryLimit
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	return query
}

func normalizeManagedAttendanceShift(shift domain.AttendanceShift) (domain.AttendanceShift, error) {
	shift.ID = strings.TrimSpace(shift.ID)
	if shift.ID == "" {
		return domain.AttendanceShift{}, errors.New("service: attendance shift id cannot be empty")
	}
	shift.Name = strings.TrimSpace(shift.Name)
	if shift.Name == "" {
		shift.Name = shift.ID
	}
	if err := validateClockTime(shift.Start, "start_time"); err != nil {
		return domain.AttendanceShift{}, err
	}
	if err := validateClockTime(shift.End, "end_time"); err != nil {
		return domain.AttendanceShift{}, err
	}
	if shift.LateGrace < 0 || shift.EarlyLeaveGrace < 0 || shift.Flexible < 0 {
		return domain.AttendanceShift{}, errors.New("service: attendance shift durations cannot be negative")
	}

	return shift, nil
}

func normalizeAttendanceCalendarDayQuery(query domain.AttendanceCalendarDayQuery) (domain.AttendanceCalendarDayQuery, error) {
	query.StartDate = startOfDay(query.StartDate)
	query.EndDate = startOfDay(query.EndDate)
	if !query.StartDate.IsZero() && !query.EndDate.IsZero() && query.EndDate.Before(query.StartDate) {
		return domain.AttendanceCalendarDayQuery{}, errors.New("service: end date must not be before start date")
	}
	if query.DayType != "" && !validCalendarDayType(query.DayType) {
		return domain.AttendanceCalendarDayQuery{}, fmt.Errorf("service: unsupported attendance calendar day type %q", query.DayType)
	}
	if query.Limit < 0 {
		query.Limit = 0
	}
	if query.Limit > maxQueryLimit {
		query.Limit = maxQueryLimit
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	return query, nil
}

func normalizeAttendanceCalendarDay(day domain.AttendanceCalendarDay) (domain.AttendanceCalendarDay, error) {
	day.Date = startOfDay(day.Date)
	if day.Date.IsZero() {
		return domain.AttendanceCalendarDay{}, errors.New("service: attendance calendar date cannot be empty")
	}
	if !validCalendarDayType(day.DayType) {
		return domain.AttendanceCalendarDay{}, fmt.Errorf("service: unsupported attendance calendar day type %q", day.DayType)
	}
	day.Name = strings.TrimSpace(day.Name)

	return day, nil
}

func normalizeAttendanceScheduleQuery(query domain.AttendanceScheduleQuery) (domain.AttendanceScheduleQuery, error) {
	query.UserID = strings.TrimSpace(query.UserID)
	query.DeviceSN = strings.TrimSpace(query.DeviceSN)
	query.StartDate = startOfDay(query.StartDate)
	query.EndDate = startOfDay(query.EndDate)
	if !query.StartDate.IsZero() && !query.EndDate.IsZero() && query.EndDate.Before(query.StartDate) {
		return domain.AttendanceScheduleQuery{}, errors.New("service: end date must not be before start date")
	}
	if query.Limit < 0 {
		query.Limit = 0
	}
	if query.Limit > maxQueryLimit {
		query.Limit = maxQueryLimit
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	return query, nil
}

func normalizeManagedAttendanceSchedule(schedule domain.ManagedAttendanceSchedule) (domain.ManagedAttendanceSchedule, error) {
	schedule.UserID = strings.TrimSpace(schedule.UserID)
	schedule.DeviceSN = strings.TrimSpace(schedule.DeviceSN)
	schedule.Date = startOfDay(schedule.Date)
	schedule.ShiftID = strings.TrimSpace(schedule.ShiftID)
	schedule.Reason = strings.TrimSpace(schedule.Reason)
	if schedule.Date.IsZero() {
		return domain.ManagedAttendanceSchedule{}, errors.New("service: attendance schedule date cannot be empty")
	}
	if !schedule.Rest && schedule.ShiftID == "" {
		return domain.ManagedAttendanceSchedule{}, errors.New("service: attendance schedule shift_id cannot be empty when rest is false")
	}
	if schedule.Rest && schedule.Reason == "" {
		schedule.Reason = "scheduled_rest"
	}

	return schedule, nil
}

func normalizeAttendanceWeeklyScheduleQuery(query domain.AttendanceWeeklyScheduleQuery) domain.AttendanceWeeklyScheduleQuery {
	query.UserID = strings.TrimSpace(query.UserID)
	query.DeviceSN = strings.TrimSpace(query.DeviceSN)
	if query.Limit < 0 {
		query.Limit = 0
	}
	if query.Limit > maxQueryLimit {
		query.Limit = maxQueryLimit
	}
	if query.Offset < 0 {
		query.Offset = 0
	}

	return query
}

func normalizeManagedAttendanceWeeklySchedule(schedule domain.ManagedAttendanceWeeklySchedule) (domain.ManagedAttendanceWeeklySchedule, error) {
	schedule.UserID = strings.TrimSpace(schedule.UserID)
	schedule.DeviceSN = strings.TrimSpace(schedule.DeviceSN)
	schedule.ShiftID = strings.TrimSpace(schedule.ShiftID)
	schedule.Reason = strings.TrimSpace(schedule.Reason)
	if schedule.Weekday < time.Sunday || schedule.Weekday > time.Saturday {
		return domain.ManagedAttendanceWeeklySchedule{}, errors.New("service: attendance weekly schedule weekday must be between 0 and 6")
	}
	if !schedule.Rest && schedule.ShiftID == "" {
		return domain.ManagedAttendanceWeeklySchedule{}, errors.New("service: attendance weekly schedule shift_id cannot be empty when rest is false")
	}
	if schedule.Rest && schedule.Reason == "" {
		schedule.Reason = "scheduled_rest"
	}

	return schedule, nil
}

func validCalendarDayType(dayType domain.CalendarDayType) bool {
	switch dayType {
	case domain.CalendarDayTypeHoliday, domain.CalendarDayTypeWorkday, domain.CalendarDayTypeRestDay:
		return true
	default:
		return false
	}
}

func validateClockTime(value domain.ClockTime, field string) error {
	if value.Hour < 0 || value.Hour > 23 {
		return fmt.Errorf("service: attendance shift %s hour must be between 0 and 23", field)
	}
	if value.Minute < 0 || value.Minute > 59 {
		return fmt.Errorf("service: attendance shift %s minute must be between 0 and 59", field)
	}

	return nil
}
