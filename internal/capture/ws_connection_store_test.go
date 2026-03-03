package capture

import "testing"

func TestWSConnectionTracker_StatusFiltersOpenAndClosed(t *testing.T) {
	tracker := WSConnectionTracker{
		connections: map[string]*connectionState{
			"conn-a": {id: "conn-a", url: "wss://chat.example/ws", state: "open", openedAt: "2026-03-03T09:00:00Z"},
			"conn-b": {id: "conn-b", url: "wss://prices.example/ws", state: "open", openedAt: "2026-03-03T09:00:01Z"},
		},
		closedConns: []WebSocketClosedConnection{
			{ID: "conn-c", URL: "wss://chat.example/ws", State: "closed"},
		},
		connOrder: []string{"conn-a", "conn-b"},
	}

	status := tracker.status(WebSocketStatusFilter{URLFilter: "chat"})
	if len(status.Connections) != 1 {
		t.Fatalf("connections len = %d, want 1", len(status.Connections))
	}
	if status.Connections[0].ID != "conn-a" {
		t.Fatalf("connection id = %q, want conn-a", status.Connections[0].ID)
	}
	if len(status.Closed) != 1 {
		t.Fatalf("closed len = %d, want 1", len(status.Closed))
	}
	if status.Closed[0].ID != "conn-c" {
		t.Fatalf("closed id = %q, want conn-c", status.Closed[0].ID)
	}
}

func TestWSConnectionTracker_ClearResetsState(t *testing.T) {
	tracker := WSConnectionTracker{
		connections: map[string]*connectionState{
			"conn-a": {id: "conn-a", url: "wss://chat.example/ws", state: "open"},
			"conn-b": {id: "conn-b", url: "wss://prices.example/ws", state: "open"},
		},
		closedConns: []WebSocketClosedConnection{
			{ID: "conn-c", URL: "wss://chat.example/ws", State: "closed"},
		},
		connOrder: []string{"conn-a", "conn-b"},
	}

	removed := tracker.clear()
	if removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	if tracker.connectionCount() != 0 {
		t.Fatalf("connection count = %d, want 0", tracker.connectionCount())
	}
	if len(tracker.closedConns) != 0 {
		t.Fatalf("closedConns len = %d, want 0", len(tracker.closedConns))
	}
	if len(tracker.connOrder) != 0 {
		t.Fatalf("connOrder len = %d, want 0", len(tracker.connOrder))
	}
}
