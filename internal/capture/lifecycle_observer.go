// lifecycle_observer.go — Typed lifecycle event bus with panic isolation and unsubscribe support.
// Why: Replaces closure-chaining pattern (AddLifecycleCallback) with a proper observer.
// Improvements: typed events, slice-based listeners, panic recovery in Emit, Unsubscribe support.
package capture

import (
	"fmt"
	"os"
	"sync"
)

// LifecycleEvent is a typed enum for capture lifecycle events.
type LifecycleEvent int

const (
	EventUnknown                LifecycleEvent = iota
	EventCircuitOpened                         // Circuit breaker opened (rate exceeded)
	EventCircuitClosed                         // Circuit breaker closed (recovered)
	EventExtensionConnected                    // Extension connected or reconnected
	EventExtensionDisconnected                 // Extension disconnected (poll timeout)
	EventBufferEviction                        // Ring buffer evicted old entries
	EventRateLimitTriggered                    // Rate limit threshold hit
	EventCommandStateDesync                    // Command state mismatch with extension
	EventSyncSnapshot                          // Periodic sync state snapshot
)

// eventNames maps typed events to their wire-format string names.
var eventNames = map[LifecycleEvent]string{
	EventUnknown:                "unknown",
	EventCircuitOpened:          "circuit_opened",
	EventCircuitClosed:          "circuit_closed",
	EventExtensionConnected:     "extension_connected",
	EventExtensionDisconnected:  "extension_disconnected",
	EventBufferEviction:         "buffer_eviction",
	EventRateLimitTriggered:     "rate_limit_triggered",
	EventCommandStateDesync:     "command_state_desync",
	EventSyncSnapshot:           "sync_snapshot",
}

// stringToEvent maps wire-format string names to typed events (reverse of eventNames).
var stringToEvent map[string]LifecycleEvent

func init() {
	stringToEvent = make(map[string]LifecycleEvent, len(eventNames))
	for ev, name := range eventNames {
		stringToEvent[name] = ev
	}
}

// String returns the wire-format name for a lifecycle event.
func (e LifecycleEvent) String() string {
	if name, ok := eventNames[e]; ok {
		return name
	}
	return "unknown"
}

// ParseLifecycleEvent converts a string event name to its typed enum.
// Returns EventUnknown for unrecognized strings.
func ParseLifecycleEvent(s string) LifecycleEvent {
	if ev, ok := stringToEvent[s]; ok {
		return ev
	}
	return EventUnknown
}

// LifecycleListener is the callback signature for lifecycle event subscribers.
type LifecycleListener func(event LifecycleEvent, data map[string]any)

// listenerEntry pairs a listener with a stable subscription ID.
type listenerEntry struct {
	id int
	fn LifecycleListener
}

// LifecycleObserver is a concurrency-safe event bus for capture lifecycle events.
// Supports multiple listeners, unsubscribe by ID, and panic isolation per listener.
type LifecycleObserver struct {
	mu        sync.RWMutex
	listeners []listenerEntry
	nextID    int
}

// NewLifecycleObserver creates an empty observer ready for subscriptions.
func NewLifecycleObserver() *LifecycleObserver {
	return &LifecycleObserver{}
}

// Subscribe registers a listener and returns a subscription ID for later removal.
func (o *LifecycleObserver) Subscribe(fn LifecycleListener) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	id := o.nextID
	o.nextID++
	o.listeners = append(o.listeners, listenerEntry{id: id, fn: fn})
	return id
}

// Unsubscribe removes a listener by its subscription ID. No-op if ID not found.
func (o *LifecycleObserver) Unsubscribe(id int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, entry := range o.listeners {
		if entry.id == id {
			o.listeners = append(o.listeners[:i], o.listeners[i+1:]...)
			return
		}
	}
}

// Emit dispatches an event to all listeners. Each listener is called with panic
// recovery so one misbehaving listener cannot break others. Listeners are called
// synchronously in subscription order; callers should wrap in util.SafeGo if needed.
func (o *LifecycleObserver) Emit(event LifecycleEvent, data map[string]any) {
	o.mu.RLock()
	snapshot := make([]listenerEntry, len(o.listeners))
	copy(snapshot, o.listeners)
	o.mu.RUnlock()

	for _, entry := range snapshot {
		func(fn LifecycleListener) {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "[gasoline] lifecycle observer: listener panic on %s: %v\n", event.String(), r)
				}
			}()
			fn(event, data)
		}(entry.fn)
	}
}

// EmitString converts a string event name to a typed event and emits it.
// Backward compatibility bridge for callers that still use string event names.
func (o *LifecycleObserver) EmitString(event string, data map[string]any) {
	o.Emit(ParseLifecycleEvent(event), data)
}

// EmitFunc returns a func(string, map[string]any) suitable for injection into
// subsystems (e.g., CircuitBreaker) that expect the old string-based callback signature.
func (o *LifecycleObserver) EmitFunc() func(string, map[string]any) {
	return func(event string, data map[string]any) {
		o.EmitString(event, data)
	}
}

