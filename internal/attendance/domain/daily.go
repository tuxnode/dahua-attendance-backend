package domain

import "time"

type DailyAttendanceStatus string

const (
	DailyAttendanceStatusNormal            DailyAttendanceStatus = "normal"
	DailyAttendanceStatusLate              DailyAttendanceStatus = "late"
	DailyAttendanceStatusEarlyLeave        DailyAttendanceStatus = "early_leave"
	DailyAttendanceStatusLateAndEarlyLeave DailyAttendanceStatus = "late_and_early_leave"
	DailyAttendanceStatusMissingCheckIn    DailyAttendanceStatus = "missing_check_in"
	DailyAttendanceStatusMissingCheckOut   DailyAttendanceStatus = "missing_check_out"
	DailyAttendanceStatusAbsent            DailyAttendanceStatus = "absent"
	DailyAttendanceStatusRestDay           DailyAttendanceStatus = "rest_day"
	DailyAttendanceStatusUnknown           DailyAttendanceStatus = "unknown"
)

func (s DailyAttendanceStatus) String() string {
	return string(s)
}

type DailyAttendanceException string

const (
	DailyAttendanceExceptionLate            DailyAttendanceException = "late"
	DailyAttendanceExceptionEarlyLeave      DailyAttendanceException = "early_leave"
	DailyAttendanceExceptionMissingCheckIn  DailyAttendanceException = "missing_check_in"
	DailyAttendanceExceptionMissingCheckOut DailyAttendanceException = "missing_check_out"
	DailyAttendanceExceptionAbsent          DailyAttendanceException = "absent"
)

func (e DailyAttendanceException) String() string {
	return string(e)
}

type DailyAttendanceQuery struct {
	UserID    string
	DeviceSN  string
	StartDate time.Time
	EndDate   time.Time
	Limit     int
	Offset    int
}

type MonthlyAttendanceQuery struct {
	UserID   string
	DeviceSN string
	Month    time.Time
	Limit    int
	Offset   int
}

type AttendanceSummaryQuery struct {
	UserID    string
	DeviceSN  string
	StartDate time.Time
	EndDate   time.Time
}

type AttendanceExceptionQuery struct {
	UserID    string
	DeviceSN  string
	StartDate time.Time
	EndDate   time.Time
	Limit     int
	Offset    int
}

type DailyAttendance struct {
	Date               time.Time
	UserID             string
	UserName           string
	DeviceSN           string
	ShiftID            string
	ShiftName          string
	IsWorkday          bool
	NonWorkdayReason   string
	Status             DailyAttendanceStatus
	Exceptions         []DailyAttendanceException
	WorkStartAt        time.Time
	WorkEndAt          time.Time
	FirstEntryAt       time.Time
	LastExitAt         time.Time
	LateDuration       time.Duration
	EarlyLeaveDuration time.Duration
	RecordCount        int
	SnapshotCount      int
}

type MonthlyAttendance struct {
	Month    time.Time
	UserID   string
	UserName string
	DeviceSN string
	Stats    AttendanceStats
}

type AttendanceSummary struct {
	StartDate time.Time
	EndDate   time.Time
	UserCount int
	Stats     AttendanceStats
}

type AttendanceStats struct {
	TotalDays               int
	WorkDays                int
	RestDays                int
	NormalDays              int
	AbnormalDays            int
	LateDays                int
	EarlyLeaveDays          int
	LateAndEarlyLeaveDays   int
	MissingCheckInDays      int
	MissingCheckOutDays     int
	AbsentDays              int
	RecordCount             int
	SnapshotCount           int
	TotalLateDuration       time.Duration
	TotalEarlyLeaveDuration time.Duration
}

func (a DailyAttendance) IsAbnormal() bool {
	return a.Status != "" &&
		a.Status != DailyAttendanceStatusNormal &&
		a.Status != DailyAttendanceStatusRestDay
}

func (a DailyAttendance) HasException(exception DailyAttendanceException) bool {
	for _, item := range a.Exceptions {
		if item == exception {
			return true
		}
	}

	return false
}
