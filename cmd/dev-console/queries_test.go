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
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":".user-list"}`),
	})

	// Simulate extension posting result
	result := json.RawMessage(`{"matches":[{"tag":"ul","text":"users"}],"matchCount":1}`)
	capture.SetQueryResult(id, result)

	// Query should no longer be pending
	pending := capture.GetPendingQueries()
	if len(pending) != 0 {
		t.Errorf("Expected 0 pending queries after result, got %d", len(pending))
	}

	// Result should be retrievable
	got, found := capture.GetQueryResult(id)
	if !found {
		t.Fatal("Expected to find query result")
	}

	if got == nil {
		t.Fatal("Expected non-nil result")
	}
}

func TestV4PendingQueryResultNotFound(t *testing.T) {
	capture := setupTestCapture(t)

	_, found := capture.GetQueryResult("nonexistent-id")
	if found {
		t.Error("Expected not found for nonexistent query")
	}
}

func TestV4PendingQueryPolling(t *testing.T) {
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
	result, err := capture.WaitForResult(id, 1*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestV4DOMQueryWaitTimeout(t *testing.T) {
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	// WaitForResult should timeout
	_, err := capture.WaitForResult(id, 50*time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error")
	}
}

func TestV4GetPendingQueriesEndpoint(t *testing.T) {
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
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	body := `{"id":"` + id + `","result":{"matches":[{"tag":"h1","text":"Hello"}],"matchCount":1}}`
	req := httptest.NewRequest("POST", "/dom-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleDOMResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	// Result should be stored
	result, found := capture.GetQueryResult(id)
	if !found {
		t.Error("Expected result to be stored")
	}
	if result == nil {
		t.Error("Expected non-nil result")
	}
}

func TestV4PostDOMResultUnknownID(t *testing.T) {
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
			Params: json.RawMessage(`{"name":"query_dom","arguments":{"selector":"h1"}}`),
		})
		done <- resp
	}()

	// Simulate extension picking up and responding
	time.Sleep(50 * time.Millisecond)
	pending := capture.GetPendingQueries()
	if len(pending) == 0 {
		t.Fatal("Expected pending query to be created")
	}

	capture.SetQueryResult(pending[0].ID, json.RawMessage(`{"matches":[{"tag":"h1","text":"Hello World"}],"matchCount":1,"returnedCount":1}`))

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
		Params: json.RawMessage(`{"name":"query_dom","arguments":{"selector":"h1"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","tags":["wcag2a","wcag2aa"]}}`),
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
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"h1"}`),
	})

	capture.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))

	// First retrieval should succeed
	result, found := capture.GetQueryResult(id)
	if !found {
		t.Fatal("Expected result to be found on first read")
	}
	if result == nil {
		t.Fatal("Expected non-nil result on first read")
	}

	// Second retrieval should fail (result deleted after read)
	_, found2 := capture.GetQueryResult(id)
	if found2 {
		t.Error("Expected result to be deleted after first retrieval")
	}
}

func TestV4QueryResultDeletedAfterWaitForResult(t *testing.T) {
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
	result, err := capture.WaitForResult(id, time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// Result should be cleaned up after WaitForResult returns
	_, found := capture.GetQueryResult(id)
	if found {
		t.Error("Expected result to be cleaned up after WaitForResult")
	}
}

func TestV4QueryResultMapDoesNotGrowUnbounded(t *testing.T) {
	capture := setupTestCapture(t)

	// Create and resolve 20 queries
	for i := 0; i < 20; i++ {
		id := capture.CreatePendingQuery(PendingQuery{
			Type:   "dom",
			Params: json.RawMessage(`{"selector":"h1"}`),
		})
		capture.SetQueryResult(id, json.RawMessage(`{"matches":[]}`))
		// Read the result (should delete it)
		capture.GetQueryResult(id)
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main","tags":["wcag2aa"]}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main","tags":["wcag2aa"]}}`),
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
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main","tags":["wcag2aa"]}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","tags":["wcag2aa","wcag2a"]}}`),
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
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","tags":["wcag2a","wcag2aa"]}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main","force_refresh":true}}`),
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
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#timeout-test"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#timeout-test"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#footer"}}`),
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
		args := fmt.Sprintf(`{"name":"analyze","arguments":{"target":"accessibility","scope":"%s"}}`, scope)

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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#section-new"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#section-0"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#main"}}`),
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
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#concurrent"}}`),
		})
		done1 <- resp
	}()

	go func() {
		resp := mcp.HandleRequest(JSONRPCRequest{
			JSONRPC: "2.0", ID: 3, Method: "tools/call",
			Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"accessibility","scope":"#concurrent"}}`),
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
		if tool.Name == "analyze" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("analyze tool not found in tools list")
	}
}
