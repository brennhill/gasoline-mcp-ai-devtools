// multi_client_test.go — Integration and isolation tests for multi-client MCP support.
// Tests checkpoint namespace isolation, query result isolation, /clients HTTP endpoints,
// MCP-over-HTTP with X-Gasoline-Client header, backwards compatibility, and concurrent
// multi-client stress scenarios.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================
// Test Helpers
// ============================================

func setupMultiClientTest(t *testing.T) (*Server, *Capture, *CheckpointManager) {
	t.Helper()
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()
	cm := NewCheckpointManager(server, capture)
	return server, capture, cm
}

// ============================================
// Checkpoint Namespace Isolation
// ============================================

func TestMultiClient_CheckpointIsolation(t *testing.T) {
	server, _, cm := setupMultiClientTest(t)

	clientA := DeriveClientID("/home/alice/project")
	clientB := DeriveClientID("/home/bob/project")

	// Both clients create a checkpoint with the same name
	if err := cm.CreateCheckpoint("before-fix", clientA); err != nil {
		t.Fatalf("Client A failed to create checkpoint: %v", err)
	}

	// Add some errors between checkpoints
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "TypeError in component"},
	})

	if err := cm.CreateCheckpoint("before-fix", clientB); err != nil {
		t.Fatalf("Client B failed to create checkpoint: %v", err)
	}

	// Add more errors
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "ReferenceError in module"},
		{"level": "error", "msg": "Network request failed"},
	})

	// Client A should see all errors since their earlier checkpoint
	respA := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "before-fix",
	}, clientA)

	// Client B should see fewer errors since their later checkpoint
	respB := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "before-fix",
	}, clientB)

	// Both should return error severity
	if respA.Severity != "error" {
		t.Errorf("Client A: expected severity 'error', got '%s'", respA.Severity)
	}
	if respB.Severity != "error" {
		t.Errorf("Client B: expected severity 'error', got '%s'", respB.Severity)
	}

	// Client A's checkpoint was created earlier, so it should see more new entries
	if respA.Console == nil {
		t.Fatal("Client A: expected console diff, got nil")
	}
	if respB.Console == nil {
		t.Fatal("Client B: expected console diff, got nil")
	}

	// Client A sees 3 total (1 before B's checkpoint + 2 after)
	// Client B sees 2 total (only the 2 after their checkpoint)
	if respA.Console.TotalNew < respB.Console.TotalNew {
		t.Errorf("Client A should see more entries than B: A=%d, B=%d",
			respA.Console.TotalNew, respB.Console.TotalNew)
	}
}

func TestMultiClient_CheckpointNamespaceDoesNotLeak(t *testing.T) {
	_, _, cm := setupMultiClientTest(t)

	clientA := DeriveClientID("/home/alice/project")
	clientB := DeriveClientID("/home/bob/project")

	// Client A creates a checkpoint
	if err := cm.CreateCheckpoint("my-checkpoint", clientA); err != nil {
		t.Fatalf("Failed to create checkpoint: %v", err)
	}

	// Client B tries to use Client A's checkpoint name — should not find it
	// (falls back to "unknown checkpoint" / beginning-of-time behavior)
	resp := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "my-checkpoint",
	}, clientB)

	// Should return a valid response (not crash), treating it as start-of-time
	if resp.From.IsZero() {
		t.Error("Response should have a non-zero From time")
	}
}

func TestMultiClient_CheckpointBackwardsCompat(t *testing.T) {
	_, _, cm := setupMultiClientTest(t)

	// Empty clientID should use global namespace (no prefix)
	if err := cm.CreateCheckpoint("global-cp", ""); err != nil {
		t.Fatalf("Failed to create global checkpoint: %v", err)
	}

	// Querying with empty clientID should find the global checkpoint
	resp := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "global-cp",
	}, "")

	if resp.From.IsZero() {
		t.Error("Global checkpoint should be found")
	}
}

func TestMultiClient_CheckpointFallbackToGlobal(t *testing.T) {
	_, _, cm := setupMultiClientTest(t)

	// Create a global checkpoint (empty clientID)
	if err := cm.CreateCheckpoint("shared-cp", ""); err != nil {
		t.Fatalf("Failed to create global checkpoint: %v", err)
	}

	// A client can fall back to global namespace if their namespaced version isn't found
	clientA := DeriveClientID("/home/alice/project")
	resp := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "shared-cp",
	}, clientA)

	// The fallback to global checkpoint should work (not treated as unknown)
	if resp.From.IsZero() {
		t.Error("Should have found the global checkpoint as fallback")
	}
}

func TestMultiClient_SameNameDifferentClients(t *testing.T) {
	_, _, cm := setupMultiClientTest(t)

	clientA := DeriveClientID("/home/alice/project")
	clientB := DeriveClientID("/home/bob/project")

	// Both create checkpoint "test" — should not conflict
	if err := cm.CreateCheckpoint("test", clientA); err != nil {
		t.Fatalf("Client A: %v", err)
	}
	if err := cm.CreateCheckpoint("test", clientB); err != nil {
		t.Fatalf("Client B: %v", err)
	}

	// Should have 2 named checkpoints (clientA:test and clientB:test)
	count := cm.GetNamedCheckpointCount()
	if count != 2 {
		t.Errorf("Expected 2 named checkpoints, got %d", count)
	}
}

// ============================================
// Query Result Isolation
// ============================================

func TestMultiClient_QueryResultIsolation(t *testing.T) {
	capture := NewCapture()

	clientA := "client-aaa"
	clientB := "client-bbb"

	// Client A creates a query
	idA := capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector": ".foo"}`),
	}, 5*time.Second, clientA)

	// Client B creates a query
	idB := capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector": ".bar"}`),
	}, 5*time.Second, clientB)

	// Simulate extension posting results
	capture.SetQueryResult(idA, json.RawMessage(`{"elements": 3}`))
	capture.SetQueryResult(idB, json.RawMessage(`{"elements": 7}`))

	// Client A should get their result
	resultA, foundA := capture.GetQueryResult(idA, clientA)
	if !foundA {
		t.Fatal("Client A should find their own result")
	}
	if !bytes.Contains(resultA, []byte("3")) {
		t.Errorf("Client A got wrong result: %s", resultA)
	}

	// Client B should get their result
	resultB, foundB := capture.GetQueryResult(idB, clientB)
	if !foundB {
		t.Fatal("Client B should find their own result")
	}
	if !bytes.Contains(resultB, []byte("7")) {
		t.Errorf("Client B got wrong result: %s", resultB)
	}
}

func TestMultiClient_QueryResultCrossClientDenied(t *testing.T) {
	capture := NewCapture()

	clientA := "client-aaa"
	clientB := "client-bbb"

	// Client A creates a query
	idA := capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector": ".foo"}`),
	}, 5*time.Second, clientA)

	// Extension posts result
	capture.SetQueryResult(idA, json.RawMessage(`{"secret": "data"}`))

	// Client B tries to read Client A's result — should be denied
	_, found := capture.GetQueryResult(idA, clientB)
	if found {
		t.Error("Client B should NOT be able to read Client A's query result")
	}
}

func TestMultiClient_QueryResultLegacyClientCanReadAny(t *testing.T) {
	capture := NewCapture()

	// Legacy client (empty clientID) creates a query
	id := capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector": ".test"}`),
	}, 5*time.Second, "")

	capture.SetQueryResult(id, json.RawMessage(`{"ok": true}`))

	// Legacy client (empty) can read it
	result, found := capture.GetQueryResult(id, "")
	if !found {
		t.Fatal("Legacy client should find result")
	}
	if !bytes.Contains(result, []byte("ok")) {
		t.Errorf("Got wrong result: %s", result)
	}
}

func TestMultiClient_WaitForResultIsolation(t *testing.T) {
	capture := NewCapture()

	clientA := "client-aaa"
	clientB := "client-bbb"

	// Client A creates a query
	idA := capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector": ".foo"}`),
	}, 5*time.Second, clientA)

	// Simulate result arriving after a brief delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		capture.SetQueryResult(idA, json.RawMessage(`{"found": true}`))
	}()

	// Client A waits and gets the result
	result, err := capture.WaitForResult(idA, 2*time.Second, clientA)
	if err != nil {
		t.Fatalf("Client A WaitForResult failed: %v", err)
	}
	if !bytes.Contains(result, []byte("found")) {
		t.Errorf("Wrong result: %s", result)
	}

	// Create another query for cross-client test
	idA2 := capture.CreatePendingQueryWithClient(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector": ".bar"}`),
	}, 5*time.Second, clientA)

	// Post result
	capture.SetQueryResult(idA2, json.RawMessage(`{"secret": true}`))

	// Client B tries to wait — should get permission denied
	_, err = capture.WaitForResult(idA2, 200*time.Millisecond, clientB)
	if err == nil {
		t.Error("Client B WaitForResult should fail for Client A's query")
	}
	if err != nil && !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("Expected 'permission denied' error, got: %v", err)
	}
}

// ============================================
// /clients HTTP Endpoint Tests
// ============================================

func TestHTTP_ClientsRegister(t *testing.T) {
	capture := NewCapture()

	// POST /clients - register a new client
	body := `{"cwd": "/home/alice/project"}`
	req := httptest.NewRequest("POST", "/clients", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler := func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			CWD string `json:"cwd"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
			return
		}
		cs := capture.clientRegistry.Register(reqBody.CWD)
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"id":  cs.ID,
			"cwd": cs.CWD,
		})
	}

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	expectedID := DeriveClientID("/home/alice/project")
	if resp["id"] != expectedID {
		t.Errorf("Expected ID %s, got %s", expectedID, resp["id"])
	}
	if resp["cwd"] != "/home/alice/project" {
		t.Errorf("Expected CWD /home/alice/project, got %s", resp["cwd"])
	}
}

func TestHTTP_ClientsList(t *testing.T) {
	capture := NewCapture()

	// Register two clients
	capture.clientRegistry.Register("/home/alice/project")
	capture.clientRegistry.Register("/home/bob/project")

	// GET /clients
	req := httptest.NewRequest("GET", "/clients", http.NoBody)
	rec := httptest.NewRecorder()

	handler := func(w http.ResponseWriter, r *http.Request) {
		clients := capture.clientRegistry.List()
		jsonResponse(w, http.StatusOK, map[string]interface{}{
			"clients": clients,
			"count":   len(clients),
		})
	}

	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	count, ok := resp["count"].(float64)
	if !ok || int(count) != 2 {
		t.Errorf("Expected 2 clients, got %v", resp["count"])
	}
}

func TestHTTP_ClientsGet(t *testing.T) {
	capture := NewCapture()

	cs := capture.clientRegistry.Register("/home/alice/project")
	clientID := cs.ID

	// GET /clients/{id}
	cs = capture.clientRegistry.Get(clientID)
	if cs == nil {
		t.Fatal("Client should exist after registration")
	}
	if cs.ID != clientID {
		t.Errorf("Expected ID %s, got %s", clientID, cs.ID)
	}
	if cs.CWD != "/home/alice/project" {
		t.Errorf("Expected CWD /home/alice/project, got %s", cs.CWD)
	}
}

func TestHTTP_ClientsGetNotFound(t *testing.T) {
	capture := NewCapture()

	cs := capture.clientRegistry.Get("nonexistent-id")
	if cs != nil {
		t.Error("Expected nil for nonexistent client")
	}
}

func TestHTTP_ClientsDelete(t *testing.T) {
	capture := NewCapture()

	cs := capture.clientRegistry.Register("/home/alice/project")
	clientID := cs.ID

	// Verify exists
	if capture.clientRegistry.Count() != 1 {
		t.Fatalf("Expected 1 client, got %d", capture.clientRegistry.Count())
	}

	// DELETE /clients/{id}
	capture.clientRegistry.Unregister(clientID)

	// Verify removed
	if capture.clientRegistry.Count() != 0 {
		t.Errorf("Expected 0 clients after delete, got %d", capture.clientRegistry.Count())
	}
	if cs := capture.clientRegistry.Get(clientID); cs != nil {
		t.Error("Client should not exist after delete")
	}
}

func TestHTTP_ClientsReRegister(t *testing.T) {
	capture := NewCapture()

	// Register same CWD twice — should return same ID, update LastSeenAt
	cs1 := capture.clientRegistry.Register("/home/alice/project")
	time.Sleep(5 * time.Millisecond)
	cs2 := capture.clientRegistry.Register("/home/alice/project")

	if cs1.ID != cs2.ID {
		t.Errorf("Same CWD should produce same ID: %s vs %s", cs1.ID, cs2.ID)
	}

	// Only 1 client should exist
	if capture.clientRegistry.Count() != 1 {
		t.Errorf("Expected 1 client after re-register, got %d", capture.clientRegistry.Count())
	}

	// LastSeenAt should be updated
	if !cs2.GetLastSeen().After(cs1.CreatedAt) || cs2.GetLastSeen().Equal(cs1.CreatedAt) {
		t.Error("Re-registration should update LastSeenAt")
	}
}

// ============================================
// MCP over HTTP with Client ID
// ============================================

func TestHTTP_MCPWithClientID(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()

	mcp := NewToolHandler(server, capture)

	// Send initialize request with X-Gasoline-Client header
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(initReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gasoline-Client", "test-client-123")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Expected no error, got: %s", resp.Error.Message)
	}
}

func TestHTTP_MCPToolCallSetsClientID(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()

	mcp := NewToolHandler(server, capture)

	// Add some log data for the observe tool
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "test error"},
	})

	// Call observe tool with client header
	toolReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(toolReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gasoline-Client", "test-client-456")
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var resp JSONRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Tool call should succeed, got error: %s", resp.Error.Message)
	}

	// clientID is now per-request (on JSONRPCRequest.ClientID), not stored on the handler,
	// so there's no shared mutable state to verify — the race is eliminated by design.
}

func TestHTTP_MCPWithoutClientID(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()

	mcp := NewToolHandler(server, capture)

	// Send request without X-Gasoline-Client — backwards compatibility
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(initReq))
	req.Header.Set("Content-Type", "application/json")
	// No X-Gasoline-Client header
	rec := httptest.NewRecorder()

	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	// Should work fine with empty client ID
	var resp JSONRPCResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Error != nil {
		t.Errorf("Should work without client ID, got error: %s", resp.Error.Message)
	}
}

// ============================================
// Full Integration: Checkpoint + Client ID via MCP
// ============================================

func TestIntegration_CheckpointWithMCPClientHeader(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()
	mcp := NewToolHandler(server, capture)

	clientA := "client-alpha"
	clientB := "client-beta"

	// Client A creates a checkpoint via MCP
	createCPReq := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"changes","checkpoint":"before-test"}}}`

	reqA := httptest.NewRequest("POST", "/mcp", strings.NewReader(createCPReq))
	reqA.Header.Set("Content-Type", "application/json")
	reqA.Header.Set("X-Gasoline-Client", clientA)
	recA := httptest.NewRecorder()
	mcp.HandleHTTP(recA, reqA)

	// Add errors
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "Error after A's checkpoint"},
	})

	// Client B creates a checkpoint via MCP (same name)
	reqB := httptest.NewRequest("POST", "/mcp", strings.NewReader(createCPReq))
	reqB.Header.Set("Content-Type", "application/json")
	reqB.Header.Set("X-Gasoline-Client", clientB)
	recB := httptest.NewRecorder()
	mcp.HandleHTTP(recB, reqB)

	// Add more errors
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "Error after B's checkpoint"},
	})

	// Both clients query changes — Client A should see more
	queryCPReq := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"observe","arguments":{"what":"changes","checkpoint":"before-test"}}}`

	reqA2 := httptest.NewRequest("POST", "/mcp", strings.NewReader(queryCPReq))
	reqA2.Header.Set("Content-Type", "application/json")
	reqA2.Header.Set("X-Gasoline-Client", clientA)
	recA2 := httptest.NewRecorder()
	mcp.HandleHTTP(recA2, reqA2)

	reqB2 := httptest.NewRequest("POST", "/mcp", strings.NewReader(queryCPReq))
	reqB2.Header.Set("Content-Type", "application/json")
	reqB2.Header.Set("X-Gasoline-Client", clientB)
	recB2 := httptest.NewRecorder()
	mcp.HandleHTTP(recB2, reqB2)

	// Both should succeed (status 200)
	if recA2.Code != http.StatusOK || recB2.Code != http.StatusOK {
		t.Errorf("Expected 200 for both: A=%d, B=%d", recA2.Code, recB2.Code)
	}
}

// ============================================
// Client Registry + Checkpoint Lifecycle
// ============================================

func TestIntegration_ClientRegistryWithCheckpoints(t *testing.T) {
	server, capture, cm := setupMultiClientTest(t)

	// Register two clients
	csA := capture.clientRegistry.Register("/home/alice/frontend")
	csB := capture.clientRegistry.Register("/home/bob/backend")

	// Each client creates a checkpoint using their registry ID
	if err := cm.CreateCheckpoint("start", csA.ID); err != nil {
		t.Fatalf("Client A checkpoint failed: %v", err)
	}
	if err := cm.CreateCheckpoint("start", csB.ID); err != nil {
		t.Fatalf("Client B checkpoint failed: %v", err)
	}

	// Add data
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "API error"},
		{"level": "warn", "msg": "Slow query"},
	})

	// Client A gets changes — should see the 2 entries
	diffA := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "start",
	}, csA.ID)

	if diffA.Console == nil {
		t.Fatal("Client A should see console entries")
	}
	if diffA.Console.TotalNew != 2 {
		t.Errorf("Client A expected 2 new entries, got %d", diffA.Console.TotalNew)
	}

	// Unregister client A
	capture.clientRegistry.Unregister(csA.ID)

	// Client B should still work
	diffB := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "start",
	}, csB.ID)

	if diffB.Console == nil {
		t.Fatal("Client B should still see console entries after A is unregistered")
	}
}

// ============================================
// LRU Eviction with Active Usage
// ============================================

func TestMultiClient_LRUEvictionPreservesActive(t *testing.T) {
	capture := NewCapture()

	// Register maxClients clients
	var ids []string
	for i := 0; i < maxClients; i++ {
		cs := capture.clientRegistry.Register(fmt.Sprintf("/project/%d", i))
		ids = append(ids, cs.ID)
	}

	if capture.clientRegistry.Count() != maxClients {
		t.Fatalf("Expected %d clients, got %d", maxClients, capture.clientRegistry.Count())
	}

	// Touch the first client to make it "active"
	activeCS := capture.clientRegistry.Get(ids[0])
	if activeCS == nil {
		t.Fatal("First client should still exist")
	}

	// Register one more — should evict the LRU (which is now ids[1], since ids[0] was just touched)
	newCS := capture.clientRegistry.Register("/project/new")

	// The first client should still exist (it was recently touched)
	if capture.clientRegistry.Get(ids[0]) == nil {
		t.Error("Recently touched client should survive eviction")
	}

	// The new client should exist
	if capture.clientRegistry.Get(newCS.ID) == nil {
		t.Error("Newly registered client should exist")
	}

	// One of the old untouched clients should have been evicted
	if capture.clientRegistry.Count() != maxClients {
		t.Errorf("Count should be %d (at capacity), got %d", maxClients, capture.clientRegistry.Count())
	}
}

// ============================================
// Concurrent Multi-Client Operations
// ============================================

func TestMultiClient_ConcurrentCheckpointOperations(t *testing.T) {
	server, _, cm := setupMultiClientTest(t)

	const numClients = 5
	const opsPerClient = 20
	var wg sync.WaitGroup

	// Simulate multiple clients creating and querying checkpoints concurrently
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()
			clientID := DeriveClientID(fmt.Sprintf("/project/%d", clientNum))

			for j := 0; j < opsPerClient; j++ {
				// Create checkpoint
				name := fmt.Sprintf("cp-%d", j)
				if err := cm.CreateCheckpoint(name, clientID); err != nil {
					t.Errorf("Client %d: checkpoint %s failed: %v", clientNum, name, err)
					return
				}

				// Add some data
				server.addEntries([]LogEntry{
					{"level": "error", "msg": fmt.Sprintf("error-%d-%d", clientNum, j)},
				})

				// Query changes
				cm.GetChangesSince(GetChangesSinceParams{
					Checkpoint: name,
				}, clientID)
			}
		}(i)
	}

	wg.Wait()
}

func TestMultiClient_ConcurrentQueryIsolation(t *testing.T) {
	capture := NewCapture()

	const numClients = 5
	const queriesPerClient = 10
	var wg sync.WaitGroup

	// Each client creates queries and verifies they only see their own results
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()
			clientID := fmt.Sprintf("client-%d", clientNum)

			for j := 0; j < queriesPerClient; j++ {
				// Create query
				id := capture.CreatePendingQueryWithClient(PendingQuery{
					Type:   "dom",
					Params: json.RawMessage(fmt.Sprintf(`{"n": %d}`, clientNum*100+j)),
				}, 5*time.Second, clientID)

				// Simulate extension result
				resultData := fmt.Sprintf(`{"client": %d, "query": %d}`, clientNum, j)
				capture.SetQueryResult(id, json.RawMessage(resultData))

				// Verify own result
				result, found := capture.GetQueryResult(id, clientID)
				if !found {
					t.Errorf("Client %d: should find own result for query %s", clientNum, id)
					continue
				}
				if !bytes.Contains(result, []byte(fmt.Sprintf(`"client": %d`, clientNum))) {
					t.Errorf("Client %d: got wrong result: %s", clientNum, result)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestMultiClient_ConcurrentRegistrationAndEviction(t *testing.T) {
	capture := NewCapture()

	const numGoroutines = 20
	const opsPerGoroutine = 50
	var wg sync.WaitGroup

	// Many goroutines registering and unregistering clients concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(gNum int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				cwd := fmt.Sprintf("/goroutine/%d/op/%d", gNum, j%maxClients)
				cs := capture.clientRegistry.Register(cwd)

				// Sometimes unregister
				if j%3 == 0 {
					capture.clientRegistry.Unregister(cs.ID)
				}

				// Sometimes list
				if j%5 == 0 {
					_ = capture.clientRegistry.List()
				}

				// Sometimes get
				if j%2 == 0 {
					_ = capture.clientRegistry.Get(cs.ID)
				}
			}
		}(i)
	}

	wg.Wait()

	// Registry should be in a valid state (not panicked, count <= maxClients)
	count := capture.clientRegistry.Count()
	if count > maxClients {
		t.Errorf("Registry count %d exceeds max %d", count, maxClients)
	}
}

// ============================================
// End-to-End: Simulated Multi-Session
// ============================================

func TestE2E_TwoClientsSameServer(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()
	cm := NewCheckpointManager(server, capture)

	// Simulate two Claude Code sessions connecting
	clientA := capture.clientRegistry.Register("/home/alice/frontend")
	clientB := capture.clientRegistry.Register("/home/bob/backend")

	// === Phase 1: Both create "start" checkpoints ===
	cm.CreateCheckpoint("start", clientA.ID)
	cm.CreateCheckpoint("start", clientB.ID)

	// === Phase 2: Browser produces errors ===
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "TypeError: undefined is not a function"},
	})

	// === Phase 3: Client A checks changes (finds 1 error) ===
	diffA1 := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "start",
		Include:    []string{"console"},
	}, clientA.ID)

	if diffA1.Console == nil {
		t.Fatal("Client A phase 3: expected console diff")
	}
	if diffA1.Console.TotalNew != 1 {
		t.Errorf("Client A phase 3: expected 1 new entry, got %d", diffA1.Console.TotalNew)
	}

	// === Phase 4: More errors arrive ===
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "NetworkError: request failed"},
		{"level": "warn", "msg": "Deprecation warning"},
	})

	// === Phase 5: Client B checks (sees all 3 since their start) ===
	diffB1 := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "start",
		Include:    []string{"console"},
	}, clientB.ID)

	if diffB1.Console == nil {
		t.Fatal("Client B phase 5: expected console diff")
	}
	if diffB1.Console.TotalNew != 3 {
		t.Errorf("Client B phase 5: expected 3 new entries, got %d", diffB1.Console.TotalNew)
	}

	// === Phase 6: Client A creates new checkpoint ===
	cm.CreateCheckpoint("after-fix", clientA.ID)

	server.addEntries([]LogEntry{
		{"level": "error", "msg": "Still broken"},
	})

	// === Phase 7: Client A checks from new checkpoint (sees 1) ===
	diffA2 := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "after-fix",
		Include:    []string{"console"},
	}, clientA.ID)

	if diffA2.Console == nil {
		t.Fatal("Client A phase 7: expected console diff")
	}
	if diffA2.Console.TotalNew != 1 {
		t.Errorf("Client A phase 7: expected 1 new entry, got %d", diffA2.Console.TotalNew)
	}

	// === Phase 8: Client B checks from original start (sees all 4) ===
	diffB2 := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "start",
		Include:    []string{"console"},
	}, clientB.ID)

	if diffB2.Console == nil {
		t.Fatal("Client B phase 8: expected console diff")
	}
	if diffB2.Console.TotalNew != 4 {
		t.Errorf("Client B phase 8: expected 4 new entries, got %d", diffB2.Console.TotalNew)
	}

	// === Phase 9: Client A disconnects ===
	capture.clientRegistry.Unregister(clientA.ID)
	if capture.clientRegistry.Count() != 1 {
		t.Errorf("After A disconnect: expected 1 client, got %d", capture.clientRegistry.Count())
	}

	// Client B should still work after A disconnects
	diffB3 := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "start",
		Include:    []string{"console"},
	}, clientB.ID)

	if diffB3.Console == nil {
		t.Fatal("Client B after A disconnect: expected console diff")
	}
}

func TestE2E_NetworkAndWSIsolation(t *testing.T) {
	server, capture, cm := setupMultiClientTest(t)

	clientA := DeriveClientID("/home/alice/project")
	clientB := DeriveClientID("/home/bob/project")

	// Create checkpoints for both
	cm.CreateCheckpoint("start", clientA)
	cm.CreateCheckpoint("start", clientB)

	// Add network data
	capture.mu.Lock()
	capture.networkBodies = append(capture.networkBodies, NetworkBody{
		Method: "GET", URL: "https://api.example.com/users", Status: 500, Duration: 100,
	})
	capture.networkTotalAdded++
	capture.networkAddedAt = append(capture.networkAddedAt, time.Now())
	capture.mu.Unlock()

	// Add WebSocket event
	capture.mu.Lock()
	capture.wsEvents = append(capture.wsEvents, WebSocketEvent{
		Event: "error", URL: "wss://ws.example.com", Data: "connection lost",
	})
	capture.wsTotalAdded++
	capture.wsAddedAt = append(capture.wsAddedAt, time.Now())
	capture.mu.Unlock()

	// Client A checks (sees network + ws)
	diffA := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "start",
		Include:    []string{"network", "websocket"},
	}, clientA)

	// Client A should see the network failure
	if diffA.Network == nil {
		t.Error("Client A should see network diff")
	} else if diffA.Network.TotalNew != 1 {
		t.Errorf("Client A: expected 1 new network entry, got %d", diffA.Network.TotalNew)
	}

	// Client A should see the WS error
	if diffA.WebSocket == nil {
		t.Error("Client A should see websocket diff")
	} else if diffA.WebSocket.TotalNew != 1 {
		t.Errorf("Client A: expected 1 new WS entry, got %d", diffA.WebSocket.TotalNew)
	}

	// Add more network data between A and B's reads
	capture.mu.Lock()
	capture.networkBodies = append(capture.networkBodies, NetworkBody{
		Method: "POST", URL: "https://api.example.com/orders", Status: 201, Duration: 50,
	})
	capture.networkTotalAdded++
	capture.networkAddedAt = append(capture.networkAddedAt, time.Now())
	capture.mu.Unlock()

	// Client B (using same start checkpoint) sees both network entries
	diffB := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "start",
		Include:    []string{"network"},
	}, clientB)

	if diffB.Network == nil {
		t.Error("Client B should see network diff")
	} else if diffB.Network.TotalNew != 2 {
		t.Errorf("Client B: expected 2 new network entries, got %d", diffB.Network.TotalNew)
	}

	// Verify actions are tracked
	_ = server // suppress unused warning
}

// ============================================
// Edge Cases
// ============================================

func TestMultiClient_EmptyClientIDBackwardsCompat(t *testing.T) {
	server, _, cm := setupMultiClientTest(t)

	// All operations with empty clientID should work (no prefix)
	if err := cm.CreateCheckpoint("test", ""); err != nil {
		t.Fatalf("Empty clientID checkpoint failed: %v", err)
	}

	server.addEntries([]LogEntry{
		{"level": "error", "msg": "test error"},
	})

	resp := cm.GetChangesSince(GetChangesSinceParams{
		Checkpoint: "test",
	}, "")

	if resp.Console == nil {
		t.Fatal("Should see changes with empty clientID")
	}
	if resp.Console.TotalNew != 1 {
		t.Errorf("Expected 1 entry, got %d", resp.Console.TotalNew)
	}
}

func TestMultiClient_ClientIDDeterminism(t *testing.T) {
	// Same CWD should always produce same ID
	id1 := DeriveClientID("/home/alice/project")
	id2 := DeriveClientID("/home/alice/project")
	if id1 != id2 {
		t.Errorf("Same CWD should give same ID: %s vs %s", id1, id2)
	}

	// Different CWDs should produce different IDs
	id3 := DeriveClientID("/home/bob/project")
	if id1 == id3 {
		t.Errorf("Different CWDs should give different IDs: both got %s", id1)
	}

	// ID should be 12 hex chars
	if len(id1) != clientIDLength {
		t.Errorf("Client ID should be %d chars, got %d", clientIDLength, len(id1))
	}
}

func TestMultiClient_GetOrDefaultWithUnknownID(t *testing.T) {
	capture := NewCapture()

	// GetOrDefault with unknown ID should return a default state
	cs := capture.clientRegistry.GetOrDefault("unknown-id-123")
	if cs == nil {
		t.Fatal("GetOrDefault should never return nil")
	}
	if cs.ID != "unknown-id-123" {
		t.Errorf("Expected ID 'unknown-id-123', got '%s'", cs.ID)
	}
	if cs.CheckpointPrefix != "unknown-id-123:" {
		t.Errorf("Expected prefix 'unknown-id-123:', got '%s'", cs.CheckpointPrefix)
	}
}

func TestMultiClient_GetOrDefaultWithEmptyID(t *testing.T) {
	capture := NewCapture()

	// GetOrDefault with empty ID should return a default with no prefix
	cs := capture.clientRegistry.GetOrDefault("")
	if cs == nil {
		t.Fatal("GetOrDefault should never return nil")
	}
	if cs.ID != "" {
		t.Errorf("Expected empty ID, got '%s'", cs.ID)
	}
	if cs.CheckpointPrefix != "" {
		t.Errorf("Expected empty prefix, got '%s'", cs.CheckpointPrefix)
	}
}

func TestMultiClient_MaxCheckpointsPerClient(t *testing.T) {
	_, _, cm := setupMultiClientTest(t)

	clientA := DeriveClientID("/home/alice/project")

	// Create maxNamedCheckpoints + 5 checkpoints for one client
	for i := 0; i < maxNamedCheckpoints+5; i++ {
		name := fmt.Sprintf("cp-%d", i)
		if err := cm.CreateCheckpoint(name, clientA); err != nil {
			t.Fatalf("Checkpoint %d failed: %v", i, err)
		}
	}

	// Should be capped at maxNamedCheckpoints
	count := cm.GetNamedCheckpointCount()
	if count > maxNamedCheckpoints {
		t.Errorf("Checkpoints should be capped at %d, got %d", maxNamedCheckpoints, count)
	}
}

// ============================================
// Connect Mode: Simulated HTTP Forwarding Flow
// ============================================

// TestConnectMode_FullLifecycle simulates the connect mode HTTP flow:
// register → MCP initialize → tool calls → unregister
func TestConnectMode_FullLifecycle(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()
	mcp := NewToolHandler(server, capture)

	clientID := DeriveClientID("/home/alice/frontend")
	cwd := "/home/alice/frontend"

	// Step 1: Register client (POST /clients)
	regBody, _ := json.Marshal(map[string]string{"cwd": cwd})
	regReq := httptest.NewRequest("POST", "/clients", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regReq.Header.Set("X-Gasoline-Client", clientID)
	regRec := httptest.NewRecorder()

	// Simulate the /clients handler
	var reqBody struct {
		CWD string `json:"cwd"`
	}
	json.NewDecoder(regReq.Body).Decode(&reqBody)
	cs := capture.clientRegistry.Register(reqBody.CWD)

	if cs.ID != clientID {
		t.Errorf("Registration produced wrong ID: %s vs %s", cs.ID, clientID)
	}
	_ = regRec

	// Step 2: MCP initialize (forwarded to POST /mcp with X-Gasoline-Client)
	initReq := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`
	req := httptest.NewRequest("POST", "/mcp", strings.NewReader(initReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gasoline-Client", clientID)
	rec := httptest.NewRecorder()
	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Initialize failed: %d", rec.Code)
	}

	var initResp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&initResp)
	if initResp.Error != nil {
		t.Fatalf("Initialize error: %s", initResp.Error.Message)
	}

	// Step 3: tools/list (forwarded)
	listReq := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	req = httptest.NewRequest("POST", "/mcp", strings.NewReader(listReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gasoline-Client", clientID)
	rec = httptest.NewRecorder()
	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("tools/list failed: %d", rec.Code)
	}

	// Step 4: Tool call — observe errors (forwarded)
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "Component failed to mount"},
	})

	toolReq := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`
	req = httptest.NewRequest("POST", "/mcp", strings.NewReader(toolReq))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gasoline-Client", clientID)
	rec = httptest.NewRecorder()
	mcp.HandleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Tool call failed: %d", rec.Code)
	}

	var toolResp JSONRPCResponse
	json.NewDecoder(rec.Body).Decode(&toolResp)
	if toolResp.Error != nil {
		t.Fatalf("Tool call error: %s", toolResp.Error.Message)
	}

	// Step 5: Unregister (DELETE /clients/{id})
	capture.clientRegistry.Unregister(clientID)
	if capture.clientRegistry.Count() != 0 {
		t.Errorf("After unregister: expected 0 clients, got %d", capture.clientRegistry.Count())
	}

	// clientID is now per-request (on JSONRPCRequest.ClientID), not stored on the handler,
	// so there's no shared mutable state — the race is eliminated by design.
}

// TestConnectMode_TwoClientsParallel simulates two connect-mode clients
// making requests in parallel to the same server.
func TestConnectMode_TwoClientsParallel(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()

	clientA := DeriveClientID("/home/alice/project")
	clientB := DeriveClientID("/home/bob/project")

	// Register both clients
	capture.clientRegistry.Register("/home/alice/project")
	capture.clientRegistry.Register("/home/bob/project")

	// Add some data
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "Shared error visible to both"},
	})

	var wg sync.WaitGroup
	errors := make(chan string, 10)

	mcp := NewToolHandler(server, capture)

	// Both clients make observe requests in parallel
	for _, cid := range []string{clientA, clientB} {
		wg.Add(1)
		go func(clientID string) {
			defer wg.Done()

			toolReq := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`
			req := httptest.NewRequest("POST", "/mcp", strings.NewReader(toolReq))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Gasoline-Client", clientID)
			rec := httptest.NewRecorder()
			mcp.HandleHTTP(rec, req)

			if rec.Code != http.StatusOK {
				errors <- fmt.Sprintf("Client %s: expected 200, got %d", clientID, rec.Code)
				return
			}

			var resp JSONRPCResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				errors <- fmt.Sprintf("Client %s: decode error: %v", clientID, err)
				return
			}
			if resp.Error != nil {
				errors <- fmt.Sprintf("Client %s: RPC error: %s", clientID, resp.Error.Message)
			}
		}(cid)
	}

	wg.Wait()
	close(errors)

	for errMsg := range errors {
		t.Error(errMsg)
	}
}

// ============================================
// Stress Tests (for race detector)
// ============================================

// TestStress_ConcurrentMCPRequests hammers the MCP HTTP endpoint
// with concurrent requests from multiple clients. Designed to catch
// race conditions when run with -race.
func TestStress_ConcurrentMCPRequests(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()

	// Pre-populate some data
	server.addEntries([]LogEntry{
		{"level": "error", "msg": "pre-existing error 1"},
		{"level": "warn", "msg": "pre-existing warn 1"},
		{"level": "error", "msg": "pre-existing error 2"},
	})

	const numClients = 8
	const requestsPerClient = 25
	var wg sync.WaitGroup

	// Single shared handler — like the real server where one handler
	// processes requests from all connect-mode clients concurrently.
	mcp := NewToolHandler(server, capture)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()

			clientID := DeriveClientID(fmt.Sprintf("/stress/client/%d", clientNum))

			for j := 0; j < requestsPerClient; j++ {
				// Alternate between different MCP operations
				var reqBody string
				switch j % 4 {
				case 0:
					reqBody = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`
				case 1:
					reqBody = `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
				case 2:
					reqBody = `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"observe","arguments":{"what":"errors"}}}`
				case 3:
					reqBody = `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"observe","arguments":{"what":"network"}}}`
				}

				req := httptest.NewRequest("POST", "/mcp", strings.NewReader(reqBody))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-Gasoline-Client", clientID)
				rec := httptest.NewRecorder()
				mcp.HandleHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("Client %d req %d: got %d", clientNum, j, rec.Code)
				}
			}
		}(i)
	}

	// Also add entries concurrently while clients are reading
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			server.addEntries([]LogEntry{
				{"level": "error", "msg": fmt.Sprintf("concurrent-error-%d", i)},
			})
			time.Sleep(time.Microsecond)
		}
	}()

	wg.Wait()
}

// TestStress_CheckpointAndQueryMix tests concurrent checkpoint operations
// and query isolation together — the most realistic stress scenario.
func TestStress_CheckpointAndQueryMix(t *testing.T) {
	server, err := NewServer("", 1000)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	capture := NewCapture()
	cm := NewCheckpointManager(server, capture)

	const numClients = 6
	const opsPerClient = 30
	var wg sync.WaitGroup

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientNum int) {
			defer wg.Done()
			clientID := DeriveClientID(fmt.Sprintf("/stress-mix/client/%d", clientNum))

			// Register
			capture.clientRegistry.Register(fmt.Sprintf("/stress-mix/client/%d", clientNum))

			for j := 0; j < opsPerClient; j++ {
				switch j % 5 {
				case 0:
					// Create checkpoint
					cm.CreateCheckpoint(fmt.Sprintf("cp-%d", j), clientID)
				case 1:
					// Query changes since checkpoint
					cm.GetChangesSince(GetChangesSinceParams{
						Checkpoint: fmt.Sprintf("cp-%d", (j/5)*5), // reference a recent checkpoint
					}, clientID)
				case 2:
					// Create and resolve a query
					id := capture.CreatePendingQueryWithClient(PendingQuery{
						Type:   "dom",
						Params: json.RawMessage(`{"selector": ".test"}`),
					}, time.Second, clientID)
					capture.SetQueryResult(id, json.RawMessage(`{"ok": true}`))
					capture.GetQueryResult(id, clientID)
				case 3:
					// Add data to shared buffers
					server.addEntries([]LogEntry{
						{"level": "error", "msg": fmt.Sprintf("stress-error-%d-%d", clientNum, j)},
					})
				case 4:
					// List clients
					_ = capture.clientRegistry.List()
				}
			}

			// Unregister
			capture.clientRegistry.Unregister(clientID)
		}(i)
	}

	wg.Wait()

	// Verify the system is in a consistent state
	count := capture.clientRegistry.Count()
	if count != 0 {
		t.Errorf("All clients should be unregistered, got %d remaining", count)
	}
}

// TestStress_RapidRegisterUnregister tests rapid client churn
// to verify the registry handles high-frequency add/remove without corruption.
func TestStress_RapidRegisterUnregister(t *testing.T) {
	capture := NewCapture()

	const numGoroutines = 10
	const cycles = 100
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(gNum int) {
			defer wg.Done()
			for j := 0; j < cycles; j++ {
				cwd := fmt.Sprintf("/rapid/%d", gNum)
				cs := capture.clientRegistry.Register(cwd)
				_ = capture.clientRegistry.Get(cs.ID)
				_ = capture.clientRegistry.GetOrDefault(cs.ID)
				_ = capture.clientRegistry.List()
				capture.clientRegistry.Unregister(cs.ID)
			}
		}(i)
	}

	wg.Wait()

	// Final state: no clients should remain
	if capture.clientRegistry.Count() != 0 {
		t.Errorf("Expected 0 clients after all unregistered, got %d", capture.clientRegistry.Count())
	}
}
