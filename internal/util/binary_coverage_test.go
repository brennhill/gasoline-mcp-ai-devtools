// binary_coverage_test.go — Coverage tests for isLikelyText, CBOR, and MessagePack detection.
// Targets uncovered branches in text threshold, CBOR major types, tagged values,
// simple markers, and MessagePack length-gated markers.
package util

import (
	"testing"
)

// ---------------------------------------------------------------------------
// isLikelyText — empty input and threshold boundary
// ---------------------------------------------------------------------------

func TestIsLikelyText_Empty(t *testing.T) {
	t.Parallel()
	if isLikelyText(nil) {
		t.Error("isLikelyText(nil) = true, want false")
	}
	if isLikelyText([]byte{}) {
		t.Error("isLikelyText([]byte{}) = true, want false")
	}
}

func TestIsLikelyText_MostlyBinary(t *testing.T) {
	t.Parallel()
	data := make([]byte, 100)
	for i := range data {
		data[i] = 0x01
	}
	if isLikelyText(data) {
		t.Error("isLikelyText(all binary) = true, want false")
	}
}

func TestIsLikelyText_ExactlyAtThreshold(t *testing.T) {
	t.Parallel()
	data := make([]byte, 100)
	for i := 0; i < 91; i++ {
		data[i] = 'a'
	}
	for i := 91; i < 100; i++ {
		data[i] = 0x01
	}
	if !isLikelyText(data) {
		t.Error("isLikelyText(91% text) = false, want true")
	}
}

func TestIsLikelyText_BelowThreshold(t *testing.T) {
	t.Parallel()
	data := make([]byte, 100)
	for i := 0; i < 90; i++ {
		data[i] = 'a'
	}
	for i := 90; i < 100; i++ {
		data[i] = 0x01
	}
	if isLikelyText(data) {
		t.Error("isLikelyText(exactly 90% text) = true, want false")
	}
}

func TestIsLikelyText_ControlChars(t *testing.T) {
	t.Parallel()
	data := []byte{0x09, 0x0a, 0x0d, 'a', 'b', 'c'}
	if !isLikelyText(data) {
		t.Error("isLikelyText(with control chars) = false, want true")
	}
}

// ---------------------------------------------------------------------------
// detectCBORMajorType — cover all branches
// ---------------------------------------------------------------------------

func TestDetectCBORMajorType_Array(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(4, 0x00)
	if result == nil {
		t.Fatal("detectCBORMajorType(4, 0x00) = nil, want array")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Confidence != 0.75 {
		t.Errorf("Confidence = %f, want 0.75", result.Confidence)
	}
	if result.Details != "array" {
		t.Errorf("Details = %q, want array", result.Details)
	}
}

func TestDetectCBORMajorType_ArrayMaxInfo(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(4, 0x17)
	if result == nil {
		t.Fatal("detectCBORMajorType(4, 0x17) = nil, want array")
	}
	if result.Details != "array" {
		t.Errorf("Details = %q, want array", result.Details)
	}
}

func TestDetectCBORMajorType_ArrayIndefinite(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(4, 0x1f)
	if result == nil {
		t.Fatal("detectCBORMajorType(4, 0x1f) = nil, want array")
	}
	if result.Details != "array" {
		t.Errorf("Details = %q, want array", result.Details)
	}
}

func TestDetectCBORMajorType_Map(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(5, 0x03)
	if result == nil {
		t.Fatal("detectCBORMajorType(5, 0x03) = nil, want map")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "map" {
		t.Errorf("Details = %q, want map", result.Details)
	}
}

func TestDetectCBORMajorType_MapIndefinite(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(5, 0x1f)
	if result == nil {
		t.Fatal("detectCBORMajorType(5, 0x1f) = nil, want map")
	}
	if result.Details != "map" {
		t.Errorf("Details = %q, want map", result.Details)
	}
}

func TestDetectCBORMajorType_InvalidAdditionalInfo(t *testing.T) {
	t.Parallel()
	result := detectCBORMajorType(4, 0x18)
	if result != nil {
		t.Errorf("detectCBORMajorType(4, 0x18) = %+v, want nil", result)
	}
	result = detectCBORMajorType(5, 0x1c)
	if result != nil {
		t.Errorf("detectCBORMajorType(5, 0x1c) = %+v, want nil", result)
	}
}

func TestDetectCBORMajorType_OtherMajorTypes(t *testing.T) {
	t.Parallel()
	for _, mt := range []byte{0, 1, 2, 3, 6, 7} {
		result := detectCBORMajorType(mt, 0x00)
		if result != nil {
			t.Errorf("detectCBORMajorType(%d, 0) = %+v, want nil", mt, result)
		}
	}
}

// ---------------------------------------------------------------------------
// detectCBOR — tagged values, simple markers, and array/map via majorType
// ---------------------------------------------------------------------------

func TestDetectCBOR_Empty(t *testing.T) {
	t.Parallel()
	if detectCBOR(nil) != nil {
		t.Error("detectCBOR(nil) != nil")
	}
	if detectCBOR([]byte{}) != nil {
		t.Error("detectCBOR([]byte{}) != nil")
	}
}

func TestDetectCBOR_Tagged(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0xc6, 0x01})
	if result == nil {
		t.Fatal("detectCBOR(tagged 0xc6) = nil, want cbor tagged")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "tagged" {
		t.Errorf("Details = %q, want tagged", result.Details)
	}
	if result.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", result.Confidence)
	}
}

func TestDetectCBOR_SimpleMarkerInsufficientLength(t *testing.T) {
	t.Parallel()
	if result := detectCBOR([]byte{0xf9, 0x00}); result != nil {
		t.Errorf("detectCBOR(float16 short) = %+v, want nil", result)
	}
	if result := detectCBOR([]byte{0xfa, 0x00, 0x00}); result != nil {
		t.Errorf("detectCBOR(float32 short) = %+v, want nil", result)
	}
	if result := detectCBOR([]byte{0xfb, 0x00, 0x00, 0x00, 0x00}); result != nil {
		t.Errorf("detectCBOR(float64 short) = %+v, want nil", result)
	}
}

func TestDetectCBOR_BreakCode(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0xff})
	if result == nil {
		t.Fatal("detectCBOR(0xff) = nil, want cbor break")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "break" {
		t.Errorf("Details = %q, want break", result.Details)
	}
	if result.Confidence != 0.8 {
		t.Errorf("Confidence = %f, want 0.8", result.Confidence)
	}
}

func TestDetectCBOR_UndefinedSimple(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0xf7})
	if result == nil {
		t.Fatal("detectCBOR(0xf7) = nil, want cbor undefined")
	}
	if result.Details != "undefined" {
		t.Errorf("Details = %q, want undefined", result.Details)
	}
}

func TestDetectCBOR_MajorType7_UnknownSimple(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0xf8, 0x20})
	if result != nil {
		t.Errorf("detectCBOR(0xf8 unknown simple) = %+v, want nil", result)
	}
}

func TestDetectCBOR_NoMatchMajorType(t *testing.T) {
	t.Parallel()
	result := detectCBOR([]byte{0x00})
	if result != nil {
		t.Errorf("detectCBOR(majorType 0) = %+v, want nil", result)
	}
}

func TestDetectCBOR_ArrayViaMajorType(t *testing.T) {
	t.Parallel()
	// 0x83 = majorType 4, additionalInfo 3 (3-element array)
	result := detectCBOR([]byte{0x83, 0x01, 0x02, 0x03})
	if result == nil {
		t.Fatal("detectCBOR(array byte) = nil, want cbor array")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "array" {
		t.Errorf("Details = %q, want array", result.Details)
	}
	if result.Confidence != 0.75 {
		t.Errorf("Confidence = %f, want 0.75", result.Confidence)
	}
}

func TestDetectCBOR_MapViaMajorType(t *testing.T) {
	t.Parallel()
	// 0xA1 = majorType 5, additionalInfo 1 (1-element map)
	result := detectCBOR([]byte{0xa1, 0x01, 0x02})
	if result == nil {
		t.Fatal("detectCBOR(map byte) = nil, want cbor map")
	}
	if result.Name != "cbor" {
		t.Errorf("Name = %q, want cbor", result.Name)
	}
	if result.Details != "map" {
		t.Errorf("Details = %q, want map", result.Details)
	}
}

// ---------------------------------------------------------------------------
// detectMessagePack — empty, insufficient length, markers
// ---------------------------------------------------------------------------

func TestDetectMessagePack_Empty(t *testing.T) {
	t.Parallel()
	if detectMessagePack(nil) != nil {
		t.Error("detectMessagePack(nil) != nil")
	}
	if detectMessagePack([]byte{}) != nil {
		t.Error("detectMessagePack([]byte{}) != nil")
	}
}

func TestDetectMessagePack_InsufficientLength(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data []byte
	}{
		{"float32_short", []byte{0xca, 0x00, 0x00}},
		{"float64_short", []byte{0xcb, 0x00, 0x00, 0x00, 0x00}},
		{"uint8_short", []byte{0xcc}},
		{"uint16_short", []byte{0xcd, 0x00}},
		{"uint32_short", []byte{0xce, 0x00, 0x00}},
		{"uint64_short", []byte{0xcf, 0x00, 0x00, 0x00, 0x00}},
		{"int8_short", []byte{0xd0}},
		{"int16_short", []byte{0xd1, 0x00}},
		{"int32_short", []byte{0xd2, 0x00, 0x00}},
		{"int64_short", []byte{0xd3, 0x00, 0x00, 0x00, 0x00}},
		{"str8_short", []byte{0xd9}},
		{"str16_short", []byte{0xda, 0x00}},
		{"str32_short", []byte{0xdb, 0x00, 0x00}},
		{"array16_short", []byte{0xdc, 0x00}},
		{"array32_short", []byte{0xdd, 0x00, 0x00}},
		{"map16_short", []byte{0xde, 0x00}},
		{"map32_short", []byte{0xdf, 0x00, 0x00}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if result := detectMessagePack(tt.data); result != nil {
				t.Errorf("detectMessagePack(%s) = %+v, want nil", tt.name, result)
			}
		})
	}
}

func TestDetectMessagePack_NotInMarkers(t *testing.T) {
	t.Parallel()
	if result := detectMessagePack([]byte{0xc1}); result != nil {
		t.Errorf("detectMessagePack(0xc1) = %+v, want nil", result)
	}
}

func TestDetectMessagePack_BinMarkers(t *testing.T) {
	t.Parallel()
	for _, b := range []byte{0xc4, 0xc5, 0xc6} {
		result := detectMessagePack([]byte{b})
		if result == nil {
			t.Errorf("detectMessagePack(0x%02x) = nil, want bin", b)
			continue
		}
		if result.Name != "messagepack" {
			t.Errorf("detectMessagePack(0x%02x).Name = %q, want messagepack", b, result.Name)
		}
		if result.Details != "bin" {
			t.Errorf("detectMessagePack(0x%02x).Details = %q, want bin", b, result.Details)
		}
	}
}

func TestDetectMessagePack_ExtMarkers(t *testing.T) {
	t.Parallel()
	for _, b := range []byte{0xc7, 0xc8, 0xc9} {
		result := detectMessagePack([]byte{b})
		if result == nil {
			t.Errorf("detectMessagePack(0x%02x) = nil, want ext", b)
			continue
		}
		if result.Details != "ext" {
			t.Errorf("detectMessagePack(0x%02x).Details = %q, want ext", b, result.Details)
		}
	}
}

func TestDetectMessagePack_FixextMarkers(t *testing.T) {
	t.Parallel()
	for _, b := range []byte{0xd4, 0xd5, 0xd6, 0xd7, 0xd8} {
		result := detectMessagePack([]byte{b})
		if result == nil {
			t.Errorf("detectMessagePack(0x%02x) = nil, want fixext", b)
			continue
		}
		if result.Details != "fixext" {
			t.Errorf("detectMessagePack(0x%02x).Details = %q, want fixext", b, result.Details)
		}
	}
}

// ---------------------------------------------------------------------------
// detectMessagePackRange — boundary values
// ---------------------------------------------------------------------------

func TestDetectMessagePackRange_Boundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		b       byte
		details string
		isNil   bool
	}{
		{0x7f, "", true},
		{0x80, "fixmap", false},
		{0x8f, "fixmap", false},
		{0x90, "fixarray", false},
		{0x9f, "fixarray", false},
		{0xa0, "fixstr", false},
		{0xbf, "fixstr", false},
		{0xc0, "", true},
	}
	for _, tt := range tests {
		result := detectMessagePackRange(tt.b)
		if tt.isNil {
			if result != nil {
				t.Errorf("detectMessagePackRange(0x%02x) = %+v, want nil", tt.b, result)
			}
		} else {
			if result == nil {
				t.Errorf("detectMessagePackRange(0x%02x) = nil, want %s", tt.b, tt.details)
				continue
			}
			if result.Details != tt.details {
				t.Errorf("detectMessagePackRange(0x%02x).Details = %q, want %q", tt.b, result.Details, tt.details)
			}
		}
	}
}
