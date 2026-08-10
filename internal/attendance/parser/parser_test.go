package parser_test

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"io"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/parser"
)

func TestParseJSONDoorStatus(t *testing.T) {
	payload := `{
		"Action": "Pulse",
		"Code": "DoorStatus",
		"Data": {
			"RealUTC": 1700000120,
			"SN": "REDACTED_DEVICE_SN",
			"Status": "Open",
			"UTC": 1700000120
		},
		"Index": 0
	}`

	parsed, err := parser.Parse("application/json", "", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("unexpected events length: %d", len(parsed.Events))
	}
	if parsed.Events[0].Code != domain.EventCodeDoorStatus {
		t.Fatalf("unexpected code: %s", parsed.Events[0].Code)
	}
	if len(parsed.Images) != 0 {
		t.Fatalf("unexpected images length: %d", len(parsed.Images))
	}
}

func TestParseJSONBatchAccessControl(t *testing.T) {
	payload := batchAccessControlPayload()

	parsed, err := parser.Parse("application/json", "", strings.NewReader(payload))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("unexpected events length: %d", len(parsed.Events))
	}

	event := parsed.Events[0]
	if event.Code != domain.EventCodeAccessControl {
		t.Fatalf("unexpected code: %s", event.Code)
	}
	if event.DataSource != domain.DataSourceOffline {
		t.Fatalf("unexpected data source: %s", event.DataSource)
	}
}

func TestParseMultipartMixedReplace(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	jsonPart, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"text/plain"},
	})
	if err != nil {
		t.Fatalf("create json part: %v", err)
	}
	if _, err := io.WriteString(jsonPart, batchAccessControlPayload()); err != nil {
		t.Fatalf("write json part: %v", err)
	}

	imagePart, err := writer.CreatePart(map[string][]string{
		"Content-Type": {"image/jpeg"},
	})
	if err != nil {
		t.Fatalf("create image part: %v", err)
	}
	if _, err := imagePart.Write([]byte{0xff, 0xd8, 0xff, 0xdb}); err != nil {
		t.Fatalf("write image part: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	contentType := "multipart/x-mixed-replace; boundary=" + writer.Boundary()
	parsed, err := parser.Parse(contentType, "", &body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(parsed.Events) != 1 {
		t.Fatalf("unexpected events length: %d", len(parsed.Events))
	}
	if len(parsed.Images) != 1 {
		t.Fatalf("unexpected images length: %d", len(parsed.Images))
	}
	if parsed.Images[0].ContentType != "image/jpeg" {
		t.Fatalf("unexpected image content type: %s", parsed.Images[0].ContentType)
	}
}

func TestParseDeflateZlibJSON(t *testing.T) {
	var body bytes.Buffer
	writer := zlib.NewWriter(&body)
	if _, err := io.WriteString(writer, batchAccessControlPayload()); err != nil {
		t.Fatalf("write zlib payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zlib writer: %v", err)
	}

	parsed, err := parser.Parse("application/json", "deflate", &body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Events) != 1 {
		t.Fatalf("unexpected events length: %d", len(parsed.Events))
	}
}

func TestParseDeflateRawJSON(t *testing.T) {
	var body bytes.Buffer
	writer, err := flate.NewWriter(&body, flate.DefaultCompression)
	if err != nil {
		t.Fatalf("create raw deflate writer: %v", err)
	}
	if _, err := io.WriteString(writer, batchAccessControlPayload()); err != nil {
		t.Fatalf("write raw deflate payload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close raw deflate writer: %v", err)
	}

	parsed, err := parser.Parse("application/json", "deflate", &body)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Events) != 1 {
		t.Fatalf("unexpected events length: %d", len(parsed.Events))
	}
}

func TestParseDeflateHeaderWithPlainJSON(t *testing.T) {
	parsed, err := parser.Parse("application/json", "deflate", strings.NewReader(batchAccessControlPayload()))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.Events) != 1 {
		t.Fatalf("unexpected events length: %d", len(parsed.Events))
	}
}

func TestParseUnsupportedContentType(t *testing.T) {
	_, err := parser.Parse("application/octet-stream", "", strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func batchAccessControlPayload() string {
	return `{
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
	}`
}
