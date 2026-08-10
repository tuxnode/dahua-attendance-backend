package domain

import "encoding/json"

type EventCode string

const (
	EventCodeAccessControl EventCode = "AccessControl"
	EventCodeDoorStatus    EventCode = "DoorStatus"
)

type EventAction string

const (
	EventActionPulse EventAction = "Pulse"
)

type DataSource string

const (
	DataSourceOffline  DataSource = "Offline"
	DataSourceRealtime DataSource = "Realtime"
)

type AccessDirection string

const (
	AccessDirectionEntry AccessDirection = "Entry"
	AccessDirectionExit  AccessDirection = "Exit"
)

type DoorState string

const (
	DoorStateOpen  DoorState = "Open"
	DoorStateClose DoorState = "Close"
)

type EventEnvelope struct {
	Action     EventAction     `json:"Action,omitempty"`
	Code       EventCode       `json:"Code,omitempty"`
	Index      int32           `json:"Index,omitempty"`
	DataSource DataSource      `json:"DataSource,omitempty"`
	Data       json.RawMessage `json:"Data,omitempty"`
	Events     []EventEnvelope `json:"Events,omitempty"`
}

type ImageInfo struct {
	Height int32 `json:"Height,omitempty"`
	Width  int32 `json:"Width,omitempty"`
	Length int32 `json:"Length,omitempty"`
	Offset int32 `json:"Offset,omitempty"`
}

type AccessControlEvent struct {
	UserID             string            `json:"UserID,omitempty"`
	CardName           string            `json:"CardName,omitempty"`
	CardNo             string            `json:"CardNo,omitempty"`
	CardType           int32             `json:"CardType,omitempty"`
	Method             AccessMethod      `json:"Method,omitempty"`
	Type               AccessDirection   `json:"Type,omitempty"`
	Status             int32             `json:"Status,omitempty"`
	SN                 string            `json:"SN,omitempty"`
	CreateTime         int64             `json:"CreateTime,omitempty"`
	RealUTC            int64             `json:"RealUTC,omitempty"`
	UTC                int64             `json:"UTC,omitempty"`
	Name               string            `json:"Name,omitempty"`
	ReaderID           string            `json:"ReaderID,omitempty"`
	Druss              bool              `json:"Druss,omitempty"`
	ImageInfo          []ImageInfo       `json:"ImageInfo,omitempty"`
	ErrorCode          int32             `json:"ErrorCode,omitempty"`
	BlockID            int64             `json:"BlockId,omitempty"`
	Door               int32             `json:"Door,omitempty"`
	OperationMode      int32             `json:"OperationMode,omitempty"`
	OperatorID         string            `json:"OperatorID,omitempty"`
	UserType           int32             `json:"UserType,omitempty"`
	FaceIndex          int32             `json:"FaceIndex,omitempty"`
	CurrentTemperature float64           `json:"CurrentTemperature,omitempty"`
	IsOverTemperature  int32             `json:"IsOverTemperature,omitempty"`
	TemperatureUnit    int32             `json:"TemperatureUnit,omitempty"`
	HatType            int32             `json:"HatType,omitempty"`
	HatColor           string            `json:"HatColor,omitempty"`
	ExternUser         []json.RawMessage `json:"ExternUser,omitempty"`
}

type DoorStatusEvent struct {
	SN      string    `json:"SN,omitempty"`
	Status  DoorState `json:"Status,omitempty"`
	UTC     int64     `json:"UTC,omitempty"`
	RealUTC int64     `json:"RealUTC,omitempty"`
}

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
