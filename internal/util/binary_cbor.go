// Purpose: Detects CBOR binary format by matching simple value markers and major type headers.
// Why: Separates CBOR detection heuristics from other binary format detectors.
package util

var cborSimpleMarkers = map[byte]binaryMarker{
	0xf4: {0, 0.9, "false"},
	0xf5: {0, 0.9, "true"},
	0xf6: {0, 0.9, "null"},
	0xf7: {0, 0.9, "undefined"},
	0xf9: {3, 0.85, "float16"},
	0xfa: {5, 0.85, "float32"},
	0xfb: {9, 0.85, "float64"},
	0xff: {0, 0.8, "break"},
}

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
