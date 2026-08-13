package domain

import "time"

type AttendancePersonFilter struct {
	UserID   string
	UserName string
	DeviceSN string
}

type DateRangeFilter struct {
	StartDate time.Time
	EndDate   time.Time
}

type TimeRangeFilter struct {
	StartTime time.Time
	EndTime   time.Time
}

type Pagination struct {
	Limit  int
	Offset int
}
