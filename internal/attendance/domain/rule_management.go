package domain

import "time"

type AttendanceSettings struct {
	Timezone       string
	DefaultShiftID string
	WeekendDays    []time.Weekday
}

type AttendanceShiftQuery struct {
	IncludeDisabled bool
	Limit           int
	Offset          int
}

type AttendanceCalendarDayQuery struct {
	StartDate time.Time
	EndDate   time.Time
	DayType   CalendarDayType
	Limit     int
	Offset    int
}

type AttendanceScheduleQuery struct {
	UserID    string
	DeviceSN  string
	StartDate time.Time
	EndDate   time.Time
	Limit     int
	Offset    int
}

type AttendanceWeeklyScheduleQuery struct {
	UserID   string
	DeviceSN string
	Weekday  *time.Weekday
	Limit    int
	Offset   int
}

type CalendarDayType string

const (
	CalendarDayTypeHoliday CalendarDayType = "holiday"
	CalendarDayTypeWorkday CalendarDayType = "workday"
	CalendarDayTypeRestDay CalendarDayType = "rest_day"
)

func (t CalendarDayType) String() string {
	return string(t)
}

type AttendanceCalendarDay struct {
	Date    time.Time
	DayType CalendarDayType
	Name    string
}

type ManagedAttendanceSchedule struct {
	ID       int64
	UserID   string
	DeviceSN string
	Date     time.Time
	ShiftID  string
	Rest     bool
	Reason   string
	Enabled  bool
}

type ManagedAttendanceWeeklySchedule struct {
	ID       int64
	UserID   string
	DeviceSN string
	Weekday  time.Weekday
	ShiftID  string
	Rest     bool
	Reason   string
	Enabled  bool
}
