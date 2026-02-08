// ring_buffer.go — Generic ring buffer with cursor-based reads.
// Provides a fixed-capacity circular buffer that tracks timestamps for each entry,
// enabling clients to read from specific positions or time points.
// Thread-safe: all access guarded by RWMutex (see LOCKING.md for ordering).
package buffers

import (
	"sync"
	"time"
)

// ============================================
// BufferCursor
// ============================================

// BufferCursor tracks a read position in a ring buffer.
// The timestamp allows detecting when the buffer has wrapped and
// the position is no longer valid (all data at that position has been evicted).
type BufferCursor struct {
	Position  int64     // Monotonic position in the buffer (total items ever added)
	Timestamp time.Time // When this position was last valid
}

// ============================================
// RingBuffer
// ============================================

// RingBuffer is a generic fixed-capacity circular buffer with timestamp tracking.
// Entries are evicted in FIFO order when capacity is reached.
// Supports cursor-based reads so multiple clients can maintain independent read positions.
type RingBuffer[T any] struct {
	mu sync.RWMutex

	// Buffer storage
	entries  []T
	addedAt  []time.Time // Parallel slice: when each entry was added
	capacity int

	// Position tracking
	totalAdded int64 // Monotonic counter of all entries ever added
	head       int   // Index where next write goes
}

// NewRingBuffer creates a new ring buffer with the given capacity.
func NewRingBuffer[T any](capacity int) *RingBuffer[T] {
	return &RingBuffer[T]{
		entries:  make([]T, 0, capacity),
		addedAt:  make([]time.Time, 0, capacity),
		capacity: capacity,
	}
}

// Write appends entries to the buffer, evicting oldest entries if at capacity.
// Returns the number of entries actually written (may be less if entries > capacity).
func (rb *RingBuffer[T]) Write(entries []T) int {
	if len(entries) == 0 {
		return 0
	}

	rb.mu.Lock()
	defer rb.mu.Unlock()

	now := time.Now()
	written := 0

	for _, entry := range entries {
		rb.writeOneLocked(entry, now)
		written++
	}

	return written
}

// WriteOne appends a single entry to the buffer.
func (rb *RingBuffer[T]) WriteOne(entry T) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.writeOneLocked(entry, time.Now())
}

// writeOneLocked adds one entry, must be called with mu held.
func (rb *RingBuffer[T]) writeOneLocked(entry T, addedAt time.Time) {
	if len(rb.entries) < rb.capacity {
		// Buffer not full, append
		rb.entries = append(rb.entries, entry)
		rb.addedAt = append(rb.addedAt, addedAt)
	} else {
		// Buffer full, overwrite at head position
		rb.entries[rb.head] = entry
		rb.addedAt[rb.head] = addedAt
	}
	rb.head = (rb.head + 1) % rb.capacity
	rb.totalAdded++
}

// ReadFrom reads entries starting from the given cursor position.
// Returns entries added after the cursor position and a new cursor for subsequent reads.
// If the cursor position has been evicted (buffer wrapped), reads from the oldest available.
func (rb *RingBuffer[T]) ReadFrom(cursor BufferCursor) ([]T, BufferCursor) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return nil, BufferCursor{Position: rb.totalAdded, Timestamp: time.Now()}
	}

	// Calculate the oldest position still in buffer
	oldestPosition := rb.totalAdded - int64(len(rb.entries))
	if oldestPosition < 0 {
		oldestPosition = 0
	}

	// Determine where to start reading
	startPosition := cursor.Position
	if startPosition < oldestPosition {
		// Cursor position has been evicted, start from oldest
		startPosition = oldestPosition
	}

	// Calculate how many entries to return
	entriesAvailable := rb.totalAdded - startPosition
	if entriesAvailable <= 0 {
		return nil, BufferCursor{Position: rb.totalAdded, Timestamp: time.Now()}
	}

	// Calculate the index in the circular buffer for startPosition
	startIndex := rb.positionToIndex(startPosition)

	// Extract entries in order
	result := make([]T, 0, entriesAvailable)
	for i := int64(0); i < entriesAvailable; i++ {
		idx := int((int64(startIndex) + i) % int64(len(rb.entries)))
		result = append(result, rb.entries[idx])
	}

	return result, BufferCursor{Position: rb.totalAdded, Timestamp: time.Now()}
}

// ReadAll returns all entries currently in the buffer, oldest first.
func (rb *RingBuffer[T]) ReadAll() []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return nil
	}

	result := make([]T, len(rb.entries))
	if len(rb.entries) < rb.capacity {
		// Buffer not full, entries are in order from 0
		copy(result, rb.entries)
	} else {
		// Buffer full, head points to oldest entry
		n := copy(result, rb.entries[rb.head:])
		copy(result[n:], rb.entries[:rb.head])
	}
	return result
}

// ReadLast returns the last n entries, oldest first.
func (rb *RingBuffer[T]) ReadLast(n int) []T {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 || n <= 0 {
		return nil
	}

	if n > len(rb.entries) {
		n = len(rb.entries)
	}

	result := make([]T, n)

	// Calculate starting position
	if len(rb.entries) < rb.capacity {
		// Buffer not full, entries are in order
		startIdx := len(rb.entries) - n
		copy(result, rb.entries[startIdx:])
	} else {
		// Buffer full and wrapped
		// The "newest" entry is at (head-1+capacity)%capacity
		// We want the last n entries ending at the newest
		endIdx := (rb.head - 1 + rb.capacity) % rb.capacity
		for i := n - 1; i >= 0; i-- {
			result[i] = rb.entries[endIdx]
			endIdx = (endIdx - 1 + rb.capacity) % rb.capacity
		}
	}
	return result
}

// FindPositionAtTime returns the position of the first entry added at or after the given time.
// Returns -1 if no entries are at or after the given time.
// This handles buffer wraparound by checking timestamps.
func (rb *RingBuffer[T]) FindPositionAtTime(t time.Time) int64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return -1
	}

	oldestPosition := rb.totalAdded - int64(len(rb.entries))
	if oldestPosition < 0 {
		oldestPosition = 0
	}

	// Linear scan to find first entry at or after time t
	// Note: Entries are time-ordered, so binary search (O(log n)) would be more efficient
	// than linear scan (O(n)). Current buffer sizes (500-1000) make this acceptable.
	// Future optimization: implement binary search for larger buffers.
	for i := 0; i < len(rb.entries); i++ {
		idx := rb.positionToIndex(oldestPosition + int64(i))
		if !rb.addedAt[idx].Before(t) {
			return oldestPosition + int64(i)
		}
	}

	return -1 // All entries are before time t
}

// GetCurrentPosition returns the current total added count (next write position).
func (rb *RingBuffer[T]) GetCurrentPosition() int64 {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.totalAdded
}

// Len returns the number of entries currently in the buffer.
func (rb *RingBuffer[T]) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return len(rb.entries)
}

// Cap returns the buffer capacity.
func (rb *RingBuffer[T]) Cap() int {
	return rb.capacity // Immutable, no lock needed
}

// Clear removes all entries from the buffer.
func (rb *RingBuffer[T]) Clear() {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	rb.entries = nil
	rb.addedAt = nil
	rb.head = 0
	// Don't reset totalAdded - cursors need it to stay monotonic
}

// positionToIndex converts a monotonic position to a buffer index.
// Must be called with at least a read lock held.
func (rb *RingBuffer[T]) positionToIndex(position int64) int {
	if len(rb.entries) < rb.capacity {
		// Buffer not full, positions map directly to indices
		return int(position)
	}
	// Buffer is full and wrapped
	// oldestPosition is at index rb.head
	oldestPosition := rb.totalAdded - int64(len(rb.entries))
	offset := int(position - oldestPosition)
	return (rb.head + offset) % rb.capacity
}

// ============================================
// Filtered Read Helpers
// ============================================

// ReadFromWithFilter reads entries from cursor and applies a filter function.
// Only entries where filter returns true are included in the result.
func (rb *RingBuffer[T]) ReadFromWithFilter(cursor BufferCursor, filter func(T) bool, limit int) ([]T, BufferCursor) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if len(rb.entries) == 0 {
		return nil, BufferCursor{Position: rb.totalAdded, Timestamp: time.Now()}
	}

	oldestPosition := rb.totalAdded - int64(len(rb.entries))
	if oldestPosition < 0 {
		oldestPosition = 0
	}

	startPosition := cursor.Position
	if startPosition < oldestPosition {
		startPosition = oldestPosition
	}

	entriesAvailable := rb.totalAdded - startPosition
	if entriesAvailable <= 0 {
		return nil, BufferCursor{Position: rb.totalAdded, Timestamp: time.Now()}
	}

	startIndex := rb.positionToIndex(startPosition)

	capHint := int(entriesAvailable)
	if limit > 0 && limit < capHint {
		capHint = limit
	}
	result := make([]T, 0, capHint)
	for i := int64(0); i < entriesAvailable; i++ {
		idx := int((int64(startIndex) + i) % int64(len(rb.entries)))
		entry := rb.entries[idx]
		if filter(entry) {
			result = append(result, entry)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result, BufferCursor{Position: rb.totalAdded, Timestamp: time.Now()}
}

// ReadAllWithFilter returns all entries that pass the filter, oldest first.
func (rb *RingBuffer[T]) ReadAllWithFilter(filter func(T) bool, limit int) []T {
	entries := rb.ReadAll()
	if entries == nil {
		return nil
	}

	capHint := len(entries)
	if limit > 0 && limit < capHint {
		capHint = limit
	}
	result := make([]T, 0, capHint)
	for _, entry := range entries {
		if filter(entry) {
			result = append(result, entry)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}
