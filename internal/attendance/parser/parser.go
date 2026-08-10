package parser

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	"github.com/tuxnode/dahua-attendance-backend/internal/attendance/domain"
)

type ParsedPayload struct {
	Events []domain.EventEnvelope
	Images []ImagePart
}

type ImagePart struct {
	ContentType string
	Data        []byte
}

func Parse(contentType string, contentEncoding string, body io.Reader) (*ParsedPayload, error) {
	if body == nil {
		return nil, errors.New("parser: body is nil")
	}

	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("parser: read body: %w", err)
	}

	decoded, err := decodeBody(raw, contentEncoding)
	if err != nil {
		return nil, err
	}

	mediaType, params, err := parseMediaType(contentType)
	if err != nil {
		return nil, err
	}

	switch mediaType {
	case "", "application/json", "text/json":
		return parseJSONPayload(decoded)
	case "multipart/x-mixed-replace", "multipart/form-data", "multipart/mixed":
		return parseMultipartPayload(decoded, params)
	default:
		if looksLikeJSON(decoded) {
			return parseJSONPayload(decoded)
		}
		return nil, fmt.Errorf("parser: unsupported content type %q", contentType)
	}
}

func decodeBody(raw []byte, contentEncoding string) ([]byte, error) {
	decoded := raw

	encodings := strings.Split(contentEncoding, ",")
	for i := len(encodings) - 1; i >= 0; i-- {
		encoding := strings.ToLower(strings.TrimSpace(encodings[i]))
		switch encoding {
		case "", "identity":
			continue
		case "deflate":
			data, err := decodeDeflate(decoded)
			if err != nil {
				return nil, err
			}
			decoded = data
		case "gzip":
			data, err := decodeGzip(decoded)
			if err != nil {
				return nil, err
			}
			decoded = data
		default:
			return nil, fmt.Errorf("parser: unsupported content encoding %q", encoding)
		}
	}

	return decoded, nil
}

func decodeDeflate(data []byte) ([]byte, error) {
	zlibReader, zlibErr := zlib.NewReader(bytes.NewReader(data))
	if zlibErr == nil {
		defer zlibReader.Close()
		decoded, err := io.ReadAll(zlibReader)
		if err == nil {
			return decoded, nil
		}
		zlibErr = err
	}

	flateReader := flate.NewReader(bytes.NewReader(data))
	defer flateReader.Close()

	decoded, flateErr := io.ReadAll(flateReader)
	if flateErr == nil {
		return decoded, nil
	}

	// Some device captures advertise deflate while the body is already plain.
	if looksLikeJSON(data) || looksLikeMultipart(data) {
		return data, nil
	}

	return nil, fmt.Errorf("parser: decode deflate: zlib: %v; raw deflate: %v", zlibErr, flateErr)
}

func decodeGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parser: decode gzip: %w", err)
	}
	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("parser: read gzip: %w", err)
	}

	return decoded, nil
}

func parseMediaType(contentType string) (string, map[string]string, error) {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return "", nil, nil
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", nil, fmt.Errorf("parser: parse content type %q: %w", contentType, err)
	}

	return strings.ToLower(mediaType), params, nil
}

func parseJSONPayload(data []byte) (*ParsedPayload, error) {
	events, err := parseEventEnvelopes(data)
	if err != nil {
		return nil, err
	}

	return &ParsedPayload{Events: events}, nil
}

func parseMultipartPayload(data []byte, params map[string]string) (*ParsedPayload, error) {
	boundary := params["boundary"]
	if boundary == "" {
		return nil, errors.New("parser: multipart boundary is missing")
	}

	reader := multipart.NewReader(bytes.NewReader(data), boundary)
	payload := &ParsedPayload{}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parser: read multipart part: %w", err)
		}

		partData, err := io.ReadAll(part)
		part.Close()
		if err != nil {
			return nil, fmt.Errorf("parser: read multipart part body: %w", err)
		}

		partType, _, err := parseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}

		switch {
		case isJSONPart(partType, partData):
			events, err := parseEventEnvelopes(partData)
			if err != nil {
				return nil, err
			}
			payload.Events = append(payload.Events, events...)
		case isImagePart(partType, partData):
			payload.Images = append(payload.Images, ImagePart{
				ContentType: imageContentType(partType),
				Data:        partData,
			})
		}
	}

	if len(payload.Events) == 0 {
		return nil, errors.New("parser: no event payload found")
	}

	return payload, nil
}

func parseEventEnvelopes(data []byte) ([]domain.EventEnvelope, error) {
	var root domain.EventEnvelope
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parser: decode json payload: %w", err)
	}

	if len(root.Events) > 0 {
		events := make([]domain.EventEnvelope, 0, len(root.Events))
		for index, event := range root.Events {
			event = inheritEnvelopeFields(root, event)
			if err := validateEventEnvelope(index, event); err != nil {
				return nil, err
			}
			events = append(events, event)
		}
		return events, nil
	}

	if root.Code == "" {
		return nil, errors.New("parser: event code is missing")
	}
	if err := validateEventEnvelope(0, root); err != nil {
		return nil, err
	}

	return []domain.EventEnvelope{root}, nil
}

func inheritEnvelopeFields(root, event domain.EventEnvelope) domain.EventEnvelope {
	if event.Action == "" {
		event.Action = root.Action
	}
	if event.DataSource == "" {
		event.DataSource = root.DataSource
	}
	return event
}

func validateEventEnvelope(index int, event domain.EventEnvelope) error {
	if event.Code == "" {
		return fmt.Errorf("parser: event %d code is missing", index)
	}
	if isEmptyJSON(event.Data) {
		return fmt.Errorf("parser: event %d data is missing", index)
	}

	switch event.Code {
	case domain.EventCodeAccessControl:
		var accessEvent domain.AccessControlEvent
		if err := json.Unmarshal(event.Data, &accessEvent); err != nil {
			return fmt.Errorf("parser: event %d decode access control data: %w", index, err)
		}
	case domain.EventCodeDoorStatus:
		var doorEvent domain.DoorStatusEvent
		if err := json.Unmarshal(event.Data, &doorEvent); err != nil {
			return fmt.Errorf("parser: event %d decode door status data: %w", index, err)
		}
	default:
		return fmt.Errorf("parser: event %d unsupported code %q", index, event.Code)
	}

	return nil
}

func isEmptyJSON(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

func isJSONPart(mediaType string, data []byte) bool {
	switch mediaType {
	case "application/json", "text/json":
		return true
	case "", "text/plain":
		return looksLikeJSON(data)
	default:
		return false
	}
}

func isImagePart(mediaType string, data []byte) bool {
	return strings.HasPrefix(mediaType, "image/") || looksLikeJPEG(data)
}

func imageContentType(mediaType string) string {
	if mediaType != "" {
		return mediaType
	}
	return "application/octet-stream"
}

func looksLikeJSON(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return bytes.HasPrefix(trimmed, []byte("{")) || bytes.HasPrefix(trimmed, []byte("["))
}

func looksLikeMultipart(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	return bytes.HasPrefix(trimmed, []byte("--"))
}

func looksLikeJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8
}
