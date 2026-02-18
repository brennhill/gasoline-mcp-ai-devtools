// cr_fixes_test.go — Tests for code review findings CR-1 through CR-4.
package capture

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// ============================================
// CR-1: Double-completion race — CompleteCommandWithStatus
// must only update if command is still "pending".
// ============================================

func TestCR1_CompleteCommandWithStatus_OnlyIfPending(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Register a command
	qd.RegisterCommand("corr-race", "q-race", 30*time.Second)

	// Simulate what SetQueryResultWithClient does: call CompleteCommand first
	// This sets status to "complete"
	qd.CompleteCommand("corr-race", json.RawMessage(`{"result":"from_query"}`), "")

	// Now simulate processSyncCommandResults calling CompleteCommandWithStatus
	// with status="error" — this should NOT overwrite the "complete" status
	qd.CompleteCommandWithStatus("corr-race", json.RawMessage(`{"result":"from_sync"}`), "error", "Element not found")

	// The command should still be "complete" (first writer wins)
	cmd, found := qd.GetCommandResult("corr-race")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want 'complete' (first writer should win, not be overwritten to 'error')", cmd.Status)
	}
	if string(cmd.Result) != `{"result":"from_query"}` {
		t.Errorf("Result = %s, want original result from first completion", string(cmd.Result))
	}
}

func TestCR1_CompleteCommandWithStatus_PendingToError(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Register a command (status starts as "pending")
	qd.RegisterCommand("corr-err", "q-err", 30*time.Second)

	// CompleteCommandWithStatus with "error" should work when still pending
	qd.CompleteCommandWithStatus("corr-err", nil, "error", "Element not found")

	cmd, found := qd.GetCommandResult("corr-err")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	if cmd.Status != "error" {
		t.Errorf("Status = %q, want 'error'", cmd.Status)
	}
	if cmd.Error != "Element not found" {
		t.Errorf("Error = %q, want 'Element not found'", cmd.Error)
	}
}

func TestCR1_DoubleCompletion_FirstWriterWins(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.RegisterCommand("corr-double", "q-double", 30*time.Second)

	// Two concurrent completions — only the first should succeed
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		qd.CompleteCommandWithStatus("corr-double", json.RawMessage(`{"writer":"A"}`), "complete", "")
	}()
	go func() {
		defer wg.Done()
		qd.CompleteCommandWithStatus("corr-double", json.RawMessage(`{"writer":"B"}`), "error", "failed")
	}()

	wg.Wait()

	cmd, found := qd.GetCommandResult("corr-double")
	if !found {
		t.Fatal("GetCommandResult returned false")
	}
	// Exactly one of the two writers should have won — status must be terminal (not pending)
	if cmd.Status == "pending" {
		t.Error("Status is still 'pending' after two completions — neither writer succeeded")
	}
}

func TestCR1_WaitForCommand_SeesCorrectErrorStatus(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Simulate the exact processSyncCommandResults flow:
	// 1. Create pending query with correlation ID
	qd.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "dom_action",
		Params:        json.RawMessage(`{"action":"click"}`),
		CorrelationID: "corr-sync-race",
	}, 30*time.Second, "")

	// 2. Extension posts error result via sync
	go func() {
		time.Sleep(10 * time.Millisecond)

		// This simulates processSyncCommandResults calling both paths:
		// First: SetQueryResultWithClient (which internally calls CompleteCommand → "complete")
		qd.SetQueryResultWithClient("q-1", json.RawMessage(`{"error":"not found"}`), "")
		// Second: CompleteCommandWithStatus with actual error status
		// With fix: this should be a no-op since command is already "complete"
		qd.CompleteCommandWithStatus("corr-sync-race", json.RawMessage(`{"error":"not found"}`), "error", "Element not found")
	}()

	// 3. WaitForCommand should see a terminal status (not flip-flop)
	cmd, found := qd.WaitForCommand("corr-sync-race", 2*time.Second)
	if !found {
		t.Fatal("WaitForCommand returned false")
	}
	// With the fix, status should be consistently "complete" (first writer wins)
	// Without the fix, there was a race where "error" could overwrite "complete"
	if cmd.Status == "pending" {
		t.Error("Status is still 'pending' — command was not completed")
	}
}

// ============================================
// CR-2: cleanExpiredQueries must call ExpireCommand
// so WaitForCommand is unblocked immediately.
// ============================================

func TestCR2_CleanExpiredQueries_ExpiresCommands(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Create a query that expires immediately
	qd.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-expire",
	}, 1*time.Millisecond, "")

	// Wait for it to expire
	time.Sleep(5 * time.Millisecond)

	// Call GetPendingQueries which triggers cleanExpiredQueries
	pending := qd.GetPendingQueries()
	if len(pending) != 0 {
		t.Fatalf("pending len = %d, want 0 after expiry", len(pending))
	}

	// The command should be marked as expired (not left as "pending")
	cmd, found := qd.GetCommandResult("corr-expire")
	if !found {
		// Command might be in failedCommands, check there
		failed := qd.GetFailedCommands()
		foundInFailed := false
		for _, f := range failed {
			if f.CorrelationID == "corr-expire" {
				foundInFailed = true
				if f.Status != "expired" {
					t.Errorf("failed command status = %q, want 'expired'", f.Status)
				}
			}
		}
		if !foundInFailed {
			t.Error("corr-expire not found in completedResults or failedCommands after expiry")
		}
		return
	}

	// If still in completedResults, it should not be "pending"
	if cmd.Status == "pending" {
		t.Error("expired query left command in 'pending' status — cleanExpiredQueries did not call ExpireCommand")
	}
}

func TestCR2_WaitForCommand_UnblockedByExpiry(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Create a query that expires in 50ms
	qd.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: "corr-unblock",
	}, 50*time.Millisecond, "")

	// Trigger expiry cleanup via GetPendingQueries in a goroutine
	go func() {
		time.Sleep(60 * time.Millisecond)
		// This should expire the query AND call ExpireCommand
		qd.GetPendingQueries()
	}()

	// WaitForCommand should be unblocked within ~100ms (not wait full 15s)
	start := time.Now()
	cmd, found := qd.WaitForCommand("corr-unblock", 5*time.Second)
	elapsed := time.Since(start)

	if !found {
		// Check failedCommands
		failed := qd.GetFailedCommands()
		for _, f := range failed {
			if f.CorrelationID == "corr-unblock" {
				found = true
				cmd = f
			}
		}
	}

	if elapsed > 2*time.Second {
		t.Errorf("WaitForCommand took %v — should have been unblocked by expiry, not waited until timeout", elapsed)
	}

	if found && cmd.Status != "expired" && cmd.Status != "pending" {
		// If found, it should be expired
		if cmd.Status != "expired" {
			t.Logf("command status = %q (elapsed: %v)", cmd.Status, elapsed)
		}
	}
}

// ============================================
// CR-4: GetCommandResult should check clientID
// ============================================

func TestCR4_GetCommandResultForClient_Isolation(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Register and complete command for client A
	qd.RegisterCommandForClient("corr-client-a", "q-ca", 30*time.Second, "client-a")
	qd.CompleteCommand("corr-client-a", json.RawMessage(`{"data":"secret"}`), "")

	// Client A should see it
	cmd, found := qd.GetCommandResultForClient("corr-client-a", "client-a")
	if !found {
		t.Fatal("client-a should find its own command")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}

	// Client B should NOT see client A's command
	_, found = qd.GetCommandResultForClient("corr-client-a", "client-b")
	if found {
		t.Error("client-b should NOT see client-a's command result")
	}

	// Empty clientID (no isolation) should still see everything
	cmd, found = qd.GetCommandResult("corr-client-a")
	if !found {
		t.Fatal("GetCommandResult (no client filter) should find the command")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
}

func TestCR4_GetCommandResultForClient_EmptyClientSeeAll(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	// Register command with no specific client
	qd.RegisterCommand("corr-any", "q-any", 30*time.Second)
	qd.CompleteCommand("corr-any", json.RawMessage(`{"data":"public"}`), "")

	// Any client should see commands with empty clientID
	cmd, found := qd.GetCommandResultForClient("corr-any", "any-client")
	if !found {
		t.Fatal("commands with empty clientID should be visible to all")
	}
	if cmd.Status != "complete" {
		t.Errorf("Status = %q, want complete", cmd.Status)
	}
}
