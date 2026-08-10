package transporthttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/parser"
	transporthttp "github.com/tuxnode/dahua-attendance-backend/internal/transport/http"
)

type fakeConsumer struct {
	payload *parser.ParsedPayload
	err     error
	calls   int
}

func (c *fakeConsumer) HandleDevicePayload(_ context.Context, payload *parser.ParsedPayload) error {
	c.calls++
	c.payload = payload
	return c.err
}

func TestHandleDeviceEventsAcceptsRootJSON(t *testing.T) {
	consumer := &fakeConsumer{}
	handler := transporthttp.NewHandler(consumer, transporthttp.WithLogger(discardLogger()))

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.HandleDeviceEvents(response, request)

	assertSuccessResponse(t, response)
	if consumer.payload == nil {
		t.Fatal("consumer was not called")
	}
	if got := len(consumer.payload.Events); got != 1 {
		t.Fatalf("unexpected events length: %d", got)
	}
	if consumer.payload.Events[0].Code != domain.EventCodeDoorStatus {
		t.Fatalf("unexpected event code: %s", consumer.payload.Events[0].Code)
	}
}

func TestHandleDeviceEventsAcceptsDeviceEventsPath(t *testing.T) {
	handler := transporthttp.NewHandler(nil, transporthttp.WithLogger(discardLogger()))

	request := httptest.NewRequest(http.MethodPost, transporthttp.DeviceEventsPath, strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.HandleDeviceEvents(response, request)

	assertSuccessResponse(t, response)
}

func TestHandleDeviceEventsAcceptsMultipartMixedReplace(t *testing.T) {
	handler := transporthttp.NewHandler(nil, transporthttp.WithLogger(discardLogger()))

	contentType, body := multipartPayload(t)
	request := httptest.NewRequest(http.MethodPost, "/", body)
	request.Header.Set("Content-Type", contentType)
	response := httptest.NewRecorder()

	handler.HandleDeviceEvents(response, request)

	assertSuccessResponse(t, response)
}

func TestHandleDeviceEventsRejectsUnsupportedMethod(t *testing.T) {
	handler := transporthttp.NewHandler(nil, transporthttp.WithLogger(discardLogger()))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	handler.HandleDeviceEvents(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("unexpected status: %d", response.Code)
	}
	if response.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("unexpected allow header: %s", response.Header().Get("Allow"))
	}
}

func TestHandleDeviceEventsRejectsUnknownPath(t *testing.T) {
	handler := transporthttp.NewHandler(nil, transporthttp.WithLogger(discardLogger()))

	request := httptest.NewRequest(http.MethodPost, "/unknown", strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.HandleDeviceEvents(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func TestHandleDeviceEventsAcksInvalidPayloadWithoutConsumer(t *testing.T) {
	consumer := &fakeConsumer{}
	handler := transporthttp.NewHandler(consumer, transporthttp.WithLogger(discardLogger()))

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"Code": "Unknown", "Data": {}}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.HandleDeviceEvents(response, request)

	assertSuccessResponse(t, response)
	if consumer.calls != 0 {
		t.Fatalf("consumer should not be called, calls: %d", consumer.calls)
	}
}

func TestHandleDeviceEventsAcksOversizedPayloadWithoutConsumer(t *testing.T) {
	consumer := &fakeConsumer{}
	handler := transporthttp.NewHandler(consumer, transporthttp.WithLogger(discardLogger()), transporthttp.WithMaxBodyBytes(8))

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.HandleDeviceEvents(response, request)

	assertSuccessResponse(t, response)
	if consumer.calls != 0 {
		t.Fatalf("consumer should not be called, calls: %d", consumer.calls)
	}
}

func TestHandleDeviceEventsReturnsServerErrorWhenConsumerFails(t *testing.T) {
	handler := transporthttp.NewHandler(
		&fakeConsumer{err: errors.New("store failed")},
		transporthttp.WithLogger(discardLogger()),
	)

	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(doorStatusPayload()))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	handler.HandleDeviceEvents(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", response.Code)
	}
}

func assertSuccessResponse(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d, body: %s", response.Code, response.Body.String())
	}

	var body domain.Response
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if body.Code != 0 || body.Message != "success" {
		t.Fatalf("unexpected response body: %+v", body)
	}
}

func multipartPayload(t *testing.T) (string, io.Reader) {
	t.Helper()

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

	return "multipart/x-mixed-replace; boundary=" + writer.Boundary(), &body
}

func doorStatusPayload() string {
	return `{
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
					"Door": 0,
					"ErrorCode": 0,
					"ImageInfo": [
						{
							"Height": 640,
							"Length": 15344,
							"Offset": 0,
							"Width": 384
						}
					],
					"Method": 15,
					"ReaderID": "1",
					"RealUTC": 1700000000,
					"SN": "REDACTED_DEVICE_SN",
					"Status": 1,
					"Type": "Entry",
					"UTC": 1700000000,
					"UserID": "REDACTED_USER_ID"
				},
				"DataSource": "Offline"
			}
		]
	}`
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
