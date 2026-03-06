// lifecycle_observer_test.go — Tests for LifecycleObserver event bus.
package capture

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestLifecycleObserver_SubscribeAndEmit(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	var received LifecycleEvent
	var receivedData map[string]any
	obs.Subscribe(func(event LifecycleEvent, data map[string]any) {
		received = event
		receivedData = data
	})

	obs.Emit(EventExtensionConnected, map[string]any{"key": "value"})

	if received != EventExtensionConnected {
		t.Errorf("received event = %d, want %d", received, EventExtensionConnected)
	}
	if receivedData["key"] != "value" {
		t.Errorf("received data = %v, want key=value", receivedData)
	}
}

func TestLifecycleObserver_MultipleListeners(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	var count atomic.Int32
	obs.Subscribe(func(LifecycleEvent, map[string]any) { count.Add(1) })
	obs.Subscribe(func(LifecycleEvent, map[string]any) { count.Add(1) })
	obs.Subscribe(func(LifecycleEvent, map[string]any) { count.Add(1) })

	obs.Emit(EventCircuitOpened, nil)

	if got := count.Load(); got != 3 {
		t.Errorf("listener count = %d, want 3", got)
	}
}

func TestLifecycleObserver_Unsubscribe(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	var count atomic.Int32
	id := obs.Subscribe(func(LifecycleEvent, map[string]any) { count.Add(1) })
	obs.Unsubscribe(id)

	obs.Emit(EventCircuitClosed, nil)

	if got := count.Load(); got != 0 {
		t.Errorf("listener count = %d, want 0 after unsubscribe", got)
	}
}

func TestLifecycleObserver_UnsubscribeNonexistent(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	// Should not panic
	obs.Unsubscribe(999)
}

func TestLifecycleObserver_PanicIsolation(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	var secondCalled atomic.Bool
	obs.Subscribe(func(LifecycleEvent, map[string]any) {
		panic("listener panic")
	})
	obs.Subscribe(func(LifecycleEvent, map[string]any) {
		secondCalled.Store(true)
	})

	// Should not panic; second listener should still run
	obs.Emit(EventExtensionDisconnected, nil)

	if !secondCalled.Load() {
		t.Error("second listener should run despite first listener panic")
	}
}

func TestLifecycleObserver_EmitNoListeners(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	// Should not panic
	obs.Emit(EventCircuitOpened, nil)
}

func TestLifecycleObserver_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := obs.Subscribe(func(LifecycleEvent, map[string]any) {})
			obs.Emit(EventSyncSnapshot, nil)
			obs.Unsubscribe(id)
		}()
	}
	wg.Wait()
}

func TestLifecycleObserver_EmitString(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	var received LifecycleEvent
	obs.Subscribe(func(event LifecycleEvent, data map[string]any) {
		received = event
	})

	obs.EmitString("extension_connected", map[string]any{"key": "val"})

	if received != EventExtensionConnected {
		t.Errorf("received event = %d, want %d (EventExtensionConnected)", received, EventExtensionConnected)
	}
}

func TestLifecycleObserver_EmitString_Unknown(t *testing.T) {
	t.Parallel()
	obs := NewLifecycleObserver()

	var received LifecycleEvent
	obs.Subscribe(func(event LifecycleEvent, data map[string]any) {
		received = event
	})

	obs.EmitString("some_unknown_event", nil)

	if received != EventUnknown {
		t.Errorf("received event = %d, want %d (EventUnknown)", received, EventUnknown)
	}
}

func TestLifecycleEventString(t *testing.T) {
	t.Parallel()

	cases := []struct {
		event LifecycleEvent
		want  string
	}{
		{EventCircuitOpened, "circuit_opened"},
		{EventCircuitClosed, "circuit_closed"},
		{EventExtensionConnected, "extension_connected"},
		{EventExtensionDisconnected, "extension_disconnected"},
		{EventBufferEviction, "buffer_eviction"},
		{EventRateLimitTriggered, "rate_limit_triggered"},
		{EventCommandStateDesync, "command_state_desync"},
		{EventSyncSnapshot, "sync_snapshot"},
		{EventUnknown, "unknown"},
		{LifecycleEvent(999), "unknown"},
	}

	for _, tc := range cases {
		if got := tc.event.String(); got != tc.want {
			t.Errorf("LifecycleEvent(%d).String() = %q, want %q", tc.event, got, tc.want)
		}
	}
}

func TestParseLifecycleEvent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  LifecycleEvent
	}{
		{"circuit_opened", EventCircuitOpened},
		{"circuit_closed", EventCircuitClosed},
		{"extension_connected", EventExtensionConnected},
		{"extension_disconnected", EventExtensionDisconnected},
		{"buffer_eviction", EventBufferEviction},
		{"rate_limit_triggered", EventRateLimitTriggered},
		{"command_state_desync", EventCommandStateDesync},
		{"sync_snapshot", EventSyncSnapshot},
		{"bogus_event", EventUnknown},
	}

	for _, tc := range cases {
		if got := ParseLifecycleEvent(tc.input); got != tc.want {
			t.Errorf("ParseLifecycleEvent(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}
