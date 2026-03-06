// Purpose: Implements binary payload format detection heuristics (MessagePack/CBOR/Protobuf/BSON).
// Why: Enables safer telemetry handling by classifying opaque network bodies before downstream processing.
// Docs: docs/features/feature/binary-format-detection/index.md

package util

type BinaryFormat struct {
	Name       string
	Confidence float64
	Details    string
}

// DetectBinaryFormat analyzes bytes and returns detected format.
// Returns nil if data is empty, text, or unknown binary format.
// Detection order: MessagePack > CBOR > Protobuf > BSON.
func DetectBinaryFormat(data []byte) *BinaryFormat {
	if len(data) == 0 || isLikelyText(data) {
		return nil
	}

	if format := detectMessagePack(data); format != nil {
		return format
	}
	if format := detectCBOR(data); format != nil {
		return format
	}
	if format := detectProtobuf(data); format != nil {
		return format
	}
	if format := detectBSON(data); format != nil {
		return format
	}

	return nil
}

func isLikelyText(data []byte) bool {
	if len(data) == 0 {
		return false
	}

	textBytes := 0
	for _, b := range data {
		if (b >= 0x20 && b <= 0x7e) || b == 0x0a || b == 0x0d || b == 0x09 {
			textBytes++
		}
	}

	return float64(textBytes)/float64(len(data)) > 0.9
}
