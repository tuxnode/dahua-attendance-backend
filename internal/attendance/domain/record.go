package domain

import "time"

type AttendanceRecord struct {
	DeviceSN   string
	UserID     string
	CardName   string
	CardNo     string
	Method     AccessMethod
	Direction  AccessDirection
	Status     int32
	EventTime  time.Time
	CreateTime int64
	UTC        int64
	RealUTC    int64
	DataSource DataSource
	Index      int32
	Door       int32
	ReaderID   string
	CardType   int32
	UserType   int32
	ErrorCode  int32
	BlockID    int64
	ImageCount int
	RawEvent   []byte
	ReceivedAt time.Time
}

type AttendanceRecordQuery struct {
	UserID    string
	DeviceSN  string
	StartTime time.Time
	EndTime   time.Time
	Limit     int
	Offset    int
}

type DoorStatusRecord struct {
	DeviceSN   string
	Status     DoorState
	EventTime  time.Time
	UTC        int64
	RealUTC    int64
	DataSource DataSource
	Index      int32
	RawEvent   []byte
	ReceivedAt time.Time
}
