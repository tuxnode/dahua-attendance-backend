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

type DailyAttendance struct {
	Date               time.Time
	UserID             string
	UserName           string
	DeviceSN           string
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

func (a DailyAttendance) IsAbnormal() bool {
	return a.Status != "" && a.Status != DailyAttendanceStatusNormal
}

func (a DailyAttendance) HasException(exception DailyAttendanceException) bool {
	for _, item := range a.Exceptions {
		if item == exception {
			return true
		}
	}

	return false
}
