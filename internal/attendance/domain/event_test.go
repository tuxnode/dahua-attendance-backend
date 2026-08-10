package domain

import (
	"encoding/json"
	"testing"
)

func TestEventEnvelopeUnmarshalDoorStatus(t *testing.T) {
	payload := []byte(`{
		"Action": "Pulse",
		"Code": "DoorStatus",
		"Data": {
			"RealUTC": 1700000120,
			"SN": "REDACTED_DEVICE_SN",
			"Status": "Open",
			"UTC": 1700000120
		},
		"Index": 0
	}`)

	var envelope EventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if envelope.Action != EventActionPulse {
		t.Fatalf("unexpected action: %s", envelope.Action)
	}
	if envelope.Code != EventCodeDoorStatus {
		t.Fatalf("unexpected code: %s", envelope.Code)
	}

	var event DoorStatusEvent
	if err := json.Unmarshal(envelope.Data, &event); err != nil {
		t.Fatalf("unmarshal door status: %v", err)
	}

	if event.Status != DoorStateOpen {
		t.Fatalf("unexpected door status: %s", event.Status)
	}
}

func TestEventEnvelopeUnmarshalBatchAccessControl(t *testing.T) {
	payload := []byte(`{
		"Events": [
			{
				"Code": "AccessControl",
				"Data": {
					"BlockId": 10001,
					"CardName": "REDACTED_NAME",
					"CardNo": "",
					"CardType": 0,
					"CreateTime": 1700000000,
					"CurrentTemperature": 0.0,
					"Door": 0,
					"ErrorCode": 0,
					"ExternUser": [],
					"FaceIndex": 0,
					"HatColor": "Other",
					"HatType": 0,
					"ImageInfo": [
						{
							"Height": 640,
							"Length": 15344,
							"Offset": 0,
							"Width": 384
						}
					],
					"IsOverTemperature": 0,
					"Method": 15,
					"OperationMode": 0,
					"OperatorID": "",
					"ReaderID": "1",
					"RealUTC": 1700000000,
					"SN": "REDACTED_DEVICE_SN",
					"Status": 1,
					"TemperatureUnit": 0,
					"Type": "Entry",
					"UTC": 1700000000,
					"UserID": "REDACTED_USER_ID",
					"UserType": 0
				},
				"DataSource": "Offline"
			}
		]
	}`)

	var envelope EventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}

	if len(envelope.Events) != 1 {
		t.Fatalf("unexpected events length: %d", len(envelope.Events))
	}

	rawEvent := envelope.Events[0]
	if rawEvent.Code != EventCodeAccessControl {
		t.Fatalf("unexpected code: %s", rawEvent.Code)
	}
	if rawEvent.DataSource != DataSourceOffline {
		t.Fatalf("unexpected data source: %s", rawEvent.DataSource)
	}

	var event AccessControlEvent
	if err := json.Unmarshal(rawEvent.Data, &event); err != nil {
		t.Fatalf("unmarshal access control: %v", err)
	}

	if event.Method != AccessMethodFaceOpen {
		t.Fatalf("unexpected method: %s", event.Method)
	}
	if event.Type != AccessDirectionEntry {
		t.Fatalf("unexpected direction: %s", event.Type)
	}
	if len(event.ImageInfo) != 1 {
		t.Fatalf("unexpected image info length: %d", len(event.ImageInfo))
	}
}
