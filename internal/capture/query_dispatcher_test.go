// query_dispatcher_test.go — Tests for QueryDispatcher init, pending queries, results, and waiting.
package capture

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// ============================================
// NewQueryDispatcher Tests
// ============================================

func TestNewNewQueryDispatcher_Initialization(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	if qd.pendingQueries == nil {
		t.Fatal("pendingQueries should be initialized")
	}
	if len(qd.pendingQueries) != 0 {
		t.Errorf("pendingQueries len = %d, want 0", len(qd.pendingQueries))
	}
	if qd.queryResults == nil {
		t.Fatal("queryResults should be initialized")
	}
	if len(qd.queryResults) != 0 {
		t.Errorf("queryResults len = %d, want 0", len(qd.queryResults))
	}
	if qd.queryTimeout != queries.DefaultQueryTimeout {
		t.Errorf("queryTimeout = %v, want %v", qd.queryTimeout, queries.DefaultQueryTimeout)
	}
	if qd.completedResults == nil {
		t.Fatal("completedResults should be initialized")
	}
	if qd.failedCommands == nil {
		t.Fatal("failedCommands should be initialized")
	}
	if qd.commandNotify == nil {
		t.Fatal("commandNotify channel should be initialized")
	}
	if qd.queryCond == nil {
		t.Fatal("queryCond should be initialized")
	}
	if qd.queryIDCounter != 0 {
		t.Errorf("queryIDCounter = %d, want 0", qd.queryIDCounter)
	}
}

func TestNewQueryDispatcher_Close(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	qd.Close()
	// Should be safe to call multiple times
	qd.Close()
}

// ============================================
// GetSnapshot Tests
// ============================================

func TestNewQueryDispatcher_GetSnapshot_Empty(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	snap := qd.GetSnapshot()
	if snap.PendingQueryCount != 0 {
		t.Errorf("PendingQueryCount = %d, want 0", snap.PendingQueryCount)
	}
	if snap.QueryResultCount != 0 {
		t.Errorf("QueryResultCount = %d, want 0", snap.QueryResultCount)
	}
	if snap.QueryTimeout != queries.DefaultQueryTimeout {
		t.Errorf("QueryTimeout = %v, want %v", snap.QueryTimeout, queries.DefaultQueryTimeout)
	}
}

func TestNewQueryDispatcher_GetSnapshot_WithPending(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	qd.CreatePendingQuery(queries.PendingQuery{Type: "a11y", Params: json.RawMessage(`{}`)})

	snap := qd.GetSnapshot()
	if snap.PendingQueryCount != 2 {
		t.Errorf("PendingQueryCount = %d, want 2", snap.PendingQueryCount)
	}
}

// ============================================
// CreatePendingQuery Tests
// ============================================

func TestNewQueryDispatcher_CreatePendingQuery(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	if id == "" {
		t.Fatal("CreatePendingQuery returned empty id")
	}
	if !strings.HasPrefix(id, "q-") {
		t.Errorf("id = %q, want prefix q-", id)
	}

	pending := qd.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("pending len = %d, want 1", len(pending))
	}
	if pending[0].ID != id {
		t.Errorf("pending[0].ID = %q, want %q", pending[0].ID, id)
	}
	if pending[0].Type != "dom" {
		t.Errorf("pending[0].Type = %q, want dom", pending[0].Type)
	}
	if string(pending[0].Params) != `{"selector":"body"}` {
		t.Errorf("pending[0].Params = %s, want {\"selector\":\"body\"}", pending[0].Params)
	}
}

func TestNewQueryDispatcher_CreatePendingQueryWithClient(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQueryWithClient(queries.PendingQuery{
		Type:   "a11y",
		Params: json.RawMessage(`{}`),
	}, "client-1")

	if id == "" {
		t.Fatal("CreatePendingQueryWithClient returned empty id")
	}

	clientPending := qd.GetPendingQueriesForClient("client-1")
	if len(clientPending) != 1 {
		t.Fatalf("client pending len = %d, want 1", len(clientPending))
	}

	otherClient := qd.GetPendingQueriesForClient("client-2")
	if len(otherClient) != 0 {
		t.Fatalf("other client pending len = %d, want 0", len(otherClient))
	}
}

func TestNewQueryDispatcher_CreatePendingQueryWithTimeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQueryWithTimeout(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{}`),
		TabID:  42,
	}, 5*time.Second, "client-x")

	if id == "" {
		t.Fatal("CreatePendingQueryWithTimeout returned empty id")
	}

	clientPending := qd.GetPendingQueriesForClient("client-x")
	if len(clientPending) != 1 {
		t.Fatalf("client pending len = %d, want 1", len(clientPending))
	}
	if clientPending[0].TabID != 42 {
		t.Errorf("TabID = %d, want 42", clientPending[0].TabID)
	}
}

func TestNewQueryDispatcher_CreatePendingQuery_WithCorrelationID(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	qd.CreatePendingQuery(queries.PendingQuery{
		Type:          "execute_js",
		Params:        json.RawMessage(`{"script":"document.title"}`),
		CorrelationID: "corr-123",
	})

	cmd, found := qd.GetCommandResult("corr-123")
	if !found {
		t.Fatal("command not registered for correlation ID")
	}
	if cmd.Status != "pending" {
		t.Errorf("command status = %q, want pending", cmd.Status)
	}
	if cmd.CorrelationID != "corr-123" {
		t.Errorf("CorrelationID = %q, want corr-123", cmd.CorrelationID)
	}
}

func TestNewQueryDispatcher_CreatePendingQuery_Overflow(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	for i := 0; i < maxPendingQueries+1; i++ {
		qd.CreatePendingQuery(queries.PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{}`),
		})
	}

	pending := qd.GetPendingQueries()
	if len(pending) != maxPendingQueries {
		t.Fatalf("pending len = %d, want %d (max)", len(pending), maxPendingQueries)
	}
}

func TestNewQueryDispatcher_CreatePendingQuery_OverflowExpiresDroppedCommand(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	const droppedCorrelationID = "corr-overflow-drop"
	qd.CreatePendingQuery(queries.PendingQuery{
		Type:          "dom",
		Params:        json.RawMessage(`{}`),
		CorrelationID: droppedCorrelationID,
	})

	for i := 0; i < maxPendingQueries; i++ {
		qd.CreatePendingQuery(queries.PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{}`),
		})
	}

	cmd, found := qd.GetCommandResult(droppedCorrelationID)
	if !found {
		t.Fatalf("expected dropped command %q to remain observable", droppedCorrelationID)
	}
	if cmd.Status != "expired" {
		t.Fatalf("status = %q, want expired after overflow drop", cmd.Status)
	}
	if !strings.Contains(cmd.Error, "queue overflow") {
		t.Fatalf("error = %q, want queue overflow reason", cmd.Error)
	}
}

func TestNewQueryDispatcher_UniqueIDs(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	ids := make(map[string]bool)
	for i := 0; i < 10; i++ {
		id := qd.CreatePendingQuery(queries.PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{}`),
		})
		if ids[id] {
			t.Fatalf("duplicate query ID: %q", id)
		}
		ids[id] = true
	}
}

// ============================================
// SetQueryResult / GetQueryResult Tests
// ============================================

func TestNewQueryDispatcher_SetAndGetResult(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQuery(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"body"}`),
	})

	resultData := json.RawMessage(`{"html":"<body>test</body>"}`)
	qd.SetQueryResult(id, resultData)

	got, found := qd.GetQueryResult(id)
	if !found {
		t.Fatal("GetQueryResult returned false, want true")
	}
	if string(got) != string(resultData) {
		t.Errorf("result = %s, want %s", string(got), string(resultData))
	}

	// Second get should return not found (one-time use)
	_, found2 := qd.GetQueryResult(id)
	if found2 {
		t.Error("second GetQueryResult should return false (one-time use)")
	}
}

func TestNewQueryDispatcher_SetResultRemovesFromPending(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	qd.SetQueryResult(id, json.RawMessage(`{"ok":true}`))

	pending := qd.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("pending queries after SetQueryResult = %d, want 0", len(pending))
	}
}

func TestNewQueryDispatcher_GetResultForClient_Isolation(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQueryWithClient(queries.PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{}`),
	}, "client-A")
	qd.SetQueryResultWithClient(id, json.RawMessage(`{"found":true}`), "client-A")

	// Client B should NOT get Client A's result
	_, foundB := qd.GetQueryResultForClient(id, "client-B")
	if foundB {
		t.Error("client-B should not be able to access client-A's result")
	}

	// Client A should get it
	_, foundA := qd.GetQueryResultForClient(id, "client-A")
	if !foundA {
		t.Error("client-A should be able to access its own result")
	}
}

func TestNewQueryDispatcher_GetResult_NotFound(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	_, found := qd.GetQueryResult("nonexistent")
	if found {
		t.Error("GetQueryResult for nonexistent id should return false")
	}
}

// ============================================
// WaitForResult Tests
// ============================================

func TestNewQueryDispatcher_WaitForResult_Immediate(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})
	qd.SetQueryResult(id, json.RawMessage(`{"immediate":true}`))

	result, err := qd.WaitForResult(id, 1*time.Second)
	if err != nil {
		t.Fatalf("WaitForResult error = %v", err)
	}
	if string(result) != `{"immediate":true}` {
		t.Errorf("result = %s, want {\"immediate\":true}", string(result))
	}
}

func TestNewQueryDispatcher_WaitForResult_Timeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	_, err := qd.WaitForResult(id, 50*time.Millisecond)
	if err == nil {
		t.Fatal("WaitForResult should timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error = %q, want timeout message", err.Error())
	}
}

func TestNewQueryDispatcher_WaitForResult_Async(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	id := qd.CreatePendingQuery(queries.PendingQuery{Type: "dom", Params: json.RawMessage(`{}`)})

	go func() {
		time.Sleep(20 * time.Millisecond)
		qd.SetQueryResult(id, json.RawMessage(`{"async":true}`))
	}()

	result, err := qd.WaitForResult(id, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForResult error = %v", err)
	}
	if string(result) != `{"async":true}` {
		t.Errorf("result = %s, want {\"async\":true}", string(result))
	}
}

// ============================================
// SetQueryTimeout / GetQueryTimeout Tests
// ============================================

func TestNewQueryDispatcher_SetGetQueryTimeout(t *testing.T) {
	t.Parallel()

	qd := NewQueryDispatcher()
	defer qd.Close()

	if got := qd.GetQueryTimeout(); got != queries.DefaultQueryTimeout {
		t.Errorf("default timeout = %v, want %v", got, queries.DefaultQueryTimeout)
	}

	qd.SetQueryTimeout(10 * time.Second)
	if got := qd.GetQueryTimeout(); got != 10*time.Second {
		t.Errorf("timeout after set = %v, want 10s", got)
	}
}
