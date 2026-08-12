package domain

import "time"

const (
	DefaultAttendanceShiftID   = "day"
	DefaultAttendanceShiftName = "Day Shift"
)

type AttendanceRules struct {
	Location        *time.Location
	DefaultShiftID  string
	WeekendDays     map[time.Weekday]bool
	Holidays        map[string]string
	Workdays        map[string]bool
	CalendarDays    map[string]AttendanceCalendarDay
	Shifts          map[string]AttendanceShift
	Schedules       []AttendanceSchedule
	WeeklySchedules []AttendanceWeeklySchedule
}

type AttendanceShift struct {
	ID              string
	Name            string
	Start           ClockTime
	End             ClockTime
	LateGrace       time.Duration
	EarlyLeaveGrace time.Duration
	Flexible        time.Duration
	Enabled         bool
}

type ClockTime struct {
	Hour   int
	Minute int
}

type AttendanceSchedule struct {
	UserID   string
	DeviceSN string
	Date     time.Time
	ShiftID  string
	Rest     bool
	Reason   string
}

type AttendanceWeeklySchedule struct {
	UserID   string
	DeviceSN string
	Weekday  time.Weekday
	ShiftID  string
	Rest     bool
	Reason   string
}

type AttendanceDayRule struct {
	Date             time.Time
	Shift            AttendanceShift
	IsWorkday        bool
	NonWorkdayReason string
}

func DefaultAttendanceRules() AttendanceRules {
	shift := AttendanceShift{
		ID:      DefaultAttendanceShiftID,
		Name:    DefaultAttendanceShiftName,
		Start:   ClockTime{Hour: 9},
		End:     ClockTime{Hour: 18},
		Enabled: true,
	}

	return AttendanceRules{
		DefaultShiftID: DefaultAttendanceShiftID,
		Shifts: map[string]AttendanceShift{
			DefaultAttendanceShiftID: shift,
		},
	}
}

func (t ClockTime) At(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), t.Hour, t.Minute, 0, 0, date.Location())
}

func (s AttendanceShift) WorkStartAt(date time.Time) time.Time {
	return s.Start.At(date)
}

func (s AttendanceShift) WorkEndAt(date time.Time) time.Time {
	end := s.End.At(date)
	if !end.After(s.WorkStartAt(date)) {
		end = end.AddDate(0, 0, 1)
	}

	return end
}
