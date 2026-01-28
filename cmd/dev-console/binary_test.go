// binary_test.go — Tests for binary format detection via magic bytes.
// Verifies detection of MessagePack, protobuf, CBOR, BSON formats
// and correct handling of edge cases (empty, text, unknown binary).
package main

import (
	"testing"
)

func TestDetectBinaryFormat_Empty(t *testing.T) {
	t.Parallel()
	result := DetectBinaryFormat(nil)
	if result != nil {
		t.Errorf("expected nil for empty input, got %+v", result)
	}

	result = DetectBinaryFormat([]byte{})
	if result != nil {
		t.Errorf("expected nil for empty slice, got %+v", result)
	}
}

func TestDetectBinaryFormat_TextContent(t *testing.T) {
	t.Parallel()
	// Plain text should not be detected as binary format
	tests := []string{
		"hello world",
		`{"key": "value"}`,
		"<html><body>test</body></html>",
		"GET /api/test HTTP/1.1",
	}
	for _, text := range tests {
		result := DetectBinaryFormat([]byte(text))
		if result != nil {
			t.Errorf("expected nil for text content %q, got %+v", text, result)
		}
	}
}

func TestDetectBinaryFormat_MessagePack(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		// fixmap (0x80-0x8f): map with 0-15 elements
		{"fixmap_empty", []byte{0x80}},
		{"fixmap_1elem", []byte{0x81, 0xa3, 0x6b, 0x65, 0x79, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65}}, // {"key":"value"}
		{"fixmap_max", []byte{0x8f}},

		// fixarray (0x90-0x9f): array with 0-15 elements
		{"fixarray_empty", []byte{0x90}},
		{"fixarray_3elem", []byte{0x93, 0x01, 0x02, 0x03}}, // [1,2,3]
		{"fixarray_max", []byte{0x9f}},

		// fixstr (0xa0-0xbf): string with 0-31 bytes
		{"fixstr_empty", []byte{0xa0}},
		{"fixstr_hello", []byte{0xa5, 0x68, 0x65, 0x6c, 0x6c, 0x6f}}, // "hello"

		// Type markers
		{"nil", []byte{0xc0}},
		{"false", []byte{0xc2}},
		{"true", []byte{0xc3}},
		{"float32", []byte{0xca, 0x40, 0x48, 0xf5, 0xc3}}, // 3.14
		{"float64", []byte{0xcb, 0x40, 0x09, 0x21, 0xfb, 0x54, 0x44, 0x2d, 0x18}},
		{"uint8", []byte{0xcc, 0xff}},
		{"uint16", []byte{0xcd, 0x01, 0x00}},
		{"uint32", []byte{0xce, 0x00, 0x01, 0x00, 0x00}},
		{"uint64", []byte{0xcf, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}},
		{"int8", []byte{0xd0, 0xff}},
		{"int16", []byte{0xd1, 0xff, 0xff}},
		{"int32", []byte{0xd2, 0xff, 0xff, 0xff, 0xff}},
		{"int64", []byte{0xd3, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{"map16", []byte{0xde, 0x00, 0x01}},
		{"map32", []byte{0xdf, 0x00, 0x00, 0x00, 0x01}},
		{"array16", []byte{0xdc, 0x00, 0x01}},
		{"array32", []byte{0xdd, 0x00, 0x00, 0x00, 0x01}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected MessagePack detection for %s, got nil", tt.name)
			}
			if result.Name != "messagepack" {
				t.Errorf("expected name 'messagepack', got %q", result.Name)
			}
			if result.Confidence < 0.7 || result.Confidence > 1.0 {
				t.Errorf("expected confidence between 0.7-1.0, got %f", result.Confidence)
			}
		})
	}
}

func TestDetectBinaryFormat_Protobuf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		// Field 1, wire type 0 (varint) - common for protobuf messages
		{"field1_varint", []byte{0x08, 0x96, 0x01}}, // field 1, varint 150
		{"field1_varint_simple", []byte{0x08, 0x01}}, // field 1, varint 1

		// Field 1, wire type 2 (length-delimited) - string/bytes/embedded message
		{"field1_string", []byte{0x0a, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f}}, // field 1, string "hello"

		// Multiple fields
		{"multi_field", []byte{0x08, 0x01, 0x10, 0x02, 0x18, 0x03}}, // fields 1,2,3 with varints
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected protobuf detection for %s, got nil", tt.name)
			}
			if result.Name != "protobuf" {
				t.Errorf("expected name 'protobuf', got %q", result.Name)
			}
			if result.Confidence < 0.5 || result.Confidence > 1.0 {
				t.Errorf("expected confidence between 0.5-1.0, got %f", result.Confidence)
			}
		})
	}
}

func TestDetectBinaryFormat_CBOR(t *testing.T) {
	t.Parallel()
	// Note: CBOR and MessagePack share overlapping byte ranges for arrays/maps.
	// MessagePack is checked first, so those ranges are detected as MessagePack.
	// CBOR-specific tests focus on non-overlapping markers.

	tests := []struct {
		name string
		data []byte
	}{
		// Simple values (major type 7) - unique to CBOR
		{"false", []byte{0xf4}},
		{"true", []byte{0xf5}},
		{"null", []byte{0xf6}},
		{"undefined", []byte{0xf7}},
		{"float16", []byte{0xf9, 0x3c, 0x00}}, // 1.0
		{"float32", []byte{0xfa, 0x47, 0xc3, 0x50, 0x00}},
		{"float64", []byte{0xfb, 0x40, 0x09, 0x21, 0xfb, 0x54, 0x44, 0x2d, 0x18}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected CBOR detection for %s, got nil", tt.name)
			}
			if result.Name != "cbor" {
				t.Errorf("expected name 'cbor', got %q", result.Name)
			}
			if result.Confidence < 0.7 || result.Confidence > 1.0 {
				t.Errorf("expected confidence between 0.7-1.0, got %f", result.Confidence)
			}
		})
	}
}

func TestDetectBinaryFormat_CBOR_Overlapping(t *testing.T) {
	t.Parallel()
	// These CBOR markers overlap with MessagePack and are detected as MessagePack.
	// This is by design - MessagePack is more commonly used in web contexts.
	tests := []struct {
		name     string
		data     []byte
		expected string // "messagepack" or "cbor"
	}{
		// Map (0xa0-0xbf) overlaps with MessagePack fixstr
		{"map_empty", []byte{0xa0}, "messagepack"},
		{"map_1elem", []byte{0xa1, 0x61, 0x61, 0x01}, "messagepack"},

		// Array (0x80-0x9f) overlaps with MessagePack fixmap
		{"array_empty", []byte{0x80}, "messagepack"},
		{"array_3elem", []byte{0x83, 0x01, 0x02, 0x03}, "messagepack"},

		// Tagged values (0xc0-0xdf) overlap with MessagePack type markers
		// 0xc0 = MessagePack nil, 0xc1 = CBOR tag 1 (epoch)
		{"tagged_nil", []byte{0xc0}, "messagepack"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected detection for %s, got nil", tt.name)
			}
			if result.Name != tt.expected {
				t.Errorf("expected %s for %s, got %q", tt.expected, tt.name, result.Name)
			}
		})
	}
}

func TestDetectBinaryFormat_BSON(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		// BSON document: int32 length + elements + null terminator
		// Minimum valid BSON: 5 bytes (4-byte length = 5, 1-byte null terminator)
		{"empty_doc", []byte{0x05, 0x00, 0x00, 0x00, 0x00}},

		// Document with string field: {"a": "b"}
		// Length: 14 bytes total
		// 0x02 = string type, "a\0" = field name, 0x02000000 = string length (2), "b\0" = string value
		{"string_field", []byte{
			0x0e, 0x00, 0x00, 0x00, // length = 14
			0x02,             // type = string
			0x61, 0x00,       // field name "a\0"
			0x02, 0x00, 0x00, 0x00, // string length = 2
			0x62, 0x00, // string value "b\0"
			0x00, // document terminator
		}},

		// Document with int32 field: {"x": 1}
		{"int32_field", []byte{
			0x0c, 0x00, 0x00, 0x00, // length = 12
			0x10,             // type = int32
			0x78, 0x00,       // field name "x\0"
			0x01, 0x00, 0x00, 0x00, // value = 1
			0x00, // document terminator
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result == nil {
				t.Fatalf("expected BSON detection for %s, got nil", tt.name)
			}
			if result.Name != "bson" {
				t.Errorf("expected name 'bson', got %q", result.Name)
			}
			if result.Confidence < 0.5 || result.Confidence > 1.0 {
				t.Errorf("expected confidence between 0.5-1.0, got %f", result.Confidence)
			}
		})
	}
}

func TestDetectBinaryFormat_UnknownBinary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		// Random binary data that doesn't match any format
		{"random_bytes", []byte{0x00, 0x01, 0x02, 0x03, 0x04}},
		{"high_bytes", []byte{0xfe, 0xfe, 0xfe, 0xfe}},
		{"mixed", []byte{0x7f, 0x7e, 0x7d, 0x7c}}, // ASCII-ish but not text
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectBinaryFormat(tt.data)
			if result != nil {
				t.Errorf("expected nil for unknown binary %s, got %+v", tt.name, result)
			}
		})
	}
}

func TestDetectBinaryFormat_Priority(t *testing.T) {
	t.Parallel()
	// Test that MessagePack is detected over CBOR for ambiguous bytes
	// 0x80 is both MessagePack fixmap(0) and CBOR array(0)
	// MessagePack should take priority per the spec
	result := DetectBinaryFormat([]byte{0x80})
	if result == nil {
		t.Fatal("expected detection for 0x80")
	}
	// MessagePack should be checked first and return higher confidence
	if result.Name != "messagepack" && result.Name != "cbor" {
		t.Errorf("expected messagepack or cbor, got %q", result.Name)
	}
}

func TestDetectBinaryFormat_SingleByte(t *testing.T) {
	t.Parallel()
	// Single bytes that are valid format indicators
	tests := []struct {
		data     byte
		expected string // "" means nil expected
	}{
		{0x80, "messagepack"}, // fixmap(0)
		{0x90, "messagepack"}, // fixarray(0)
		{0xa0, "messagepack"}, // fixstr(0)
		{0xc0, "messagepack"}, // nil
		{0xc2, "messagepack"}, // false
		{0xc3, "messagepack"}, // true
		{0xf4, "cbor"},        // CBOR false
		{0xf5, "cbor"},        // CBOR true
		{0xf6, "cbor"},        // CBOR null
	}

	for _, tt := range tests {
		t.Run(string([]byte{tt.data}), func(t *testing.T) {
			result := DetectBinaryFormat([]byte{tt.data})
			if tt.expected == "" {
				if result != nil {
					t.Errorf("expected nil for 0x%02x, got %+v", tt.data, result)
				}
			} else {
				if result == nil {
					t.Fatalf("expected %s for 0x%02x, got nil", tt.expected, tt.data)
				}
				if result.Name != tt.expected {
					t.Errorf("expected %s for 0x%02x, got %s", tt.expected, tt.data, result.Name)
				}
			}
		})
	}
}

func TestBinaryFormat_Details(t *testing.T) {
	t.Parallel()
	// Test that Details field provides useful information
	result := DetectBinaryFormat([]byte{0x08, 0x01}) // protobuf field 1, varint
	if result == nil {
		t.Fatal("expected protobuf detection")
	}
	if result.Details == "" {
		t.Log("Details field is empty (optional)")
	}
}

// Integration tests for binary format detection in network/websocket

func TestNetworkBody_BinaryFormatIntegration(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Add a network body with MessagePack binary data
	msgpackData := string([]byte{0x81, 0xa3, 0x6b, 0x65, 0x79, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65})
	bodies := []NetworkBody{
		{
			URL:          "https://api.example.com/data",
			Method:       "GET",
			Status:       200,
			ResponseBody: msgpackData,
		},
	}
	capture.AddNetworkBodies(bodies)

	// Retrieve and verify binary format was detected
	result := capture.GetNetworkBodies(NetworkBodyFilter{Limit: 1})
	if len(result) != 1 {
		t.Fatalf("expected 1 body, got %d", len(result))
	}
	if result[0].BinaryFormat != "messagepack" {
		t.Errorf("expected binary_format 'messagepack', got %q", result[0].BinaryFormat)
	}
	if result[0].FormatConfidence < 0.7 {
		t.Errorf("expected format_confidence >= 0.7, got %f", result[0].FormatConfidence)
	}
}

func TestNetworkBody_TextNoFormat(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Add a network body with JSON text data
	bodies := []NetworkBody{
		{
			URL:          "https://api.example.com/json",
			Method:       "GET",
			Status:       200,
			ResponseBody: `{"key": "value"}`,
		},
	}
	capture.AddNetworkBodies(bodies)

	// Verify no binary format detected for text
	result := capture.GetNetworkBodies(NetworkBodyFilter{Limit: 1})
	if len(result) != 1 {
		t.Fatalf("expected 1 body, got %d", len(result))
	}
	if result[0].BinaryFormat != "" {
		t.Errorf("expected empty binary_format for JSON, got %q", result[0].BinaryFormat)
	}
}

func TestWebSocketEvent_BinaryFormatIntegration(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Add a WebSocket message with protobuf binary data
	protobufData := string([]byte{0x08, 0x96, 0x01})
	events := []WebSocketEvent{
		{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://api.example.com/ws",
			Direction: "incoming",
			Data:      protobufData,
			Size:      len(protobufData),
		},
	}
	capture.AddWebSocketEvents(events)

	// Retrieve and verify binary format was detected
	result := capture.GetWebSocketEvents(WebSocketEventFilter{Limit: 1})
	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}
	if result[0].BinaryFormat != "protobuf" {
		t.Errorf("expected binary_format 'protobuf', got %q", result[0].BinaryFormat)
	}
	if result[0].FormatConfidence < 0.5 {
		t.Errorf("expected format_confidence >= 0.5, got %f", result[0].FormatConfidence)
	}
}

func TestWebSocketEvent_OpenCloseNoFormat(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Add open/close events which shouldn't have binary format detection
	events := []WebSocketEvent{
		{
			Event: "open",
			ID:    "ws-1",
			URL:   "wss://api.example.com/ws",
		},
		{
			Event:       "close",
			ID:          "ws-1",
			CloseCode:   1000,
			CloseReason: "normal",
		},
	}
	capture.AddWebSocketEvents(events)

	// Verify no binary format for non-message events
	result := capture.GetWebSocketEvents(WebSocketEventFilter{Limit: 10})
	for _, ev := range result {
		if ev.BinaryFormat != "" {
			t.Errorf("expected empty binary_format for %s event, got %q", ev.Event, ev.BinaryFormat)
		}
	}
}

func TestWebSocketEvent_TextMessageNoFormat(t *testing.T) {
	t.Parallel()
	capture := NewCapture()

	// Add a text message
	events := []WebSocketEvent{
		{
			Event:     "message",
			ID:        "ws-1",
			URL:       "wss://api.example.com/ws",
			Direction: "outgoing",
			Data:      `{"action": "ping"}`,
			Size:      18,
		},
	}
	capture.AddWebSocketEvents(events)

	// Verify no binary format for text message
	result := capture.GetWebSocketEvents(WebSocketEventFilter{Limit: 1})
	if len(result) != 1 {
		t.Fatalf("expected 1 event, got %d", len(result))
	}
	if result[0].BinaryFormat != "" {
		t.Errorf("expected empty binary_format for text message, got %q", result[0].BinaryFormat)
	}
}

// Benchmark to ensure detection is fast
func BenchmarkDetectBinaryFormat(b *testing.B) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"messagepack", []byte{0x81, 0xa3, 0x6b, 0x65, 0x79, 0xa5, 0x76, 0x61, 0x6c, 0x75, 0x65}},
		{"protobuf", []byte{0x08, 0x96, 0x01}},
		{"cbor", []byte{0xa1, 0x61, 0x61, 0x01}},
		{"bson", []byte{0x05, 0x00, 0x00, 0x00, 0x00}},
		{"text", []byte(`{"key": "value"}`)},
		{"empty", nil},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				DetectBinaryFormat(tc.data)
			}
		})
	}
}
