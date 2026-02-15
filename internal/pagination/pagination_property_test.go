// pagination_property_test.go — Property-based tests for pagination cursors.

package pagination

import (
	"testing"
	"testing/quick"
	"time"
)

// TestPropertyCursorRoundTrip verifies that ParseCursor(BuildCursor(ts, seq))
// produces the same ts and seq for valid inputs.
func TestPropertyCursorRoundTrip(t *testing.T) {
	f := func(sec int64, seq int64) bool {
		// Clamp timestamp to reasonable range (0 to ~2033)
		if sec < 0 {
			sec = -sec
		}
		sec = sec % 2000000000

		// Clamp sequence to positive values
		if seq < 0 {
			seq = -seq
		}

		// Generate timestamp string
		ts := time.Unix(sec, 0).UTC().Format(time.RFC3339)

		// Build cursor
		cursorStr := BuildCursor(ts, seq)

		// Parse cursor back
		parsed, err := ParseCursor(cursorStr)
		if err != nil {
			return false
		}

		// Verify round-trip
		return parsed.Timestamp == ts && parsed.Sequence == seq
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyIsOlderIsNewerTrichotomy verifies that for any cursor and entry
// with different timestamps or sequences, exactly one of IsOlder or IsNewer
// returns true (or both false if equal).
func TestPropertyIsOlderIsNewerTrichotomy(t *testing.T) {
	f := func(seq1, seq2 int64) bool {
		// Clamp sequences to positive values
		if seq1 < 0 {
			seq1 = -seq1
		}
		if seq2 < 0 {
			seq2 = -seq2
		}

		// Use sequence-only cursors for simplicity (Timestamp="")
		cursorStr := BuildCursor("", seq1)
		cursor, err := ParseCursor(cursorStr)
		if err != nil {
			return false
		}

		entryTimestamp := ""
		entrySequence := seq2

		isOlder := cursor.IsOlder(entryTimestamp, entrySequence)
		isNewer := cursor.IsNewer(entryTimestamp, entrySequence)

		// Trichotomy: exactly one is true, or both false if equal
		if seq1 == seq2 {
			// Equal: both should be false
			return !isOlder && !isNewer
		} else if seq1 > seq2 {
			// cursor is newer than entry: entry is older
			return isOlder && !isNewer
		} else {
			// cursor is older than entry: entry is newer
			return !isOlder && isNewer
		}
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyBuildCursorSequenceOnly verifies that when timestamp is "",
// BuildCursor("", seq) produces ":seq" and ParseCursor roundtrips correctly.
func TestPropertyBuildCursorSequenceOnly(t *testing.T) {
	f := func(seq int64) bool {
		// Clamp sequence to positive values
		if seq < 0 {
			seq = -seq
		}

		// Build sequence-only cursor
		cursor := BuildCursor("", seq)

		// Parse it back
		parsed, err := ParseCursor(cursor)
		if err != nil {
			return false
		}

		// Verify timestamp is empty and sequence matches
		return parsed.Timestamp == "" && parsed.Sequence == seq
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyCursorOrderingTransitivity verifies that cursor ordering is transitive.
// If A < B and B < C, then A < C.
func TestPropertyCursorOrderingTransitivity(t *testing.T) {
	f := func(seq1, seq2, seq3 int64) bool {
		// Clamp and sort sequences
		seqs := []int64{seq1, seq2, seq3}
		for i := range seqs {
			if seqs[i] < 0 {
				seqs[i] = -seqs[i]
			}
		}

		// Sort to ensure seq1 <= seq2 <= seq3
		for i := 0; i < len(seqs); i++ {
			for j := i + 1; j < len(seqs); j++ {
				if seqs[i] > seqs[j] {
					seqs[i], seqs[j] = seqs[j], seqs[i]
				}
			}
		}

		seq1, seq2, seq3 = seqs[0], seqs[1], seqs[2]

		cursorA, err := ParseCursor(BuildCursor("", seq1))
		if err != nil {
			return false
		}
		cursorB, err := ParseCursor(BuildCursor("", seq2))
		if err != nil {
			return false
		}

		// If A < B (IsOlder(A, B)) and B < C (IsOlder(B, C)), then A < C
		aOlderB := cursorA.IsOlder("", seq2)
		bOlderC := cursorB.IsOlder("", seq3)
		aOlderC := cursorA.IsOlder("", seq3)

		// Transitivity: if both conditions hold, then A < C must hold
		if aOlderB && bOlderC {
			return aOlderC
		}

		// If conditions don't hold, property is vacuously true
		return true
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}

// TestPropertyParseCursorHandlesInvalid verifies that ParseCursor handles
// malformed cursors gracefully.
func TestPropertyParseCursorHandlesInvalid(t *testing.T) {
	f := func(s string) bool {
		// ParseCursor should never panic
		parsed, err := ParseCursor(s)

		// If parsing failed with error, that's acceptable
		if err != nil {
			return true
		}

		// If parsing succeeded, sequence should be non-negative
		if parsed.Sequence < 0 {
			return false
		}

		// Timestamp can be any string (including empty)
		_ = parsed.Timestamp

		return true
	}

	cfg := &quick.Config{MaxCount: 1000}
	if err := quick.Check(f, cfg); err != nil {
		t.Error(err)
	}
}
