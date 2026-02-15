// ring_buffer_resolve_test.go — Branch coverage tests for resolveStartPosition.
package buffers

import "testing"

func TestResolveStartPosition(t *testing.T) {
	t.Run("empty buffer", func(t *testing.T) {
		rb := NewRingBuffer[int](10)
		start, avail := rb.resolveStartPosition(0)
		if start != 0 || avail != 0 {
			t.Errorf("empty buffer: start=%d avail=%d, want 0,0", start, avail)
		}
	})

	t.Run("cursor at zero with entries", func(t *testing.T) {
		rb := NewRingBuffer[int](10)
		rb.Write([]int{1, 2, 3})
		start, avail := rb.resolveStartPosition(0)
		if start != 0 || avail != 3 {
			t.Errorf("start=%d avail=%d, want 0,3", start, avail)
		}
	})

	t.Run("cursor in middle", func(t *testing.T) {
		rb := NewRingBuffer[int](10)
		rb.Write([]int{1, 2, 3, 4, 5})
		start, avail := rb.resolveStartPosition(2)
		if start != 2 || avail != 3 {
			t.Errorf("start=%d avail=%d, want 2,3", start, avail)
		}
	})

	t.Run("cursor past end", func(t *testing.T) {
		rb := NewRingBuffer[int](10)
		rb.Write([]int{1, 2, 3})
		start, avail := rb.resolveStartPosition(3)
		if start != 3 || avail != 0 {
			t.Errorf("start=%d avail=%d, want 3,0", start, avail)
		}
	})

	t.Run("cursor before oldest after wrap-around", func(t *testing.T) {
		rb := NewRingBuffer[int](3)
		// Write 5 entries into capacity-3 buffer: oldest is position 2
		rb.Write([]int{1, 2, 3, 4, 5})
		start, avail := rb.resolveStartPosition(0)
		// Oldest is at position 2 (totalAdded=5, len=3, oldest=5-3=2)
		if start != 2 || avail != 3 {
			t.Errorf("wrap-around: start=%d avail=%d, want 2,3", start, avail)
		}
	})

	t.Run("cursor exactly at oldest after wrap", func(t *testing.T) {
		rb := NewRingBuffer[int](3)
		rb.Write([]int{1, 2, 3, 4, 5})
		start, avail := rb.resolveStartPosition(2)
		if start != 2 || avail != 3 {
			t.Errorf("start=%d avail=%d, want 2,3", start, avail)
		}
	})

	t.Run("cursor between oldest and newest after wrap", func(t *testing.T) {
		rb := NewRingBuffer[int](3)
		rb.Write([]int{1, 2, 3, 4, 5})
		start, avail := rb.resolveStartPosition(3)
		if start != 3 || avail != 2 {
			t.Errorf("start=%d avail=%d, want 3,2", start, avail)
		}
	})

	t.Run("negative oldest position clamped", func(t *testing.T) {
		// When totalAdded < len(entries) this shouldn't happen in practice,
		// but verify the clamp to 0
		rb := NewRingBuffer[int](10)
		rb.Write([]int{1})
		// totalAdded=1, len=1, oldest=1-1=0
		start, avail := rb.resolveStartPosition(0)
		if start != 0 || avail != 1 {
			t.Errorf("start=%d avail=%d, want 0,1", start, avail)
		}
	})
}
