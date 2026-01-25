// binary.go — Binary format detection via magic bytes.
// Identifies protobuf, MessagePack, CBOR, BSON in network bodies and
// WebSocket messages. Detection is heuristic-based using format-specific
// byte patterns; confidence reflects pattern strength.
// Design: Check formats in order of specificity. MessagePack before CBOR
// due to overlapping ranges. Protobuf uses wire-type validation.
package main

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

// detectMessagePack checks for MessagePack format markers
func detectMessagePack(data []byte) *BinaryFormat {
	if len(data) == 0 {
		return nil
	}

	b := data[0]

	// fixmap: 0x80-0x8f (map with 0-15 elements)
	if b >= 0x80 && b <= 0x8f {
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "fixmap"}
	}

	// fixarray: 0x90-0x9f (array with 0-15 elements)
	if b >= 0x90 && b <= 0x9f {
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "fixarray"}
	}

	// fixstr: 0xa0-0xbf (string with 0-31 bytes)
	if b >= 0xa0 && b <= 0xbf {
		return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "fixstr"}
	}

	// Specific type markers (high confidence)
	switch b {
	case 0xc0: // nil
		return &BinaryFormat{Name: "messagepack", Confidence: 0.9, Details: "nil"}
	case 0xc2: // false
		return &BinaryFormat{Name: "messagepack", Confidence: 0.9, Details: "false"}
	case 0xc3: // true
		return &BinaryFormat{Name: "messagepack", Confidence: 0.9, Details: "true"}
	case 0xc4, 0xc5, 0xc6: // bin8, bin16, bin32
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "bin"}
	case 0xc7, 0xc8, 0xc9: // ext8, ext16, ext32
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "ext"}
	case 0xca: // float32
		if len(data) >= 5 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "float32"}
		}
	case 0xcb: // float64
		if len(data) >= 9 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "float64"}
		}
	case 0xcc: // uint8
		if len(data) >= 2 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "uint8"}
		}
	case 0xcd: // uint16
		if len(data) >= 3 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "uint16"}
		}
	case 0xce: // uint32
		if len(data) >= 5 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "uint32"}
		}
	case 0xcf: // uint64
		if len(data) >= 9 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "uint64"}
		}
	case 0xd0: // int8
		if len(data) >= 2 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "int8"}
		}
	case 0xd1: // int16
		if len(data) >= 3 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "int16"}
		}
	case 0xd2: // int32
		if len(data) >= 5 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "int32"}
		}
	case 0xd3: // int64
		if len(data) >= 9 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "int64"}
		}
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8: // fixext 1,2,4,8,16
		return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "fixext"}
	case 0xd9: // str8
		if len(data) >= 2 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "str8"}
		}
	case 0xda: // str16
		if len(data) >= 3 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "str16"}
		}
	case 0xdb: // str32
		if len(data) >= 5 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.8, Details: "str32"}
		}
	case 0xdc: // array16
		if len(data) >= 3 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "array16"}
		}
	case 0xdd: // array32
		if len(data) >= 5 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "array32"}
		}
	case 0xde: // map16
		if len(data) >= 3 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "map16"}
		}
	case 0xdf: // map32
		if len(data) >= 5 {
			return &BinaryFormat{Name: "messagepack", Confidence: 0.85, Details: "map32"}
		}
	}

	return nil
}

// detectCBOR checks for CBOR format markers
func detectCBOR(data []byte) *BinaryFormat {
	if len(data) == 0 {
		return nil
	}

	b := data[0]
	majorType := b >> 5
	additionalInfo := b & 0x1f

	switch majorType {
	case 4: // Array (0x80-0x9f)
		// Note: overlaps with MessagePack, but MessagePack is checked first
		if additionalInfo <= 0x17 || additionalInfo == 0x1f { // definite or indefinite
			return &BinaryFormat{Name: "cbor", Confidence: 0.75, Details: "array"}
		}
	case 5: // Map (0xa0-0xbf)
		// Note: overlaps with MessagePack fixstr
		if additionalInfo <= 0x17 || additionalInfo == 0x1f {
			return &BinaryFormat{Name: "cbor", Confidence: 0.75, Details: "map"}
		}
	case 6: // Tagged value (0xc0-0xdf)
		// CBOR tags are distinctive
		return &BinaryFormat{Name: "cbor", Confidence: 0.85, Details: "tagged"}
	case 7: // Simple/float (0xe0-0xff)
		switch b {
		case 0xf4: // false
			return &BinaryFormat{Name: "cbor", Confidence: 0.9, Details: "false"}
		case 0xf5: // true
			return &BinaryFormat{Name: "cbor", Confidence: 0.9, Details: "true"}
		case 0xf6: // null
			return &BinaryFormat{Name: "cbor", Confidence: 0.9, Details: "null"}
		case 0xf7: // undefined
			return &BinaryFormat{Name: "cbor", Confidence: 0.9, Details: "undefined"}
		case 0xf9: // float16
			if len(data) >= 3 {
				return &BinaryFormat{Name: "cbor", Confidence: 0.85, Details: "float16"}
			}
		case 0xfa: // float32
			if len(data) >= 5 {
				return &BinaryFormat{Name: "cbor", Confidence: 0.85, Details: "float32"}
			}
		case 0xfb: // float64
			if len(data) >= 9 {
				return &BinaryFormat{Name: "cbor", Confidence: 0.85, Details: "float64"}
			}
		case 0xff: // break (for indefinite)
			return &BinaryFormat{Name: "cbor", Confidence: 0.8, Details: "break"}
		}
	}

	return nil
}

// detectProtobuf checks for protobuf wire format patterns
func detectProtobuf(data []byte) *BinaryFormat {
	if len(data) < 2 {
		return nil
	}

	// Protobuf messages start with field tags
	// Field tag = (field_number << 3) | wire_type
	// Wire types: 0=varint, 1=64-bit, 2=length-delimited, 5=32-bit

	b := data[0]
	wireType := b & 0x07
	fieldNumber := b >> 3

	// Valid wire types are 0, 1, 2, 5 (3 and 4 are deprecated)
	if wireType == 3 || wireType == 4 || wireType > 5 {
		return nil
	}

	// Field number must be 1-15 for single-byte tag (most common)
	// Field 0 is reserved
	if fieldNumber == 0 || fieldNumber > 15 {
		return nil
	}

	// Validate based on wire type
	switch wireType {
	case 0: // Varint
		// Next byte should be a valid varint byte
		if len(data) >= 2 {
			// Check that subsequent bytes look like valid varints
			// High bit set means continuation
			validVarint := true
			for i := 1; i < len(data) && i < 10; i++ {
				if data[i]&0x80 == 0 {
					// End of varint
					break
				}
				if i >= 10 {
					validVarint = false
				}
			}
			if validVarint {
				return &BinaryFormat{
					Name:       "protobuf",
					Confidence: 0.7,
					Details:    "field " + string('0'+byte(fieldNumber)) + ", varint",
				}
			}
		}
	case 1: // 64-bit fixed
		if len(data) >= 9 {
			return &BinaryFormat{
				Name:       "protobuf",
				Confidence: 0.65,
				Details:    "field " + string('0'+byte(fieldNumber)) + ", fixed64",
			}
		}
	case 2: // Length-delimited
		if len(data) >= 2 {
			length := int(data[1])
			if data[1]&0x80 != 0 {
				// Multi-byte length encoding
				return &BinaryFormat{
					Name:       "protobuf",
					Confidence: 0.6,
					Details:    "field " + string('0'+byte(fieldNumber)) + ", length-delimited",
				}
			}
			// Check if length is reasonable
			if length > 0 && len(data) >= 2+length {
				return &BinaryFormat{
					Name:       "protobuf",
					Confidence: 0.7,
					Details:    "field " + string('0'+byte(fieldNumber)) + ", length-delimited",
				}
			}
		}
	case 5: // 32-bit fixed
		if len(data) >= 5 {
			return &BinaryFormat{
				Name:       "protobuf",
				Confidence: 0.65,
				Details:    "field " + string('0'+byte(fieldNumber)) + ", fixed32",
			}
		}
	}

	return nil
}

// detectBSON checks for BSON document format
func detectBSON(data []byte) *BinaryFormat {
	// BSON document: int32 (little-endian) document size + elements + 0x00
	// Minimum valid BSON is 5 bytes: length(4) + terminator(1)
	if len(data) < 5 {
		return nil
	}

	// Read document length (little-endian int32)
	docLen := int(data[0]) | int(data[1])<<8 | int(data[2])<<16 | int(data[3])<<24

	// Sanity checks on document length
	if docLen < 5 {
		return nil
	}

	// If we have the full document, verify null terminator
	if len(data) >= docLen {
		if data[docLen-1] != 0x00 {
			return nil
		}
	}

	// Check if length is reasonable (not absurdly large and matches roughly)
	if docLen > 16*1024*1024 { // 16MB max BSON doc size
		return nil
	}

	// If document length matches data length or data is prefix of full doc
	if docLen == len(data) || docLen > len(data) {
		// Additional validation: check first element type if present
		if len(data) > 4 {
			elemType := data[4]
			// Valid BSON element types: 0x01-0x13, 0x7f, 0xff, or 0x00 (terminator)
			if elemType == 0x00 || (elemType >= 0x01 && elemType <= 0x13) || elemType == 0x7f || elemType == 0xff {
				return &BinaryFormat{
					Name:       "bson",
					Confidence: 0.65,
					Details:    "document",
				}
			}
		} else {
			// Just have the length, less confident
			return &BinaryFormat{
				Name:       "bson",
				Confidence: 0.5,
				Details:    "document (partial)",
			}
		}
	}

	return nil
}
