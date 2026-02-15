// websocket_repair_test.go — Unit tests for repairWSParallelArrays.
//go:build !production

package capture

import (
	"testing"
)

func TestRepairWSParallelArrays_EqualLength(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	c.AddWebSocketEventsForTest([]WebSocketEvent{
		{Event: "message", Data: "hello", ID: "ws1"},
		{Event: "message", Data: "world", ID: "ws1"},
	})

	events, addedAt, _ := c.GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}

	// Trigger repair via AddWebSocketEvents (calls repairWSParallelArrays internally)
	c.AddWebSocketEvents([]WebSocketEvent{{Event: "message", Data: "new", ID: "ws1"}})

	events, addedAt, _ = c.GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("after add: expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
	if events != 3 {
		t.Fatalf("expected 3 events, got %d", events)
	}
}

func TestRepairWSParallelArrays_EventsLonger(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add 2 matched events, then 3 extra wsEvents without matching wsAddedAt
	c.AddWebSocketEventsForTest([]WebSocketEvent{
		{Event: "message", Data: "matched1", ID: "ws1"},
		{Event: "message", Data: "matched2", ID: "ws1"},
	})
	c.SetWSParallelMismatchForTest(3, 0)

	events, addedAt, _ := c.GetWSLengthsForTest()
	if events == addedAt {
		t.Fatal("expected mismatch before repair")
	}
	if events != 5 || addedAt != 2 {
		t.Fatalf("expected events=5 addedAt=2, got events=%d addedAt=%d", events, addedAt)
	}

	// Trigger repair by adding a new event
	c.AddWebSocketEvents([]WebSocketEvent{{Event: "message", Data: "trigger", ID: "ws1"}})

	events, addedAt, _ = c.GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("after repair: expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
}

func TestRepairWSParallelArrays_AddedAtLonger(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add 2 matched events, then 3 extra wsAddedAt without matching wsEvents
	c.AddWebSocketEventsForTest([]WebSocketEvent{
		{Event: "message", Data: "matched1", ID: "ws1"},
		{Event: "message", Data: "matched2", ID: "ws1"},
	})
	c.SetWSParallelMismatchForTest(0, 3)

	events, addedAt, _ := c.GetWSLengthsForTest()
	if events == addedAt {
		t.Fatal("expected mismatch before repair")
	}
	if events != 2 || addedAt != 5 {
		t.Fatalf("expected events=2 addedAt=5, got events=%d addedAt=%d", events, addedAt)
	}

	// Trigger repair
	c.AddWebSocketEvents([]WebSocketEvent{{Event: "message", Data: "trigger", ID: "ws1"}})

	events, addedAt, _ = c.GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("after repair: expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
}

func TestRepairWSParallelArrays_MemoryRecalculated(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Add events with known data sizes
	c.AddWebSocketEventsForTest([]WebSocketEvent{
		{Event: "message", Data: "aaaa", ID: "ws1"},  // 4 bytes data
		{Event: "message", Data: "bbbbbb", ID: "ws1"}, // 6 bytes data
	})
	// Add extra unmatched event
	c.SetWSParallelMismatchForTest(1, 0)

	// Trigger repair
	c.AddWebSocketEvents([]WebSocketEvent{{Event: "message", Data: "cc", ID: "ws1"}})

	events, addedAt, mem := c.GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
	if mem <= 0 {
		t.Fatalf("expected positive memory total, got %d", mem)
	}
}

func TestRepairWSParallelArrays_BothEmpty(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Both arrays are empty — no-op repair
	events, addedAt, _ := c.GetWSLengthsForTest()
	if events != 0 || addedAt != 0 {
		t.Fatalf("expected both empty, got events=%d addedAt=%d", events, addedAt)
	}

	// Adding an event should work fine
	c.AddWebSocketEvents([]WebSocketEvent{{Event: "open", ID: "ws1"}})

	events, addedAt, _ = c.GetWSLengthsForTest()
	if events != addedAt {
		t.Fatalf("expected equal lengths, got events=%d addedAt=%d", events, addedAt)
	}
	if events != 1 {
		t.Fatalf("expected 1 event, got %d", events)
	}
}
