// Purpose: Owns binary.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// binary.go — Binary format detection via magic bytes.
// Identifies protobuf, MessagePack, CBOR, BSON in network bodies and
// WebSocket messages. Detection is heuristic-based using format-specific
// byte patterns; confidence reflects pattern strength.
// Design: Check formats in order of specificity. MessagePack before CBOR
// due to overlapping ranges. Protobuf uses wire-type validation.
package util

// BinaryFormat describes a detected binary serialization format
type BinaryFormat struct {
	Name       string  // format identifier: "messagepack", "protobuf", "cbor", "bson"
	Confidence float64 // 0.0 - 1.0, higher = more certain
	Details    string  // optional: e.g., "protobuf field 1, varint"
}

// DetectBinaryFormat analyzes bytes and returns detected format.
// Returns nil if data is empty, text, or unknown binary format.
// Check order: MessagePack > CBOR > Protobuf > BSON (by specificity)
func DetectBinaryFormat(data []byte) *BinaryFormat {
	if len(data) == 0 {
		return nil
	}

	// Quick check: if all bytes are printable ASCII + common control chars, it's text
	if isLikelyText(data) {
		return nil
	}

	// MessagePack: distinctive type markers
	if format := detectMessagePack(data); format != nil {
		return format
	}

	// CBOR: similar structure but different markers
	if format := detectCBOR(data); format != nil {
		return format
	}

	// Protobuf: field tags with wire types
	if format := detectProtobuf(data); format != nil {
		return format
	}

	// BSON: length-prefixed documents
	if format := detectBSON(data); format != nil {
		return format
	}

	return nil
}

// isLikelyText returns true if data appears to be text content
func isLikelyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	// Count printable/text bytes
	textBytes := 0
	for _, b := range data {
		// Printable ASCII, newline, tab, carriage return
		if (b >= 0x20 && b <= 0x7e) || b == 0x0a || b == 0x0d || b == 0x09 {
			textBytes++
		}
	}

	// If >90% of bytes are text-like, treat as text
	return float64(textBytes)/float64(len(data)) > 0.9
}

// msgpackMarker defines a MessagePack type marker with its required data length and confidence.
type msgpackMarker struct {
	minLen     int     // minimum data length required (0 = no requirement)
	confidence float64 // detection confidence
	details    string  // format details
}

// msgpackMarkers maps byte values to their MessagePack marker info.
// Entries with minLen=0 need no length check beyond the first byte.
var msgpackMarkers = map[byte]msgpackMarker{
	0xc0: {0, 0.9, "nil"},
	0xc2: {0, 0.9, "false"},
	0xc3: {0, 0.9, "true"},
	0xc4: {0, 0.85, "bin"}, 0xc5: {0, 0.85, "bin"}, 0xc6: {0, 0.85, "bin"},
	0xc7: {0, 0.85, "ext"}, 0xc8: {0, 0.85, "ext"}, 0xc9: {0, 0.85, "ext"},
	0xca: {5, 0.85, "float32"}, 0xcb: {9, 0.85, "float64"},
	0xcc: {2, 0.8, "uint8"}, 0xcd: {3, 0.8, "uint16"}, 0xce: {5, 0.8, "uint32"}, 0xcf: {9, 0.8, "uint64"},
	0xd0: {2, 0.8, "int8"}, 0xd1: {3, 0.8, "int16"}, 0xd2: {5, 0.8, "int32"}, 0xd3: {9, 0.8, "int64"},
	0xd4: {0, 0.85, "fixext"}, 0xd5: {0, 0.85, "fixext"}, 0xd6: {0, 0.85, "fixext"},
	0xd7: {0, 0.85, "fixext"}, 0xd8: {0, 0.85, "fixext"},
	0xd9: {2, 0.8, "str8"}, 0xda: {3, 0.8, "str16"}, 0xdb: {5, 0.8, "str32"},
	0xdc: {3, 0.85, "array16"}, 0xdd: {5, 0.85, "array32"},
	0xde: {3, 0.85, "map16"}, 0xdf: {5, 0.85, "map32"},
}

// detectMessagePackRange checks if the byte falls in a MessagePack fixed-type range.
func detectMessagePackRange(b byte) *BinaryFormat {
	switch {
	case b >= 0x80 && b <= 0x8f:
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "fixmap"}
	case b >= 0x90 && b <= 0x9f:
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "fixarray"}
	case b >= 0xa0 && b <= 0xbf:
		return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "fixstr"}
	default:
		return nil
	}
}

// detectMessagePack checks for MessagePack format markers
func detectMessagePack(data []byte) *BinaryFormat {
	if len(data) == 0 {
		return nil
	}

	b := data[0]

	if result := detectMessagePackRange(b); result != nil {
		return result
	}

	m, ok := msgpackMarkers[b]
	if !ok {
		return nil
	}
	if m.minLen > 0 && len(data) < m.minLen {
		return nil
	}
	return &BinaryFormat{Name: "messagepack", Confidence: m.confidence, Details: m.details}
}

// cborSimpleMarkers maps specific byte values in CBOR major type 7 to their format info.
var cborSimpleMarkers = map[byte]msgpackMarker{
	0xf4: {0, 0.9, "false"},
	0xf5: {0, 0.9, "true"},
	0xf6: {0, 0.9, "null"},
	0xf7: {0, 0.9, "undefined"},
	0xf9: {3, 0.85, "float16"},
	0xfa: {5, 0.85, "float32"},
	0xfb: {9, 0.85, "float64"},
	0xff: {0, 0.8, "break"},
}

// detectCBORMajorType handles CBOR major types 4 (array) and 5 (map).
func detectCBORMajorType(majorType, additionalInfo byte) *BinaryFormat {
	if majorType != 4 && majorType != 5 {
		return nil
	}
	if additionalInfo > 0x17 && additionalInfo != 0x1f {
		return nil
	}
	details := "array"
	if majorType == 5 {
		details = "map"
	}
	return &BinaryFormat{Name: "cbor", Confidence: 0.75, Details: details}
}

// detectCBOR checks for CBOR format markers
func detectCBOR(data []byte) *BinaryFormat {
	if len(data) == 0 {
		return nil
	}

	b := data[0]
	majorType := b >> 5
	additionalInfo := b & 0x1f

	if result := detectCBORMajorType(majorType, additionalInfo); result != nil {
		return result
	}

	if majorType == 6 {
		return &BinaryFormat{Name: "cbor", Confidence: 0.85, Details: "tagged"}
	}

	if majorType == 7 {
		if m, ok := cborSimpleMarkers[b]; ok {
			if m.minLen > 0 && len(data) < m.minLen {
				return nil
			}
			return &BinaryFormat{Name: "cbor", Confidence: m.confidence, Details: m.details}
		}
	}

	return nil
}

// isValidProtobufWireType returns true if the wire type is valid (0, 1, 2, or 5).
func isValidProtobufWireType(wireType byte) bool {
	return wireType == 0 || wireType == 1 || wireType == 2 || wireType == 5
}

// protobufFieldDetail builds the detail string for a protobuf field.
func protobufFieldDetail(fieldNumber byte, wireTypeStr string) string {
	return "field " + string('0'+fieldNumber) + ", " + wireTypeStr
}

// isValidVarint checks if data starting at offset 1 looks like a valid varint encoding.
func isValidVarint(data []byte) bool {
	for i := 1; i < len(data) && i < 10; i++ {
		if data[i]&0x80 == 0 {
			return true
		}
	}
	return len(data) < 10
}

// detectProtobufLengthDelimited checks if data looks like a protobuf length-delimited field.
func detectProtobufLengthDelimited(data []byte, fieldNumber byte) *BinaryFormat {
	if data[1]&0x80 != 0 {
		return &BinaryFormat{Name: "protobuf", Confidence: 0.6, Details: protobufFieldDetail(fieldNumber, "length-delimited")}
	}
	length := int(data[1])
	if length > 0 && len(data) >= 2+length {
		return &BinaryFormat{Name: "protobuf", Confidence: 0.7, Details: protobufFieldDetail(fieldNumber, "length-delimited")}
	}
	return nil
}

// detectProtobuf checks for protobuf wire format patterns
func detectProtobuf(data []byte) *BinaryFormat {
	if len(data) < 2 {
		return nil
	}

	wireType := data[0] & 0x07
	fieldNumber := data[0] >> 3

	if !isValidProtobufWireType(wireType) || fieldNumber == 0 || fieldNumber > 15 {
		return nil
	}

	switch wireType {
	case 0: // Varint
		if isValidVarint(data) {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.7, Details: protobufFieldDetail(fieldNumber, "varint")}
		}
	case 1: // 64-bit fixed
		if len(data) >= 9 {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.65, Details: protobufFieldDetail(fieldNumber, "fixed64")}
		}
	case 2: // Length-delimited
		return detectProtobufLengthDelimited(data, fieldNumber)
	case 5: // 32-bit fixed
		if len(data) >= 5 {
			return &BinaryFormat{Name: "protobuf", Confidence: 0.65, Details: protobufFieldDetail(fieldNumber, "fixed32")}
		}
	}

	return nil
}

// bsonDocLen reads a BSON document length (little-endian int32) from the first 4 bytes.
func bsonDocLen(data []byte) int {
	return int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24
}

// isValidBSONElementType returns true if the byte is a valid BSON element type.
func isValidBSONElementType(b byte) bool {
	return b == 0x00 || (b >= 0x01 && b <= 0x13) || b == 0x7f || b == 0xff
}

// detectBSON checks for BSON document format
func detectBSON(data []byte) *BinaryFormat {
	if len(data) < 5 {
		return nil
	}

	docLen := bsonDocLen(data)
	if docLen < 5 || docLen > 16*1024*1024 {
		return nil
	}

	// Verify null terminator if we have the full document
	if len(data) >= docLen && data[docLen-1] != 0x00 {
		return nil
	}

	// Document length must encompass or exceed the data
	if docLen < len(data) {
		return nil
	}

	// Validate first element type if present
	if len(data) > 4 && isValidBSONElementType(data[4]) {
		return &BinaryFormat{Name: "bson", Confidence: 0.65, Details: "document"}
	}
	if len(data) <= 4 {
		return &BinaryFormat{Name: "bson", Confidence: 0.5, Details: "document (partial)"}
	}

	return nil
}
