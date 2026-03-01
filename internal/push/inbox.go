// inbox.go — Bounded FIFO queue for push events that couldn't be delivered.
package push

import (
	"sync"
	"time"
)

// PushInbox is a thread-safe bounded FIFO queue for push events.
type PushInbox struct {
	mu     sync.Mutex
	events []PushEvent
	maxLen int
}

// NewPushInbox creates an inbox with the given capacity.
func NewPushInbox(maxLen int) *PushInbox {
	if maxLen <= 0 {
		maxLen = 50
	}
	return &PushInbox{
		events: make([]PushEvent, 0, maxLen),
		maxLen: maxLen,
	}
}

// Enqueue adds an event, evicting oldest if full. Returns eviction count.
func (q *PushInbox) Enqueue(ev PushEvent) int {
	q.mu.Lock()
	defer q.mu.Unlock()

	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}

	q.events = append(q.events, ev)

	evicted := 0
	if len(q.events) > q.maxLen {
		evicted = len(q.events) - q.maxLen
		kept := make([]PushEvent, q.maxLen)
		copy(kept, q.events[evicted:])
		q.events = kept
	}
	return evicted
}

// DrainAll returns all events and clears the queue.
// Replaces the internal slice to release references to large PushEvents.
func (q *PushInbox) DrainAll() []PushEvent {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.events) == 0 {
		return nil
	}
	out := q.events
	q.events = make([]PushEvent, 0, q.maxLen)
	return out
}

// Peek returns all events without clearing.
func (q *PushInbox) Peek() []PushEvent {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.events) == 0 {
		return nil
	}
	out := make([]PushEvent, len(q.events))
	copy(out, q.events)
	return out
}

// Len returns the current queue length.
func (q *PushInbox) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.events)
}
