// Purpose: Detects MessagePack binary format by matching type markers and structure patterns.
// Why: Separates MessagePack detection heuristics from other binary format detectors.
package util

type binaryMarker struct {
	minLen     int
	confidence float64
	details    string
}

var msgpackMarkers = map[byte]binaryMarker{
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
