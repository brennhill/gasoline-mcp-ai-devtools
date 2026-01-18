// refactoring_verification_test.go — Post-refactoring verification tests for buffer types.
//
// These tests verify that the buffer package types work correctly after the
// interface{}/any refactoring in version 5.4. They serve as smoke tests to
// catch regressions introduced by type system changes.
package buffers

import (
	"testing"
	"time"
)

// ============================================
// Test 1: BufferCursor Type Correctness
// ============================================

func TestBufferCursor_TypeCorrectness(t *testing.T) {
	t.Parallel()

	t.Run("BufferCursor fields have correct types", func(t *testing.T) {
		cursor := BufferCursor{
			Position:  int64(42),
			Timestamp: time.Now(),
		}

		// Verify Position is int64 (not any)
		var _ int64 = cursor.Position

		// Verify Timestamp is time.Time (not any)
		var _ time.Time = cursor.Timestamp

		if cursor.Position != 42 {
			t.Errorf("Expected Position 42, got %d", cursor.Position)
		}
	})

	t.Run("BufferCursor zero value is valid", func(t *testing.T) {
		var cursor BufferCursor

		if cursor.Position != 0 {
			t.Errorf("Zero cursor should have Position 0, got %d", cursor.Position)
		}

		if !cursor.Timestamp.IsZero() {
			t.Errorf("Zero cursor should have zero Timestamp")
		}
	})
}

// ============================================
// Test 2: RingBuffer Generic Type Safety
// ============================================

func TestRingBuffer_GenericTypeSafety(t *testing.T) {
	t.Parallel()

	t.Run("RingBuffer[int] maintains type safety", func(t *testing.T) {
		rb := NewRingBuffer[int](10)

		rb.Write([]int{1, 2, 3})

		entries := rb.ReadAll()
		for i, entry := range entries {
			// entry should be int, not any
			var _ int = entry

			expected := i + 1
			if entry != expected {
				t.Errorf("Entry %d: expected %d, got %d", i, expected, entry)
			}
		}
	})

	t.Run("RingBuffer[string] maintains type safety", func(t *testing.T) {
		rb := NewRingBuffer[string](10)

		rb.Write([]string{"a", "b", "c"})

		entries := rb.ReadAll()
		for _, entry := range entries {
			// entry should be string, not any
			var _ string = entry

			if entry == "" {
				t.Error("Entry should not be empty")
			}
		}
	})

	t.Run("RingBuffer with struct type", func(t *testing.T) {
		type TestEntry struct {
			ID   int
			Name string
		}

		rb := NewRingBuffer[TestEntry](10)

		rb.Write([]TestEntry{
			{ID: 1, Name: "first"},
			{ID: 2, Name: "second"},
		})

		entries := rb.ReadAll()
		for i, entry := range entries {
			// entry should be TestEntry, not any
			var _ TestEntry = entry

			if entry.ID != i+1 {
				t.Errorf("Entry %d: expected ID %d, got %d", i, i+1, entry.ID)
			}
		}
	})
}

// ============================================
// Test 3: ReadFrom Returns Correct Types
// ============================================

func TestRingBuffer_ReadFromTypeSafety(t *testing.T) {
	t.Parallel()

	t.Run("ReadFrom returns typed slice and BufferCursor", func(t *testing.T) {
		rb := NewRingBuffer[int](10)
		rb.Write([]int{10, 20, 30})

		entries, cursor := rb.ReadFrom(BufferCursor{Position: 0})

		// entries should be []int, not []any
		var _ []int = entries

		// cursor should be BufferCursor
		var _ BufferCursor = cursor

		if len(entries) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(entries))
		}

		if cursor.Position != 3 {
			t.Errorf("Expected cursor position 3, got %d", cursor.Position)
		}
	})

	t.Run("ReadFromWithFilter returns correctly typed slice", func(t *testing.T) {
		rb := NewRingBuffer[int](10)
		rb.Write([]int{1, 2, 3, 4, 5})

		even := func(n int) bool { return n%2 == 0 }
		entries, cursor := rb.ReadFromWithFilter(BufferCursor{}, even, 0)

		// entries should be []int
		var _ []int = entries

		// cursor should be BufferCursor
		var _ BufferCursor = cursor

		if len(entries) != 2 {
			t.Errorf("Expected 2 even entries, got %d", len(entries))
		}
	})
}

// ============================================
// Test 4: Buffer Operations After Refactoring
// ============================================

func TestRingBuffer_OperationsAfterRefactoring(t *testing.T) {
	t.Parallel()

	t.Run("WriteOne accepts correct type", func(t *testing.T) {
		rb := NewRingBuffer[int](10)

		// This should compile - WriteOne takes T, not any
		rb.WriteOne(42)

		entries := rb.ReadAll()
		if len(entries) != 1 || entries[0] != 42 {
			t.Errorf("WriteOne failed: got %v", entries)
		}
	})

	t.Run("ReadLast returns correctly typed slice", func(t *testing.T) {
		rb := NewRingBuffer[string](10)
		rb.Write([]string{"a", "b", "c", "d", "e"})

		last2 := rb.ReadLast(2)

		// last2 should be []string
		var _ []string = last2

		if len(last2) != 2 {
			t.Errorf("Expected 2 entries, got %d", len(last2))
		}
		if last2[0] != "d" || last2[1] != "e" {
			t.Errorf("Expected [d, e], got %v", last2)
		}
	})

	t.Run("ReadAllWithFilter returns correctly typed results", func(t *testing.T) {
		rb := NewRingBuffer[int](10)
		rb.Write([]int{1, 2, 3, 4, 5})

		greaterThan3 := func(n int) bool { return n > 3 }
		results := rb.ReadAllWithFilter(greaterThan3, 0)

		// results should be []int
		var _ []int = results

		if len(results) != 2 {
			t.Errorf("Expected 2 results > 3, got %d", len(results))
		}
	})
}

// ============================================
// Test 5: Cursor Continuity After Wraparound
// ============================================

func TestRingBuffer_CursorContinuityAfterRefactoring(t *testing.T) {
	t.Parallel()

	t.Run("cursor position maintains type after wraparound", func(t *testing.T) {
		rb := NewRingBuffer[int](3)

		// Write enough to wrap around
		rb.Write([]int{1, 2, 3, 4, 5})

		// Get cursor after writes
		_, cursor := rb.ReadFrom(BufferCursor{Position: 0})

		// cursor.Position should be int64, not any
		var _ int64 = cursor.Position

		if cursor.Position != 5 {
			t.Errorf("Expected cursor position 5, got %d", cursor.Position)
		}
	})

	t.Run("cursor handles eviction correctly with proper types", func(t *testing.T) {
		rb := NewRingBuffer[int](3)

		rb.Write([]int{1, 2, 3, 4, 5}) // buffer has [3, 4, 5], positions 0-1 evicted

		// Read from evicted position - should return from oldest available
		entries, cursor := rb.ReadFrom(BufferCursor{Position: 0})

		// entries should be []int
		var _ []int = entries

		if len(entries) != 3 {
			t.Errorf("Expected 3 entries, got %d", len(entries))
		}

		if entries[0] != 3 {
			t.Errorf("First entry should be 3 (oldest available), got %d", entries[0])
		}

		if cursor.Position != 5 {
			t.Errorf("Expected cursor position 5, got %d", cursor.Position)
		}
	})
}
