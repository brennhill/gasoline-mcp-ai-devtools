// query_commands_test.go — Tests for command lifecycle, expiration, delegation, and disconnect detection.
package capture

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// ============================================
// Command Lifecycle Tests
// ============================================

func TestNewQueryDispatcher_RegisterCommand(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-1", "q-1", 30*time.Second)

	cmd, found := qd.GetCommandResult("corr-1")
	if !found {
		t.Fatal("GetCommandResult returned false for registered command")
	}
	if cmd.CorrelationID != "corr-1" {
		t.Errorf("CorrelationID = %q, want corr-1", cmd.CorrelationID)
	}
	if cmd.Status != "pending" {
		t.Errorf("Status = %q, want pending", cmd.Status)
	}
	if cmd.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestNewQueryDispatcher_RegisterCommand_EmptyCorrelationID(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("", "q-1", 30*time.Second)

	_, found := qd.GetCommandResult("")
	if found {
		t.Error("empty correlation ID should not register a command")
	}
}

func TestNewQueryDispatcher_CompleteCommand(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-2", "q-2", 30*time.Second)

	resultData := json.RawMessage(`{"title":"Page Title"}`)
	qd.CompleteCommand("corr-2", resultData, "")

	cmd, found := qd.GetCommandResult("corr-2")
	if !found {
		t.Fatal("GetCommandResult returned false for completed command")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
	if string(cmd.Result) != `{"title":"Page Title"}` {
		t.Errorf("Result = %s, want {\"title\":\"Page Title\"}", string(cmd.Result))
	}
	if cmd.Error != "" {
		t.Errorf("Error = %q, want empty", cmd.Error)
	}
	if cmd.CompletedAt.IsZero() {
		t.Error("CompletedAt should be set")
	}
}

func TestNewQueryDispatcher_CompleteCommand_WithError(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-err", "q-err", 30*time.Second)
	qd.CompleteCommand("corr-err", nil, "element not found")

	cmd, found := qd.GetCommandResult("corr-err")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
	if cmd.Error != "element not found" {
		t.Errorf("Error = %q, want 'element not found'", cmd.Error)
	}
}

func TestNewQueryDispatcher_CompleteCommandWithStatus_Error(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-status-err", "q-status-err", 30*time.Second)
	qd.CompleteCommandWithStatus("corr-status-err", json.RawMessage(`{"ok":false}`), "error", "element not found")

	cmd, found := qd.GetCommandResult("corr-status-err")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "error" {
		t.Errorf("Status = %q, want error", cmd.Status)
	}
	if cmd.Error != "element not found" {
		t.Errorf("Error = %q, want 'element not found'", cmd.Error)
	}
}

func TestNewQueryDispatcher_CompleteCommandWithStatus_Timeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-status-timeout", "q-status-timeout", 30*time.Second)
	qd.CompleteCommandWithStatus("corr-status-timeout", nil, "timeout", "execution timed out")

	cmd, found := qd.GetCommandResult("corr-status-timeout")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "timeout" {
		t.Errorf("Status = %q, want timeout", cmd.Status)
	}
	if cmd.Error != "execution timed out" {
		t.Errorf("Error = %q, want 'execution timed out'", cmd.Error)
	}
}

func TestNewQueryDispatcher_CompleteCommand_EmptyCorrelationID(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.CompleteCommand("", json.RawMessage(`{}`), "")
}

func TestNewQueryDispatcher_CompleteCommand_NotRegistered(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.CompleteCommand("nonexistent", json.RawMessage(`{}`), "")
}

func TestNewQueryDispatcher_ExpireCommand(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-exp", "q-exp", 30*time.Second)
	qd.ExpireCommand("corr-exp")

	failed := qd.GetFailedCommands()
	found := false
	for _, cmd := range failed {
		if cmd.CorrelationID == "corr-exp" {
			found = true
			if cmd.Status != "expired" {
				t.Errorf("Status = %q, want expired", cmd.Status)
			}
			if !strings.Contains(cmd.Error, "expired") {
				t.Errorf("Error = %q, want 'expired' message", cmd.Error)
			}
		}
	}
	if !found {
		t.Error("expired command not found in GetFailedCommands()")
	}
}

func TestNewQueryDispatcher_ExpireCommand_EmptyID(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.ExpireCommand("")
}

func TestNewQueryDispatcher_ExpireCommand_AlreadyCompleted(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-done", "q-done", 30*time.Second)
	qd.CompleteCommand("corr-done", json.RawMessage(`{"ok":true}`), "")

	qd.ExpireCommand("corr-done")

	cmd, found := qd.GetCommandResult("corr-done")
	if !found {
		t.Fatal("completed command should still be findable")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete (should not be overwritten to expired)", cmd.Status)
	}
}

// ============================================
// WaitForCommand Tests
// ============================================

func TestNewQueryDispatcher_WaitForCommand_Immediate(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-wait", "q-wait", 30*time.Second)
	qd.CompleteCommand("corr-wait", json.RawMessage(`{"done":true}`), "")

	cmd, found := qd.WaitForCommand("corr-wait", 1*time.Second)
	if !found {
		t.Fatal("WaitForCommand returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
}

func TestNewQueryDispatcher_WaitForCommand_Async(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-async", "q-async", 30*time.Second)

	go func() {
		time.Sleep(20 * time.Millisecond)
		qd.CompleteCommand("corr-async", json.RawMessage(`{"async":true}`), "")
	}()

	cmd, found := qd.WaitForCommand("corr-async", 2*time.Second)
	if !found {
		t.Fatal("WaitForCommand returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
}

func TestNewQueryDispatcher_WaitForCommand_NotFound(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	cmd, found := qd.WaitForCommand("nonexistent", 50*time.Millisecond)
	if found {
		t.Errorf("WaitForCommand found = true, want false for nonexistent; cmd = %+v", cmd)
	}
}

func TestNewQueryDispatcher_WaitForCommandForClient_Isolation(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommandForClient("corr-client-wait", "q-client-wait", 30*time.Second, "client-a")

	cmd, found := qd.WaitForCommandForClient("corr-client-wait", 100*time.Millisecond, "client-b")
	if found || cmd != nil {
		t.Fatalf("client-b should not observe client-a command; found=%v cmd=%+v", found, cmd)
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		qd.CompleteCommand("corr-client-wait", json.RawMessage(`{"ok":true}`), "")
	}()

	cmd, found = qd.WaitForCommandForClient("corr-client-wait", time.Second, "client-a")
	if !found {
		t.Fatal("client-a should observe its command completion")
	}
	if cmd.Status != "complete" {
		t.Fatalf("status = %q, want complete", cmd.Status)
	}
}

// ============================================
// GetPendingCommands / GetCompletedCommands / GetFailedCommands Tests
// ============================================

func TestNewQueryDispatcher_GetPendingCommands(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-p1", "q-p1", 30*time.Second)
	qd.RegisterCommand("corr-p2", "q-p2", 30*time.Second)

	pending := qd.GetPendingCommands()
	if len(pending) != 2 {
		t.Fatalf("GetPendingCommands len = %d, want 2", len(pending))
	}
}

func TestNewQueryDispatcher_GetCompletedCommands(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-c1", "q-c1", 30*time.Second)
	qd.CompleteCommand("corr-c1", json.RawMessage(`{}`), "")

	qd.RegisterCommand("corr-c2", "q-c2", 30*time.Second)

	completed := qd.GetCompletedCommands()
	if len(completed) != 1 {
		t.Fatalf("GetCompletedCommands len = %d, want 1", len(completed))
	}
	if completed[0].CorrelationID != "corr-c1" {
		t.Errorf("completed[0].CorrelationID = %q, want corr-c1", completed[0].CorrelationID)
	}
}

func TestNewQueryDispatcher_GetFailedCommands_Empty(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	failed := qd.GetFailedCommands()
	if len(failed) != 0 {
		t.Errorf("GetFailedCommands len = %d, want 0", len(failed))
	}
}

// ============================================
// ExpireAllPendingQueries Tests
// ============================================

func TestNewQueryDispatcher_ExpireAllPendingQueries(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.CreatePendingQuery(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-exp-1",
	})
	qd.CreatePendingQuery(queries.PendingQuery{
		Type:          "a11y",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-exp-2",
	})

	qd.ExpireAllPendingQueries("extension_disconnected")

	pending := qd.GetPendingQueries()
	if len(pending) != 0 {
		t.Fatalf("pending after ExpireAll = %d, want 0", len(pending))
	}

	failed := qd.GetFailedCommands()
	correlationIDs := make(map[string]bool)
	for _, cmd := range failed {
		correlationIDs[cmd.CorrelationID] = true
		if cmd.Status != "expired" {
			t.Errorf("cmd %s status = %q, want expired", cmd.CorrelationID, cmd.Status)
		}
		if cmd.Error != "extension_disconnected" {
			t.Errorf("cmd %s error = %q, want extension_disconnected", cmd.CorrelationID, cmd.Error)
		}
	}
	if !correlationIDs["corr-exp-1"] {
		t.Error("corr-exp-1 missing from failed commands")
	}
	if !correlationIDs["corr-exp-2"] {
		t.Error("corr-exp-2 missing from failed commands")
	}
}

// ============================================
// Capture Delegation Tests (query_dispatcher.go)
// ============================================

func TestNewCaptureDelegation_QueryDispatcher(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	id := c.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	if id == "" {
		t.Fatal("CreatePendingQuery returned empty id")
	}

	pending := c.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("GetPendingQueries len = %d, want 1", len(pending))
	}

	c.SetQueryResult(id, json.RawMessage(`{"ok":true}`))
	result, found := c.GetQueryResult(id)
	if !found {
		t.Fatal("GetQueryResult returned false")
	}
	if string(result) != `{"ok":true}` {
		t.Errorf("result = %s, want {\"ok\":true}", string(result))
	}

	c.SetQueryTimeout(5 * time.Second)
	if got := c.GetQueryTimeout(); got != 5*time.Second {
		t.Errorf("GetQueryTimeout = %v, want 5s", got)
	}

	c.RegisterCommand("c-1", "q-1", 30*time.Second)
	c.CompleteCommand("c-1", json.RawMessage(`{"done":true}`), "")
	cmd, cmdFound := c.GetCommandResult("c-1")
	if !cmdFound {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("cmd.Status = %q, want complete", cmd.Status)
	}

	c.RegisterCommand("c-2", "q-2", 30*time.Second)
	c.ExpireCommand("c-2")

	_ = c.GetPendingCommands()
	_ = c.GetCompletedCommands()
	failed := c.GetFailedCommands()
	if len(failed) == 0 {
		t.Error("GetFailedCommands should contain expired command")
	}
}

// ============================================
// GetPendingQueriesDisconnectAware Tests
// ============================================

func TestNewCapture_GetPendingQueriesDisconnectAware_NeverSynced(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	pending := c.GetPendingQueriesDisconnectAware()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1 (never synced = not disconnected)", len(pending))
	}
}

func TestNewCapture_GetPendingQueriesDisconnectAware_RecentSync(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.mu.Lock()
	c.ext.lastSyncSeen = time.Now()
	c.mu.Unlock()

	c.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	pending := c.GetPendingQueriesDisconnectAware()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1 (recently synced)", len(pending))
	}
}

func TestNewCapture_GetPendingQueriesDisconnectAware_Disconnected(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	c.mu.Lock()
	c.ext.lastSyncSeen = time.Now().Add(-20 * time.Second)
	c.mu.Unlock()

	c.CreatePendingQuery(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-disc",
	})

	pending := c.GetPendingQueriesDisconnectAware()
	if len(pending) != 0 {
		t.Fatalf("pending len = %d, want 0 (disconnected)", len(pending))
	}
}
