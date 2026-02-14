// binary_fuzz_test.go — Fuzz tests for binary format detection.

package util

import (
	"testing"
)

func FuzzDetectBinaryFormat(f *testing.F) {
	// MessagePack seeds
	f.Add([]byte{0xc0})                                   // nil
	f.Add([]byte{0x80})                                   // fixmap
	f.Add([]byte{0x90})                                   // fixarray
	f.Add([]byte{0xde, 0x00, 0x01})                       // map16
	f.Add([]byte{0x91, 0x01})                             // array[1]
	f.Add([]byte{0xcc, 0xff})                             // uint8

	// CBOR seeds
	f.Add([]byte{0xf4})                                   // false
	f.Add([]byte{0xf5})                                   // true
	f.Add([]byte{0xf6})                                   // null
	f.Add([]byte{0xa1, 0x61, 0x31})                       // {"a": "1"}

	// Protobuf seeds
	f.Add([]byte{0x08, 0x01})                             // field 1, varint=1
	f.Add([]byte{0x0a, 0x05, 0x68, 0x65, 0x6c, 0x6c, 0x6f}) // field 1, "hello"
	f.Add([]byte{0x10, 0x96, 0x01})                       // field 2, varint=150

	// BSON seeds
	f.Add([]byte{0x05, 0x00, 0x00, 0x00, 0x00})           // minimal empty document
	f.Add([]byte{0x0c, 0x00, 0x00, 0x00, 0x02, 0x61, 0x00, 0x02, 0x00, 0x00, 0x00, 0x62, 0x00}) // {"a":"b"}

	// Non-binary seeds (should return nil)
	f.Add([]byte{0x89, 0x50, 0x4e, 0x47})                 // PNG magic
	f.Add([]byte("Hello world"))                          // ASCII text
	f.Add([]byte(`{"key":"value"}`))                      // JSON
	f.Add([]byte{})                                       // empty
	f.Add([]byte("a"))                                    // single char
	f.Add([]byte{0x00})                                   // null byte
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})                 // all high bits

	f.Fuzz(func(t *testing.T, data []byte) {
		// Call the function
		result := DetectBinaryFormat(data)

		// Invariant 1: Empty data → nil result
		if len(data) == 0 {
			if result != nil {
				t.Errorf("Expected nil for empty data, got %+v", result)
			}
			return
		}

		// If result is nil, no further checks needed
		if result == nil {
			return
		}

		// Invariant 2: Confidence must be in [0.0, 1.0]
		if result.Confidence < 0.0 || result.Confidence > 1.0 {
			t.Errorf("Confidence out of range [0.0, 1.0]: %f for data: %v", result.Confidence, data)
		}

		// Invariant 3: Name must be one of the known formats
		validNames := map[string]bool{
			"messagepack": true,
			"protobuf":    true,
			"cbor":        true,
			"bson":        true,
		}
		if !validNames[result.Name] {
			t.Errorf("Invalid format name: %q (data: %v)", result.Name, data)
		}

		// Invariant 4: Pure printable ASCII (>90% threshold) should return nil
		// This matches the isLikelyText logic
		if len(data) >= 10 {
			printableCount := 0
			for _, b := range data {
				if b >= 0x20 && b <= 0x7e {
					printableCount++
				}
			}
			printableRatio := float64(printableCount) / float64(len(data))
			if printableRatio > 0.90 {
				t.Errorf("High printable ASCII ratio (%.2f) should return nil, got %+v (data: %q)",
					printableRatio, result, string(data))
			}
		}

		// Invariant 5: Determinism - calling twice with same input returns same result
		result2 := DetectBinaryFormat(data)
		if result2 == nil {
			t.Errorf("Determinism violation: first call returned %+v, second returned nil", result)
			return
		}

		if result.Name != result2.Name {
			t.Errorf("Determinism violation in Name: %q != %q", result.Name, result2.Name)
		}
		if result.Confidence != result2.Confidence {
			t.Errorf("Determinism violation in Confidence: %f != %f", result.Confidence, result2.Confidence)
		}
		if result.Details != result2.Details {
			t.Errorf("Determinism violation in Details: %q != %q", result.Details, result2.Details)
		}
	})
}
