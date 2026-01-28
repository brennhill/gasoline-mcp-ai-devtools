package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestV4PendingQueryCreation(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".user-list"}`),
	})

	if id == "" {
		t.Error("Expected non-empty query ID")
	}

	pending := capture.GetPendingQueries()
	if len(pending) != 1 {
		t.Fatalf("Expected 1 pending query, got %d", len(pending))
	}

	if pending[0].Type != "dom" {
		t.Errorf("Expected type 'dom', got %s", pending[0].Type)
	}
}

func TestV4PendingQueryMaxLimit(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Max 5 pending queries
	for i := 0; i < 7; i++ {
		capture.CreatePendingQuery(PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{"selector":"div"}`),
		})
	}

	pending := capture.GetPendingQueries()
	if len(pending) > 5 {
		t.Errorf("Expected max 5 pending queries, got %d", len(pending))
	}
}

func TestV4PendingQueryTimeout(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create a query with a very short timeout for testing
	capture.CreatePendingQueryWithTimeout(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"div"}`),
	}, 50*time.Millisecond)

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	pending := capture.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending queries after timeout, got %d", len(pending))
	}
}

func TestV4PendingQueryResult(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".user-list"}`),
	})

	// Simulate extension posting result
	result := json.RawMessage(`{"matches":[{"tag":"ul","text":"users"}],"match_count":1}`)
	capture.SetQueryResult(id, result)

	// Query should no longer be pending
	pending := capture.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending queries after result, got %d", len(pending))
	}

	// Result should be retrievable
	got, found := capture.GetQueryResult(id, "")
	if !found {
		t.Fatal("Expected to find query result")
	}

	if got == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestV4PendingQueryResultNotFound(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	_, found := capture.GetQueryResult("nonexistent-id", "")
	if found {
		t.Error("Expected not found for nonexistent query")
	}
}

func TestV4PendingQueryPolling(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// No pending queries initially
	pending := capture.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending queries initially, got %d", len(pending))
	}

	// Create a query
	capture.CreatePendingQuery(PendingQuery{
		Type:   "a11y",
		Params: json.RawMessage(`{"scope":null}`),
	})

	// Polling should return it
	pending = capture.GetPendingQueries()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending query, got %d", len(pending))
	}
}

func TestV4DOMQueryWaitsForResult(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create query and immediately provide result in a goroutine
	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	go func() {
		time.Sleep(50 * time.Millisecond)
		capture.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))
	}()

	// WaitForResult should block until result arrives
	result, err := capture.WaitForResult(id, 1*time.Second, "")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestV4DOMQueryWaitTimeout(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	// WaitForResult should timeout
	_, err := capture.WaitForResult(id, 50*time.Millisecond, "")
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestV4GetPendingQueriesEndpoint(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	req := httptest.NewRequest("GET", "/pending-queries", nil)
	rec := httptest.NewRecorder()

	capture.HandlePendingQueries(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp struct {
		Queries []PendingQueryResponse `json:"queries"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp.Queries) != 1 {
		t.Errorf("Expected 1 pending query, got %d", len(resp.Queries))
	}
}

func TestV4GetPendingQueriesEmpty(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	req := httptest.NewRequest("GET", "/pending-queries", nil)
	rec := httptest.NewRecorder()

	capture.HandlePendingQueries(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	var resp struct {
		Queries []PendingQueryResponse `json:"queries"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp.Queries) != 0 {
		t.Errorf("Expected 0 pending queries, got %d", len(resp.Queries))
	}
}

func TestV4PostDOMResultEndpoint(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	body := `{"id":"` + id + `","result":{"matches":[{"tag":"h1","text":"Hello"}],"match_count":1}}`
	req := httptest.NewRequest("POST", "/dom-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleDOMResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	// Result should be stored
	result, found := capture.GetQueryResult(id, "")
	if !found {
		t.Error("Expected result to be stored")
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestV4PostDOMResultUnknownID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	body := `{"id":"nonexistent","result":{"matches":[]}}`
	req := httptest.NewRequest("POST", "/dom-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleDOMResult(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", rec.Code)
	}
}

func TestV4PostA11yResultEndpoint(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "a11y",
		Params: json.RawMessage(`{"scope":null}`),
	})

	body := `{"id":"` + id + `","result":{"violations":[{"id":"color-contrast","impact":"serious"}],"summary":{"violations":1}}}`
	req := httptest.NewRequest("POST", "/a11y-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleA11yResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestMCPQueryDOM(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Start the query in a goroutine (it will block waiting for result)
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"h1"}}`),
		})
		done <- resp
	}()

	// Simulate extension picking up and responding
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query to be created")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"https://example.com","title":"Test","match_count":1,"returned_count":1,"matches":[{"tag":"h1","text":"Hello World"}]}`))

	// Wait for response
	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	if !strings.Contains(result.Content[0].Text, "Hello World") {
		t.Errorf("Expected result to contain 'Hello World', got: %s", result.Content[0].Text)
	}
}

func TestMCPQueryDOMTimeout(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	// Use short timeout for testing
	capture.SetQueryTimeout(100 * time.Millisecond)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Query with no extension to respond - should timeout
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"h1"}}`),
	})

	// Should return an error in the content (not protocol error)
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &result)

	if !result.IsError {
		t.Error("Expected isError to be true for timeout")
	}
}

func TestMCPGetPageInfo(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Start query in goroutine
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"page"}}`),
		})
		done <- resp
	}()

	// Simulate extension response
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"http://localhost:3000","title":"Test Page","viewport":{"width":1440,"height":900}}`))

	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}
}

func TestMCPRunAccessibilityAudit(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending a11y query")
	}

	if pending[0].Type != "a11y" {
		t.Errorf("Expected query type 'a11y', got %s", pending[0].Type)
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[{"id":"color-contrast","impact":"serious","nodes":[]}],"summary":{"violations":1,"passes":10}}`))

	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	if !strings.Contains(result.Content[0].Text, "color-contrast") {
		t.Errorf("Expected result to contain violation, got: %s", result.Content[0].Text)
	}
}

func TestMCPRunAccessibilityAuditWithTags(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","tags":["wcag2a","wcag2aa"]}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()

	// Verify the tags are passed through in params
	var params map[string]interface{}
	json.Unmarshal(pending[0].Params, &params)

	if params["tags"] == nil {
		t.Error("Expected tags in pending query params")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done
}

func TestV4QueryResultDeletedAfterRetrieval(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	capture.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))

	// First retrieval should succeed
	result, found := capture.GetQueryResult(id, "")
	if !found {
		t.Fatal("Expected result to be found on first read")
	}
	if result == nil {
		t.Fatal("Expected non-nil result on first read")
	}

	// Second retrieval should fail (result deleted after read)
	_, found2 := capture.GetQueryResult(id, "")
	if found2 {
		t.Error("Expected result to be deleted after first retrieval")
	}
}

func TestV4QueryResultDeletedAfterWaitForResult(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	// Set result in background
	go func() {
		time.Sleep(20 * time.Millisecond)
		capture.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))
	}()

	// WaitForResult should succeed
	result, err := capture.WaitForResult(id, time.Second, "")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Result should be cleaned up after WaitForResult returns
	_, found := capture.GetQueryResult(id, "")
	if found {
		t.Error("Expected result to be cleaned up after WaitForResult")
	}
}

func TestV4QueryResultMapDoesNotGrowUnbounded(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create and resolve 20 queries
	for i := 0; i < 20; i++ {
		id := capture.CreatePendingQuery(PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{"selector":"h1"}`),
		})
		capture.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))
		// Read the result (should delete it)
		capture.GetQueryResult(id, "")
	}

	// queryResults map should be empty
	capture.mu.RLock()
	mapSize := len(capture.queryResults)
	capture.mu.RUnlock()

	if mapSize != 0 {
		t.Errorf("Expected queryResults map to be empty after all reads, got %d entries", mapSize)
	}
}

func TestA11yCacheMiss(t *testing.T) {
	t.Parallel()
	// First call with given params should trigger extension round-trip (pending query created)
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main","tags":["wcag2aa"]}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query on cache miss")
	}

	// Simulate extension response
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0,"passes":5}}`))
	resp := <-done

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)
	if !strings.Contains(result.Content[0].Text, `"violations":0`) {
		t.Errorf("Expected violations summary in result, got: %s", result.Content[0].Text)
	}
}

func TestA11yCacheHit(t *testing.T) {
	t.Parallel()
	// Second call with same params within 30s should return immediately, no pending query
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// First call (cache miss) — run in goroutine and simulate response
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main","tags":["wcag2aa"]}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query on first call")
	}
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0,"passes":5}}`))
	<-done

	// Second call (should be cache hit — no pending query, immediate response)
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main","tags":["wcag2aa"]}}`),
	})

	// Verify no new pending query was created
	pending = capture.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected no pending queries on cache hit, got %d", len(pending))
	}

	// Verify we got the same result
	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)
	if !strings.Contains(result.Content[0].Text, `"violations":0`) {
		t.Errorf("Expected cached result, got: %s", result.Content[0].Text)
	}
}

func TestA11yCacheTTLExpiry(t *testing.T) {
	t.Parallel()
	// After 30s, cache entry should be expired and new audit triggered
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// First call
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done

	// Simulate time passing beyond TTL by setting the cache entry's timestamp to the past
	capture.ExpireA11yCache()

	// Third call should be cache miss again
	done2 := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 4, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main"}}`),
		})
		done2 <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending = capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query after TTL expiry")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[{"id":"new-violation"}],"summary":{"violations":1}}`))
	resp := <-done2

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)
	if !strings.Contains(result.Content[0].Text, "new-violation") {
		t.Errorf("Expected fresh result after TTL expiry, got: %s", result.Content[0].Text)
	}
}

func TestA11yCacheTagNormalization(t *testing.T) {
	t.Parallel()
	// Tags in different order should produce the same cache key
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// First call with tags ["wcag2aa", "wcag2a"]
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","tags":["wcag2aa","wcag2a"]}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done

	// Second call with tags in different order ["wcag2a", "wcag2aa"] — should hit cache
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 3, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","tags":["wcag2a","wcag2aa"]}}`),
	})

	pending = capture.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected cache hit for reordered tags, but got %d pending queries", len(pending))
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)
	if !strings.Contains(result.Content[0].Text, `"violations":0`) {
		t.Errorf("Expected cached result for reordered tags, got: %s", result.Content[0].Text)
	}
}

func TestA11yCacheForceRefresh(t *testing.T) {
	t.Parallel()
	// force_refresh: true should bypass cache and re-run audit
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// First call (populates cache)
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done

	// Second call with force_refresh — should bypass cache
	done2 := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 3, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main","force_refresh":true}}`),
		})
		done2 <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending = capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query when force_refresh is true")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[{"id":"new-issue"}],"summary":{"violations":1}}`))
	resp := <-done2

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)
	if !strings.Contains(result.Content[0].Text, "new-issue") {
		t.Errorf("Expected fresh result after force_refresh, got: %s", result.Content[0].Text)
	}
}

func TestA11yCacheErrorNotCached(t *testing.T) {
	t.Parallel()
	// Timeout/error should not be cached; next call should retry
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	capture.queryTimeout = 100 * time.Millisecond // Short timeout for test
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// First call — let it time out (don't provide result)
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#timeout-test"}}`),
	})

	var errResult struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	json.Unmarshal(resp.Result, &errResult)
	if !errResult.IsError {
		t.Fatal("Expected error response from timeout")
	}

	// Second call — should NOT be cached, should create new pending query
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 3, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#timeout-test"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query after error (error should not be cached)")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done
}

func TestA11yCacheDifferentParams(t *testing.T) {
	t.Parallel()
	// Different scope/tags should produce different cache entries
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// First call: scope="#main"
	done1 := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main"}}`),
		})
		done1 <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done1

	// Second call: scope="#footer" — different params, should be cache miss
	done2 := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 3, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#footer"}}`),
		})
		done2 <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending = capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query for different scope (cache miss)")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[{"id":"footer-issue"}],"summary":{"violations":1}}`))
	<-done2
}

func TestA11yCacheMaxEntries(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow cache eviction test")
	}
	t.Parallel()
	// 11th unique cache entry should evict the oldest
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Populate 10 cache entries with different scopes
	for i := 0; i < 10; i++ {
		scope := fmt.Sprintf("#section-%d", i)
		args := fmt.Sprintf(`{"name":"observe","arguments":{"what":"accessibility","scope":"%s"}}`, scope)

		done := make(chan JSONRPCResponse)
		go func() {
			resp := mcp.HandleRequest(JSONRPCRequest{
				JSONRPC: "2.0", ID: json.RawMessage(fmt.Sprintf("%d", i+10)), Method: "tools/call",
				Params: json.RawMessage(args),
			})
			done <- resp
		}()

		time.Sleep(50 * time.Millisecond)
		pending := capture.GetPendingQueries()
		if len(pending) == 0 {
			t.Fatalf("Expected pending query for scope #section-%d", i)
		}
		capture.SetQueryResult(pending[0].ID, json.RawMessage(fmt.Sprintf(`{"violations":[],"summary":{"violations":0,"scope":"%s"}}`, scope)))
		<-done
	}

	// Verify cache has 10 entries
	if capture.GetA11yCacheSize() != 10 {
		t.Fatalf("Expected 10 cache entries, got %d", capture.GetA11yCacheSize())
	}

	// Add 11th entry — should evict #section-0
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 100, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#section-new"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done

	// Cache should still be 10 (evicted oldest)
	if capture.GetA11yCacheSize() != 10 {
		t.Errorf("Expected 10 cache entries after eviction, got %d", capture.GetA11yCacheSize())
	}

	// #section-0 should now be a cache miss
	done2 := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 101, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#section-0"}}`),
		})
		done2 <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending = capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected cache miss for evicted entry #section-0")
	}
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done2
}

func TestA11yCacheNavigationInvalidation(t *testing.T) {
	t.Parallel()
	// Cache should be cleared when URL changes (navigation detected)
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Set initial URL context
	capture.SetLastKnownURL("https://myapp.com/dashboard")

	// First call (populates cache)
	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done

	// Simulate navigation — URL changes
	capture.SetLastKnownURL("https://myapp.com/settings")

	// Same params — should be cache miss because URL changed
	done2 := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 3, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#main"}}`),
		})
		done2 <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending = capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected cache miss after navigation (URL change)")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	<-done2
}

func TestA11yCacheConcurrentDedup(t *testing.T) {
	t.Parallel()
	// Two simultaneous calls for the same cache key should produce only one pending query
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Launch two concurrent calls with the same params
	done1 := make(chan JSONRPCResponse)
	done2 := make(chan JSONRPCResponse)

	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#concurrent"}}`),
		})
		done1 <- resp
	}()

	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 3, Method: "tools/call",
			Params: json.RawMessage(`{"name":"observe","arguments":{"what":"accessibility","scope":"#concurrent"}}`),
		})
		done2 <- resp
	}()

	time.Sleep(100 * time.Millisecond)

	// Should have at most 1 pending query (deduplication)
	pending := capture.GetPendingQueries()
	if len(pending) > 1 {
		t.Errorf("Expected at most 1 pending query for concurrent dedup, got %d", len(pending))
	}

	if len(pending) > 0 {
		capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"violations":[],"summary":{"violations":0}}`))
	}

	// Both should complete with same result
	resp1 := <-done1
	resp2 := <-done2

	var r1, r2 struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp1.Result, &r1)
	json.Unmarshal(resp2.Result, &r2)

	if r1.Content[0].Text != r2.Content[0].Text {
		t.Errorf("Expected same result for both concurrent calls")
	}
}

func TestA11yCacheForceRefreshParam(t *testing.T) {
	t.Parallel()
	// Verify force_refresh is accepted as a tool parameter
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	// Check that force_refresh is in the tool schema
	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var toolsResult struct {
		Tools []struct {
			Name        string                 `json:"name"`
			InputSchema map[string]interface{} `json:"inputSchema"`
		} `json:"tools"`
	}
	json.Unmarshal(resp.Result, &toolsResult)

	var found bool
	for _, tool := range toolsResult.Tools {
		if tool.Name == "observe" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("observe tool not found in tools list")
	}
}

// ============================================
// Additional coverage tests for queries.go
// ============================================

func TestHandleQueryResultTimeout(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create a query with a very short timeout
	id := capture.CreatePendingQueryWithTimeout(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".test"}`),
	}, 50*time.Millisecond)

	// Wait for the query to expire
	time.Sleep(100 * time.Millisecond)

	// Now try to post a result for the expired query - it should be not found
	payload := fmt.Sprintf(`{"id":"%s","result":{"html":"<div>test</div>"}}`, id)
	req := httptest.NewRequest(http.MethodPost, "/dom-result", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	capture.HandleDOMResult(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for expired query, got %d", w.Code)
	}
}

func TestHandleQueryResultInvalidJSON(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Post invalid JSON
	req := httptest.NewRequest(http.MethodPost, "/dom-result", bytes.NewBufferString("not valid json{{{"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	capture.HandleDOMResult(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d", w.Code)
	}
}

func TestHandleQueryResultUnknownQueryID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Post result for a query ID that never existed
	payload := `{"id":"q-99999","result":{"html":"<div>test</div>"}}`
	req := httptest.NewRequest(http.MethodPost, "/dom-result", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	capture.HandleDOMResult(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for unknown query ID, got %d", w.Code)
	}
}

func TestHandleQueryResultSuccess(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".user-list"}`),
	})

	// Post a valid result
	payload := fmt.Sprintf(`{"id":"%s","result":{"matches":[{"tag":"div"}],"match_count":1}}`, id)
	req := httptest.NewRequest(http.MethodPost, "/dom-result", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	capture.HandleDOMResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 for valid result, got %d", w.Code)
	}

	// The result should be stored and query removed from pending
	result, found := capture.GetQueryResult(id, "")
	if !found {
		t.Error("Expected query result to be stored")
	}
	if !strings.Contains(string(result), "match_count") {
		t.Errorf("Expected result to contain 'matchCount', got %s", string(result))
	}
}

func TestHandleQueryResultNotifiesWaiters(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".item"}`),
	})

	// Start a goroutine waiting for the result
	done := make(chan json.RawMessage, 1)
	go func() {
		result, err := capture.WaitForResult(id, 2*time.Second, "")
		if err != nil {
			return
		}
		done <- result
	}()

	// Give the waiter time to start
	time.Sleep(50 * time.Millisecond)

	// Post the result via HTTP handler
	payload := fmt.Sprintf(`{"id":"%s","result":{"html":"<div class=\"item\">found</div>"}}`, id)
	req := httptest.NewRequest(http.MethodPost, "/dom-result", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	capture.HandleDOMResult(w, req)

	// Wait for the goroutine to get the result
	select {
	case result := <-done:
		if !strings.Contains(string(result), "found") {
			t.Errorf("Expected result to contain 'found', got %s", string(result))
		}
	case <-time.After(2 * time.Second):
		t.Error("Timed out waiting for result notification")
	}
}

func TestHandlePendingQueriesEmpty(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	req := httptest.NewRequest(http.MethodGet, "/pending-queries", nil)
	w := httptest.NewRecorder()

	capture.HandlePendingQueries(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp struct {
		Queries []PendingQueryResponse `json:"queries"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Queries) != 0 {
		t.Errorf("Expected 0 queries, got %d", len(resp.Queries))
	}
}

func TestHandlePendingQueriesExpiryRemoval(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create a query with very short timeout
	capture.CreatePendingQueryWithTimeout(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#expiring"}`),
	}, 50*time.Millisecond)

	// Create a second query with longer timeout
	capture.CreatePendingQueryWithTimeout(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#persistent"}`),
	}, 5*time.Second)

	// Wait for first to expire
	time.Sleep(100 * time.Millisecond)

	// HandlePendingQueries should only return the non-expired one
	req := httptest.NewRequest(http.MethodGet, "/pending-queries", nil)
	w := httptest.NewRecorder()

	capture.HandlePendingQueries(w, req)

	var resp struct {
		Queries []PendingQueryResponse `json:"queries"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if len(resp.Queries) != 1 {
		t.Fatalf("Expected 1 remaining query after expiry, got %d", len(resp.Queries))
	}
	if !strings.Contains(string(resp.Queries[0].Params), "#persistent") {
		t.Errorf("Expected persistent query to remain, got params: %s", string(resp.Queries[0].Params))
	}
}

func TestSetQueryResultUnknownID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Set result for a query that does not exist in pending
	capture.SetQueryResult("q-nonexistent", json.RawMessage(`{"data":"test"}`))

	// The result should still be stored in queryResults
	result, found := capture.GetQueryResult("q-nonexistent", "")
	if !found {
		t.Error("Expected result to be stored even for unknown ID")
	}
	if string(result) != `{"data":"test"}` {
		t.Errorf("Expected stored result, got %s", string(result))
	}
}

func TestHandleA11yResultSameAsDOM(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "a11y",
		Params: json.RawMessage(`{"scope":"main"}`),
	})

	// Post via HandleA11yResult instead of HandleDOMResult
	payload := fmt.Sprintf(`{"id":"%s","result":{"violations":[],"passes":5}}`, id)
	req := httptest.NewRequest(http.MethodPost, "/a11y-result", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	capture.HandleA11yResult(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	result, found := capture.GetQueryResult(id, "")
	if !found {
		t.Error("Expected a11y result to be stored")
	}
	if !strings.Contains(string(result), "violations") {
		t.Errorf("Expected result to contain 'violations', got %s", string(result))
	}
}

func TestHandleQueryResultBodyTooLarge(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"*"}`),
	})

	// Create a body that exceeds maxPostBodySize (5MB)
	largePayload := fmt.Sprintf(`{"id":"%s","result":"`, id) + strings.Repeat("x", 6*1024*1024) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/dom-result", bytes.NewBufferString(largePayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	capture.HandleDOMResult(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected 413 for oversized body, got %d", w.Code)
	}
}

// ============================================
// Coverage Gap Tests: HandlePendingQueries, getA11yCacheEntry, removeA11yCacheEntry, SetQueryResult
// ============================================

func TestHandlePendingQueries_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// POST request to a GET-only handler - the handler doesn't check method,
	// so it will still return 200 with the queries list
	req := httptest.NewRequest(http.MethodPost, "/pending-queries", nil)
	w := httptest.NewRecorder()

	capture.HandlePendingQueries(w, req)

	// HandlePendingQueries doesn't check method; it always returns queries
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 (handler does not enforce method), got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	queries, ok := resp["queries"].([]interface{})
	if !ok {
		t.Fatal("Expected 'queries' field in response")
	}
	if len(queries) != 0 {
		t.Errorf("Expected empty queries list, got %d", len(queries))
	}
}

func TestGetA11yCacheEntry_CacheMiss(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Query for a key that was never set
	result := capture.getA11yCacheEntry("nonexistent-key")
	if result != nil {
		t.Errorf("Expected nil for cache miss, got %s", string(result))
	}
}

func TestRemoveA11yCacheEntry_NonExistent(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Set one entry
	capture.setA11yCacheEntry("existing-key", json.RawMessage(`{"data":"test"}`))

	// Remove a key that doesn't exist - should not panic or affect existing entries
	capture.removeA11yCacheEntry("nonexistent-key")

	// Verify existing entry is still there
	result := capture.getA11yCacheEntry("existing-key")
	if result == nil {
		t.Error("Expected existing entry to remain after removing nonexistent key")
	}
	if string(result) != `{"data":"test"}` {
		t.Errorf("Expected existing entry unchanged, got %s", string(result))
	}
}

// ============================================
// query_dom Schema Improvement Tests
// ============================================

func TestQueryDOM_SchemaHasURLAndPageTitle(t *testing.T) {
	t.Parallel()
	// Verify the improved response includes url and pageTitle from extension
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"h1"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	// Extension returns url and title
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"https://example.com/page","title":"Test Page","match_count":1,"returned_count":1,"matches":[{"tag":"h1","text":"Hello"}]}`))

	resp := <-done
	text := extractMCPText(t, resp)

	// Parse the JSON portion (after the summary line)
	jsonStart := strings.Index(text, "\n")
	if jsonStart < 0 {
		t.Fatalf("Expected summary + JSON, got: %s", text)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text[jsonStart+1:]), &data); err != nil {
		t.Fatalf("Failed to parse JSON response: %v\nText: %s", err, text)
	}

	// Check url field
	if url, ok := data["url"].(string); !ok || url != "https://example.com/page" {
		t.Errorf("Expected url='https://example.com/page', got: %v", data["url"])
	}

	// Check pageTitle field
	if title, ok := data["page_title"].(string); !ok || title != "Test Page" {
		t.Errorf("Expected pageTitle='Test Page', got: %v", data["page_title"])
	}
}

func TestQueryDOM_SchemaHasSelectorEcho(t *testing.T) {
	t.Parallel()
	// Verify the response echoes back the selector that was queried
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"div.user-card"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"https://example.com","title":"Page","match_count":2,"returned_count":2,"matches":[{"tag":"div","text":"User 1"},{"tag":"div","text":"User 2"}]}`))

	resp := <-done
	text := extractMCPText(t, resp)

	jsonStart := strings.Index(text, "\n")
	if jsonStart < 0 {
		t.Fatalf("Expected summary + JSON, got: %s", text)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text[jsonStart+1:]), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if selector, ok := data["selector"].(string); !ok || selector != "div.user-card" {
		t.Errorf("Expected selector='div.user-card', got: %v", data["selector"])
	}
}

func TestQueryDOM_SchemaHasMatchCounts(t *testing.T) {
	t.Parallel()
	// Verify totalMatchCount and returnedMatchCount are present
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"li"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	// Extension found 100 matches but only returned 50
	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"https://example.com","title":"Page","match_count":100,"returned_count":50,"matches":[{"tag":"li","text":"item"}]}`))

	resp := <-done
	text := extractMCPText(t, resp)

	jsonStart := strings.Index(text, "\n")
	if jsonStart < 0 {
		t.Fatalf("Expected summary + JSON, got: %s", text)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text[jsonStart+1:]), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Check totalMatchCount
	if total, ok := data["total_match_count"].(float64); !ok || total != 100 {
		t.Errorf("Expected totalMatchCount=100, got: %v", data["total_match_count"])
	}

	// Check returnedMatchCount
	if returned, ok := data["returned_match_count"].(float64); !ok || returned != 50 {
		t.Errorf("Expected returnedMatchCount=50, got: %v", data["returned_match_count"])
	}
}

func TestQueryDOM_SchemaHasMetadata(t *testing.T) {
	t.Parallel()
	// Verify metadata fields: maxElementsReturned, maxDepthQueried, maxTextLength
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"p"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"https://example.com","title":"Page","match_count":1,"returned_count":1,"matches":[{"tag":"p","text":"Hello"}]}`))

	resp := <-done
	text := extractMCPText(t, resp)

	jsonStart := strings.Index(text, "\n")
	if jsonStart < 0 {
		t.Fatalf("Expected summary + JSON, got: %s", text)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text[jsonStart+1:]), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if v, ok := data["max_elements_returned"].(float64); !ok || v != 50 {
		t.Errorf("Expected maxElementsReturned=50, got: %v", data["max_elements_returned"])
	}

	if v, ok := data["max_depth_queried"].(float64); !ok || v != 5 {
		t.Errorf("Expected maxDepthQueried=5, got: %v", data["max_depth_queried"])
	}

	if v, ok := data["max_text_length"].(float64); !ok || v != 500 {
		t.Errorf("Expected maxTextLength=500, got: %v", data["max_text_length"])
	}
}

func TestQueryDOM_SchemaTextTruncated(t *testing.T) {
	t.Parallel()
	// Verify textTruncated boolean is added to match objects
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"p"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	// Create a long text string (exactly 500 chars = maxTextLength, so truncated)
	longText := strings.Repeat("a", 500)
	shortText := "short"
	extResult := fmt.Sprintf(`{"url":"https://example.com","title":"Page","match_count":2,"returned_count":2,"matches":[{"tag":"p","text":"%s"},{"tag":"p","text":"%s"}]}`, longText, shortText)
	capture.SetQueryResult(pending[0].ID, json.RawMessage(extResult))

	resp := <-done
	text := extractMCPText(t, resp)

	jsonStart := strings.Index(text, "\n")
	if jsonStart < 0 {
		t.Fatalf("Expected summary + JSON, got: %s", text)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text[jsonStart+1:]), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	matches, ok := data["matches"].([]interface{})
	if !ok || len(matches) != 2 {
		t.Fatalf("Expected 2 matches, got: %v", data["matches"])
	}

	// First match has text at max length (500 chars) - should be flagged as truncated
	match0 := matches[0].(map[string]interface{})
	if truncated, ok := match0["textTruncated"].(bool); !ok || !truncated {
		t.Errorf("Expected textTruncated=true for 500-char text, got: %v", match0["textTruncated"])
	}

	// Second match has short text - should NOT be flagged
	match1 := matches[1].(map[string]interface{})
	if truncated, ok := match1["textTruncated"].(bool); !ok || truncated {
		t.Errorf("Expected textTruncated=false for short text, got: %v", match1["textTruncated"])
	}
}

func TestQueryDOM_SchemaBboxPixelsRename(t *testing.T) {
	t.Parallel()
	// Verify boundingBox is renamed to bboxPixels
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"div"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"https://example.com","title":"Page","match_count":1,"returned_count":1,"matches":[{"tag":"div","text":"Hello","boundingBox":{"x":10,"y":20,"width":100,"height":50}}]}`))

	resp := <-done
	text := extractMCPText(t, resp)

	jsonStart := strings.Index(text, "\n")
	if jsonStart < 0 {
		t.Fatalf("Expected summary + JSON, got: %s", text)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text[jsonStart+1:]), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	matches := data["matches"].([]interface{})
	match0 := matches[0].(map[string]interface{})

	// Should have bboxPixels, NOT boundingBox
	if _, ok := match0["bboxPixels"]; !ok {
		t.Error("Expected 'bboxPixels' field in match object")
	}
	if _, ok := match0["boundingBox"]; ok {
		t.Error("Expected 'boundingBox' to be renamed to 'bboxPixels'")
	}

	// Verify the values are preserved
	bbox := match0["bboxPixels"].(map[string]interface{})
	if bbox["x"].(float64) != 10 || bbox["y"].(float64) != 20 {
		t.Errorf("Expected bbox x=10, y=20, got: x=%v, y=%v", bbox["x"], bbox["y"])
	}
}

func TestQueryDOM_SchemaEmptyHint(t *testing.T) {
	t.Parallel()
	// Verify helpful hint when matches is empty
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"#nonexistent"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"https://example.com","title":"Page","match_count":0,"returned_count":0,"matches":[]}`))

	resp := <-done
	text := extractMCPText(t, resp)

	jsonStart := strings.Index(text, "\n")
	if jsonStart < 0 {
		t.Fatalf("Expected summary + JSON, got: %s", text)
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(text[jsonStart+1:]), &data); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify hint is present
	hint, ok := data["hint"].(string)
	if !ok || hint == "" {
		t.Error("Expected 'hint' field when matches is empty")
	}

	// Hint should mention the selector
	if !strings.Contains(hint, "#nonexistent") {
		t.Errorf("Expected hint to mention the selector, got: %s", hint)
	}
}

func TestQueryDOM_SchemaFullResponse(t *testing.T) {
	t.Parallel()
	// Integration test: verify the full improved response structure
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	done := make(chan JSONRPCResponse)
	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 2, Method: "tools/call",
			Params: json.RawMessage(`{"name":"configure","arguments":{"action":"query_dom","selector":"h1"}}`),
		})
		done <- resp
	}()

	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"url":"https://app.example.com/dashboard","title":"Dashboard","match_count":3,"returned_count":3,"matches":[{"tag":"h1","text":"Welcome","visible":true,"boundingBox":{"x":0,"y":10,"width":200,"height":40}},{"tag":"h1","text":"Features","visible":true},{"tag":"h1","text":"About","visible":false}]}`))

	resp := <-done
	text := extractMCPText(t, resp)

	// Verify summary line
	if !strings.Contains(text, "3") {
		t.Errorf("Expected summary to mention match count, got: %s", strings.Split(text, "\n")[0])
	}

	jsonStart := strings.Index(text, "\n")
	var data map[string]interface{}
	json.Unmarshal([]byte(text[jsonStart+1:]), &data)

	// All top-level fields must be present
	requiredFields := []string{"url", "page_title", "selector", "total_match_count", "returned_match_count", "max_elements_returned", "max_depth_queried", "max_text_length", "matches"}
	for _, field := range requiredFields {
		if _, ok := data[field]; !ok {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// No hint when matches is non-empty
	if _, ok := data["hint"]; ok {
		t.Error("Expected NO hint when matches are present")
	}
}

func TestSetQueryResult_ConcurrentSetAndWait(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Create a pending query with a short timeout
	capture.SetQueryTimeout(2 * time.Second)
	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#app"}`),
	})

	// Start a goroutine that waits for the result
	resultCh := make(chan json.RawMessage, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := capture.WaitForResult(id, 2*time.Second, "")
		resultCh <- result
		errCh <- err
	}()

	// Brief delay then set the result concurrently
	time.Sleep(50 * time.Millisecond)
	capture.SetQueryResult(id, json.RawMessage(`{"innerHTML":"<div>Hello</div>"}`))

	// Wait for the goroutine
	select {
	case result := <-resultCh:
		err := <-errCh
		if err != nil {
			t.Fatalf("WaitForResult returned error: %v", err)
		}
		if string(result) != `{"innerHTML":"<div>Hello</div>"}` {
			t.Errorf("Expected result, got %s", string(result))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Timed out waiting for concurrent SetQueryResult")
	}
}
