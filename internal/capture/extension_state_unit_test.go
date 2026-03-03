// Purpose: Unit tests for capture pipeline extension state logic.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import (
	"context"
	"testing"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"
)

func TestExtensionStateGettersAndBoundaries(t *testing.T) {
	t.Parallel()

	c := NewCapture()

	now := time.Now().UTC()
	c.mu.Lock()
	c.extensionState.trackingEnabled = true
	c.extensionState.trackedTabID = 42
	c.extensionState.trackedTabURL = "https://example.test/page"
	c.extensionState.trackedTabTitle = "Example"
	c.extensionState.pilotEnabled = true
	c.extensionState.lastPollAt = now
	c.extensionState.lastSyncSeen = now
	c.extensionState.extensionVersion = "9.9.9"
	c.extensionState.extSessionID = "session-a"
	c.extensionState.extSessionChangedAt = now
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
	c.extensionState.lastSyncSeen = time.Now().Add(-11 * time.Second) // Beyond extensionDisconnectThreshold (10s)
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
	if snap.ExtSessionID != "session-a" || !snap.PilotEnabled || snap.ActiveTestIDCount != 1 {
		t.Fatalf("extension snapshot = %+v, unexpected values", snap)
	}
}

// ============================================
// WaitForExtensionConnected tests (issue #302)
// ============================================

func TestWaitForExtensionConnected_AlreadyConnected(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Simulate extension already connected.
	c.mu.Lock()
	c.extensionState.lastSyncSeen = time.Now()
	c.mu.Unlock()

	if !c.WaitForExtensionConnected(context.Background(), 5*time.Second) {
		t.Fatal("WaitForExtensionConnected returned false when extension already connected")
	}
}

func TestWaitForExtensionConnected_ConnectsDuringWait(t *testing.T) {
	t.Parallel()
	c := NewCapture()

	// Connect at 50ms — well before the first 200ms poll tick, giving a comfortable
	// 150ms margin before the tick catches the connection.
	go func() {
		time.Sleep(50 * time.Millisecond)
		c.mu.Lock()
		c.extensionState.lastSyncSeen = time.Now()
		c.mu.Unlock()
	}()

	if !c.WaitForExtensionConnected(context.Background(), time.Second) {
		t.Fatal("WaitForExtensionConnected returned false; expected true after late connection")
	}
}

func TestWaitForExtensionConnected_Timeout(t *testing.T) {
	t.Parallel()
	c := NewCapture()
	// Extension never connects.

	if c.WaitForExtensionConnected(context.Background(), 100*time.Millisecond) {
		t.Fatal("WaitForExtensionConnected returned true; expected false after timeout")
	}
}

// TestWaitForExtensionConnected_ContextCancelled lives in readiness_gate_test.go
// with full timing bounds checks (P1-1).

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
