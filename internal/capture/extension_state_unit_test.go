package capture

import (
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

func TestExtensionStateGettersAndBoundaries(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	now := time.Now().UTC()
	c.mu.Lock()
	c.ext.trackingEnabled = true
	c.ext.trackedTabID = 42
	c.ext.trackedTabURL = "https://example.test/page"
	c.ext.trackedTabTitle = "Example"
	c.ext.pilotEnabled = true
	c.ext.lastPollAt = now
	c.ext.lastSyncSeen = now
	c.ext.extensionVersion = "9.9.9"
	c.ext.extensionSession = "session-a"
	c.ext.sessionChangedAt = now
	c.mu.Unlock()

	enabled, tabID, tabURL := c.GetTrackingStatus()
	if !enabled || tabID != 42 || tabURL != "https://example.test/page" {
		t.Fatalf("GetTrackingStatus() = (%v,%d,%q), want (true,42,https://example.test/page)", enabled, tabID, tabURL)
	}
	if got := c.GetTrackedTabTitle(); got != "Example" {
		t.Fatalf("GetTrackedTabTitle() = %q, want Example", got)
	}
	if !c.IsPilotEnabled() {
		t.Fatal("IsPilotEnabled() = false, want true")
	}
	if got := c.GetExtensionVersion(); got != "9.9.9" {
		t.Fatalf("GetExtensionVersion() = %q, want 9.9.9", got)
	}

	pilotStatus, ok := c.GetPilotStatus().(map[string]any)
	if !ok {
		t.Fatalf("GetPilotStatus() type = %T, want map[string]any", c.GetPilotStatus())
	}
	if pilotStatus["enabled"] != true {
		t.Fatalf("pilot status enabled = %v, want true", pilotStatus["enabled"])
	}
	if pilotStatus["extension_connected"] != true {
		t.Fatalf("extension_connected = %v, want true", pilotStatus["extension_connected"])
	}

	c.mu.Lock()
	c.ext.lastSyncSeen = time.Now().Add(-11 * time.Second) // Beyond extensionDisconnectThreshold (10s)
	c.mu.Unlock()
	staleStatus := c.GetPilotStatus().(map[string]any)
	if staleStatus["extension_connected"] != false {
		t.Fatalf("stale extension_connected = %v, want false", staleStatus["extension_connected"])
	}

	c.SetTestBoundaryStart("flow-1")
	c.SetTestBoundaryStart("flow-2")
	ids := c.GetActiveTestIDs()
	if len(ids) != 2 {
		t.Fatalf("active test ids len = %d, want 2", len(ids))
	}
	c.SetTestBoundaryEnd("flow-1")
	ids = c.GetActiveTestIDs()
	if len(ids) != 1 || ids[0] != "flow-2" {
		t.Fatalf("active test ids after end = %v, want [flow-2]", ids)
	}

	c.mu.RLock()
	snap := c.getExtensionSnapshot()
	c.mu.RUnlock()
	if snap.ExtensionSession != "session-a" || !snap.PilotEnabled || snap.ActiveTestIDCount != 1 {
		t.Fatalf("extension snapshot = %+v, unexpected values", snap)
	}
}

func TestCaptureTestHelpersAndTTL(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	c.AddNetworkBodiesForTest([]NetworkBody{
		{URL: "https://example.test/a", Status: 200},
		{URL: "https://example.test/b", Status: 500},
	})
	c.AddWebSocketEventsForTest([]WebSocketEvent{
		{Event: "open", URL: "wss://example.test"},
	})
	c.AddEnhancedActionsForTest([]EnhancedAction{
		{Type: "click", URL: "https://example.test", Timestamp: 123},
	})
	if got := c.GetNetworkTotalAdded(); got != 2 {
		t.Fatalf("GetNetworkTotalAdded() = %d, want 2", got)
	}
	if got := c.GetWebSocketTotalAdded(); got != 1 {
		t.Fatalf("GetWebSocketTotalAdded() = %d, want 1", got)
	}
	if got := c.GetActionTotalAdded(); got != 1 {
		t.Fatalf("GetActionTotalAdded() = %d, want 1", got)
	}

	c.SetPilotEnabled(true)
	if !c.IsPilotEnabled() {
		t.Fatal("SetPilotEnabled(true) did not update state")
	}
	c.SetTrackingStatusForTest(77, "https://tracked.test")
	enabled, tabID, tabURL := c.GetTrackingStatus()
	if !enabled || tabID != 77 || tabURL != "https://tracked.test" {
		t.Fatalf("tracking state = (%v,%d,%q), want (true,77,https://tracked.test)", enabled, tabID, tabURL)
	}

	if q := c.GetLastPendingQuery(); q != nil {
		t.Fatalf("GetLastPendingQuery() = %+v, want nil before adding pending query", q)
	}
	c.CreatePendingQuery(queries.PendingQuery{Type: "query_dom", Params: []byte(`{"selector":".x"}`)})
	c.CreatePendingQuery(queries.PendingQuery{Type: "accessibility", Params: []byte(`{"scope":"page"}`)})
	last := c.GetLastPendingQuery()
	if last == nil || last.Type != "accessibility" {
		t.Fatalf("last pending query = %+v, want accessibility query", last)
	}

	if isExpiredByTTL(time.Now(), 0) {
		t.Fatal("isExpiredByTTL should return false when ttl=0")
	}
	if !isExpiredByTTL(time.Now().Add(-2*time.Second), time.Second) {
		t.Fatal("isExpiredByTTL should return true for expired entry")
	}
	if isExpiredByTTL(time.Now().Add(-200*time.Millisecond), time.Second) {
		t.Fatal("isExpiredByTTL should return false for recent entry")
	}
}
