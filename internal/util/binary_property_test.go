// binary_property_test.go — Property-based tests for binary format detection.

package util

import (
	"testing"
	"testing/quick"
)

// TestPropertyConfidenceBounds verifies that for any []byte, if DetectBinaryFormat
// returns non-nil, Confidence is in [0.0, 1.0].
func TestPropertyConfidenceBounds(t *testing.T) {
	f := func(data []byte) bool {
		result := DetectBinaryFormat(data)

		if result == nil {
			return true // nil result is valid
		}

		// Confidence must be in [0.0, 1.0]
		return result.Confidence >= 0.0 && result.Confidence <= 1.0
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyNameValidity verifies that if non-nil, Name is one of the
// expected format names.
func TestPropertyNameValidity(t *testing.T) {
	validNames := map[string]bool{
		"messagepack": true,
		"protobuf":    true,
		"cbor":        true,
		"bson":        true,
	}

	f := func(data []byte) bool {
		result := DetectBinaryFormat(data)

		if result == nil {
			return true // nil result is valid
		}

		// Name must be one of the valid format names
		return validNames[result.Name]
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyASCIIImmunity verifies that for any string of only printable ASCII
// (0x20-0x7e), if it's long enough (>10 bytes), DetectBinaryFormat returns nil.
func TestPropertyASCIIImmunity(t *testing.T) {
	f := func(data []byte) bool {
		// Filter to only printable ASCII (0x20-0x7e)
		filtered := make([]byte, 0, len(data))
		for _, b := range data {
			if b >= 0x20 && b <= 0x7e {
				filtered = append(filtered, b)
			}
		}

		// If not long enough, skip this test case
		if len(filtered) <= 10 {
			return true // Vacuously true
		}

		// DetectBinaryFormat should return nil for pure ASCII
		result := DetectBinaryFormat(filtered)
		return result == nil
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyEmptyInputReturnsNil verifies that empty input always returns nil.
func TestPropertyEmptyInputReturnsNil(t *testing.T) {
	result := DetectBinaryFormat([]byte{})
	if result != nil {
		t.Errorf("DetectBinaryFormat([]) = %+v, want nil", result)
	}

	result = DetectBinaryFormat(nil)
	if result != nil {
		t.Errorf("DetectBinaryFormat(nil) = %+v, want nil", result)
	}
}

// TestPropertyDeterminism verifies that DetectBinaryFormat always returns
// the same result for the same input.
func TestPropertyDeterminism(t *testing.T) {
	f := func(data []byte) bool {
		first := DetectBinaryFormat(data)
		second := DetectBinaryFormat(data)

		// Both nil or both non-nil with same values
		if first == nil && second == nil {
			return true
		}

		if first == nil || second == nil {
			return false
		}

		return first.Name == second.Name && first.Confidence == second.Confidence
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyConfidenceNonNegative verifies that confidence is never negative.
func TestPropertyConfidenceNonNegative(t *testing.T) {
	f := func(data []byte) bool {
		result := DetectBinaryFormat(data)

		if result == nil {
			return true
		}

		return result.Confidence >= 0.0
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}
