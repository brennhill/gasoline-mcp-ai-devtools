// progress.go — Scaffold progress broadcaster for WebSocket streaming.

package scaffold

import (
	"sync"
)

// subscriber holds a channel subscription with its target channel name.
type subscriber struct {
	channel string
	ch      chan StepEvent
}

// Broadcaster distributes scaffold progress events to WebSocket subscribers.
// Thread-safe. Non-blocking on slow consumers (events dropped if buffer full).
type Broadcaster struct {
	mu          sync.RWMutex
	subscribers map[<-chan StepEvent]*subscriber
	closed      bool
}

// NewBroadcaster creates a new progress broadcaster.
func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		subscribers: make(map[<-chan StepEvent]*subscriber),
	}
}

// Subscribe registers a new subscriber for the given channel name.
// Returns a read-only channel that receives events for that channel.
//
// IMPORTANT: The caller MUST call Unsubscribe on disconnect (e.g., when the
// WebSocket connection closes) to prevent subscriber leaks. If the provided
// context is cancelled, the caller should treat that as a signal to unsubscribe.
func (b *Broadcaster) Subscribe(channel string) <-chan StepEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan StepEvent, 64)
	sub := &subscriber{channel: channel, ch: ch}
	b.subscribers[ch] = sub
	return ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (b *Broadcaster) Unsubscribe(ch <-chan StepEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sub, ok := b.subscribers[ch]; ok {
		close(sub.ch)
		delete(b.subscribers, ch)
	}
}

// Broadcast sends an event to all subscribers on the given channel.
// Non-blocking: if a subscriber's buffer is full, the event is dropped.
func (b *Broadcaster) Broadcast(channel string, evt StepEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		if sub.channel != channel {
			continue
		}
		select {
		case sub.ch <- evt:
		default:
			// Drop event for slow consumer.
		}
	}
}

// Close shuts down the broadcaster and closes all subscriber channels.
func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return
	}
	b.closed = true
	for ch, sub := range b.subscribers {
		close(sub.ch)
		delete(b.subscribers, ch)
	}
}
