package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestV4NetworkBodiesBuffer(t *testing.T) {
	capture := setupTestCapture(t)

	bodies := []NetworkBody{
		{
			Timestamp:    "2024-01-15T10:30:00.000Z",
			Method:       "POST",
			URL:          "/api/users",
			Status:       201,
			RequestBody:  `{"name":"Alice"}`,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
			Duration:     142,
		},
	}

	capture.AddNetworkBodies(bodies)

	if capture.GetNetworkBodyCount() != 1 {
		t.Errorf("Expected 1 body, got %d", capture.GetNetworkBodyCount())
	}
}

func TestV4NetworkBodiesBufferRotation(t *testing.T) {
	capture := setupTestCapture(t)

	// Add more than max (100) entries
	bodies := make([]NetworkBody, 120)
	for i := range bodies {
		bodies[i] = NetworkBody{Method: "GET", URL: "/api/test", Status: 200}
	}

	capture.AddNetworkBodies(bodies)

	if capture.GetNetworkBodyCount() != 100 {
		t.Errorf("Expected 100 bodies after rotation, got %d", capture.GetNetworkBodyCount())
	}
}

func TestV4NetworkBodiesFilterByURL(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Status: 200},
		{URL: "/api/products", Status: 200},
		{URL: "/api/users/1", Status: 404},
	})

	filtered := capture.GetNetworkBodies(NetworkBodyFilter{URLFilter: "users"})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 bodies matching 'users', got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterByMethod(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", Method: "GET", Status: 200},
		{URL: "/api/test", Method: "POST", Status: 201},
		{URL: "/api/test", Method: "GET", Status: 200},
	})

	filtered := capture.GetNetworkBodies(NetworkBodyFilter{Method: "POST"})

	if len(filtered) != 1 {
		t.Errorf("Expected 1 POST body, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterByStatus(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", Status: 200},
		{URL: "/api/test", Status: 404},
		{URL: "/api/test", Status: 500},
		{URL: "/api/test", Status: 201},
	})

	// Filter for errors only (>= 400)
	filtered := capture.GetNetworkBodies(NetworkBodyFilter{StatusMin: 400})

	if len(filtered) != 2 {
		t.Errorf("Expected 2 error bodies, got %d", len(filtered))
	}

	// Filter for range 400-499
	filtered = capture.GetNetworkBodies(NetworkBodyFilter{StatusMin: 400, StatusMax: 499})

	if len(filtered) != 1 {
		t.Errorf("Expected 1 client error body, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesFilterWithLimit(t *testing.T) {
	capture := setupTestCapture(t)

	for i := 0; i < 50; i++ {
		capture.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", Status: 200},
		})
	}

	filtered := capture.GetNetworkBodies(NetworkBodyFilter{Limit: 10})

	if len(filtered) != 10 {
		t.Errorf("Expected 10 bodies with limit, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesDefaultLimit(t *testing.T) {
	capture := setupTestCapture(t)

	for i := 0; i < 50; i++ {
		capture.AddNetworkBodies([]NetworkBody{
			{URL: "/api/test", Status: 200},
		})
	}

	// Default limit is 20
	filtered := capture.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered) != 20 {
		t.Errorf("Expected 20 bodies with default limit, got %d", len(filtered))
	}
}

func TestV4NetworkBodiesNewestFirst(t *testing.T) {
	capture := setupTestCapture(t)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/first", Timestamp: "2024-01-15T10:30:00.000Z"},
		{URL: "/api/last", Timestamp: "2024-01-15T10:30:05.000Z"},
	})

	filtered := capture.GetNetworkBodies(NetworkBodyFilter{})

	if filtered[0].URL != "/api/last" {
		t.Errorf("Expected newest first, got URL %s", filtered[0].URL)
	}
}

func TestV4NetworkBodiesTruncation(t *testing.T) {
	capture := setupTestCapture(t)

	// Request body > 8KB should be truncated
	largeBody := strings.Repeat("x", 10000)
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", RequestBody: largeBody, Status: 200},
	})

	filtered := capture.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered[0].RequestBody) > 8192 {
		t.Errorf("Expected request body truncated to 8KB, got %d bytes", len(filtered[0].RequestBody))
	}

	if !filtered[0].RequestTruncated {
		t.Error("Expected RequestTruncated flag to be true")
	}
}

func TestV4NetworkBodiesResponseTruncation(t *testing.T) {
	capture := setupTestCapture(t)

	// Response body > 16KB should be truncated
	largeBody := strings.Repeat("y", 20000)
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/test", ResponseBody: largeBody, Status: 200},
	})

	filtered := capture.GetNetworkBodies(NetworkBodyFilter{})

	if len(filtered[0].ResponseBody) > 16384 {
		t.Errorf("Expected response body truncated to 16KB, got %d bytes", len(filtered[0].ResponseBody))
	}

	if !filtered[0].ResponseTruncated {
		t.Error("Expected ResponseTruncated flag to be true")
	}
}

func TestV4PostNetworkBodiesEndpoint(t *testing.T) {
	capture := setupTestCapture(t)

	body := `{"bodies":[{"ts":"2024-01-15T10:30:00.000Z","method":"GET","url":"/api/test","status":200,"responseBody":"{}","contentType":"application/json"}]}`
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	if capture.GetNetworkBodyCount() != 1 {
		t.Errorf("Expected 1 body stored, got %d", capture.GetNetworkBodyCount())
	}
}

func TestV4PostNetworkBodiesInvalidJSON(t *testing.T) {
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString("garbage"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", rec.Code)
	}
}

func TestMCPGetNetworkBodies(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Method: "GET", Status: 200, ResponseBody: `[{"id":1}]`},
		{URL: "/api/users", Method: "POST", Status: 201, RequestBody: `{"name":"Alice"}`},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network_bodies"}}`),
	})

	if resp.Error != nil {
		t.Fatalf("Expected no error, got: %v", resp.Error)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	// New format: summary line + markdown table
	text := result.Content[0].Text
	if !strings.Contains(text, "2 network request-response pair(s)") {
		t.Errorf("Expected summary with '2 network request-response pair(s)', got: %s", text)
	}
	if !strings.Contains(text, "/api/users") {
		t.Errorf("Expected table to contain /api/users, got: %s", text)
	}
}

func TestMCPGetNetworkBodiesWithFilter(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.AddNetworkBodies([]NetworkBody{
		{URL: "/api/users", Method: "GET", Status: 200},
		{URL: "/api/users", Method: "GET", Status: 500},
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network_bodies","status_min":400}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	// New format: summary line + markdown table
	text := result.Content[0].Text
	if !strings.Contains(text, "1 network request-response pair(s)") {
		t.Errorf("Expected summary with '1 network request-response pair(s)', got: %s", text)
	}
}

func TestMCPGetNetworkBodiesEmpty(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"observe","arguments":{"what":"network_bodies"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	// Parse JSON response
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &data); err != nil {
		t.Fatalf("Expected JSON response, got: %s", result.Content[0].Text)
	}

	// Verify empty state
	if count, ok := data["count"].(float64); !ok || count != 0 {
		t.Errorf("Expected count=0, got: %v", data["count"])
	}
}

// ============================================
// evictNBForMemory: loop body exercised
// ============================================

// Test: evictNBForMemory removes bodies one at a time until memory is within limit.
func TestV4NetworkBodiesEvictForMemory(t *testing.T) {
	capture := setupTestCapture(t)

	// nbBufferMemoryLimit = 8MB = 8,388,608 bytes.
	// AddNetworkBodies truncates to maxRequestBodySize (8KB) + maxResponseBodySize (16KB),
	// so to exceed 8MB we must bypass truncation and load the buffer directly.
	// Then adding a new body via AddNetworkBodies triggers evictNBForMemory on existing data.
	//
	// Load 100 entries with 100KB bodies each: 100 * (100000 + 100000 + 300) = ~20MB > 8MB
	capture.mu.Lock()
	now := time.Now()
	for i := 0; i < 100; i++ {
		capture.networkBodies = append(capture.networkBodies, NetworkBody{
			Method:       "GET",
			URL:          "/api/large",
			Status:       200,
			RequestBody:  strings.Repeat("R", 100000),
			ResponseBody: strings.Repeat("S", 100000),
		})
		capture.networkAddedAt = append(capture.networkAddedAt, now)
	}
	capture.mu.Unlock()

	// Verify setup: NB memory exceeds limit
	if capture.GetNetworkBodiesBufferMemory() <= nbBufferMemoryLimit {
		t.Fatalf("setup: expected NB memory > %d, got %d", nbBufferMemoryLimit, capture.GetNetworkBodiesBufferMemory())
	}

	// Adding one more small body triggers evictNBForMemory
	capture.AddNetworkBodies([]NetworkBody{
		{Method: "GET", URL: "/api/trigger", Status: 200, ResponseBody: "ok"},
	})

	// After eviction, the NB memory should be <= nbBufferMemoryLimit
	nbMem := capture.GetNetworkBodiesBufferMemory()
	if nbMem > nbBufferMemoryLimit {
		t.Errorf("expected NB memory <= %d after eviction, got %d", nbBufferMemoryLimit, nbMem)
	}

	// Should have fewer entries than we loaded
	count := capture.GetNetworkBodyCount()
	if count >= 100 {
		t.Errorf("expected eviction to reduce entries below 100, got %d", count)
	}
	if count == 0 {
		t.Error("expected some entries to remain after eviction")
	}
}

// Test: evictNBForMemory with bodies that are just at the limit (no eviction needed).
func TestV4NetworkBodiesEvictForMemory_AtLimit(t *testing.T) {
	capture := setupTestCapture(t)

	// Each body: 1000 + 1000 + 300 = 2300 bytes.
	// nbBufferMemoryLimit = 8MB = 8388608.
	// 8388608 / 2300 = ~3647 entries. But maxNetworkBodies = 100.
	// So 100 entries * 2300 = 230000 bytes, well under limit. No eviction.
	bodies := make([]NetworkBody, 100)
	for i := range bodies {
		bodies[i] = NetworkBody{
			Method:       "GET",
			URL:          "/api/small",
			Status:       200,
			RequestBody:  strings.Repeat("a", 1000),
			ResponseBody: strings.Repeat("b", 1000),
		}
	}

	capture.AddNetworkBodies(bodies)

	count := capture.GetNetworkBodyCount()
	if count != 100 {
		t.Errorf("expected all 100 entries retained (under memory limit), got %d", count)
	}
}

// ============================================
// HandleNetworkBodies: rate limit, body too large, bad JSON, re-check rate limit
// ============================================

// Test: HandleNetworkBodies rejects when rate limited (initial check).
func TestV4HandleNetworkBodies_RateLimited(t *testing.T) {
	capture := setupTestCapture(t)

	// Force circuit breaker open to simulate rate limiting
	capture.mu.Lock()
	capture.circuitOpen = true
	capture.circuitOpenedAt = time.Now()
	capture.circuitReason = "rate_exceeded"
	capture.mu.Unlock()

	body := `{"bodies":[{"method":"GET","url":"/api/test","status":200}]}`
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}

	// Verify response body is the rate limit JSON
	var resp RateLimitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("expected JSON rate limit response, got: %s", rec.Body.String())
	}
	if !resp.CircuitOpen {
		t.Error("expected circuit_open=true in response")
	}
}

// Test: HandleNetworkBodies rejects when request body is too large.
func TestV4HandleNetworkBodies_BodyTooLarge(t *testing.T) {
	capture := setupTestCapture(t)

	// maxPostBodySize = 5MB. Create a body larger than that.
	largePayload := strings.Repeat("x", 6*1024*1024) // 6MB
	req := httptest.NewRequest("POST", "/network-bodies", strings.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", rec.Code)
	}
}

// Test: HandleNetworkBodies rejects malformed JSON with 400.
func TestV4HandleNetworkBodies_BadJSON(t *testing.T) {
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for bad JSON, got %d", rec.Code)
	}
}

// Test: HandleNetworkBodies rejects after recording events pushes over rate limit.
func TestV4HandleNetworkBodies_RateLimitAfterRecording(t *testing.T) {
	capture := setupTestCapture(t)

	// Set the rate window to current time and push event count just below threshold.
	// Then send a batch that pushes it over.
	capture.mu.Lock()
	capture.rateWindowStart = time.Now()
	capture.windowEventCount = rateLimitThreshold - 1
	capture.mu.Unlock()

	// Send a batch with 10 bodies (pushes count to threshold-1+10 = over threshold)
	bodies := make([]map[string]interface{}, 10)
	for i := range bodies {
		bodies[i] = map[string]interface{}{
			"method": "GET",
			"url":    "/api/test",
			"status": 200,
		}
	}
	payload, _ := json.Marshal(map[string]interface{}{"bodies": bodies})

	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleNetworkBodies(rec, req)

	// After recording, CheckRateLimit should return true, yielding 429
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after recording pushes over threshold, got %d", rec.Code)
	}
}

// Test: HandleNetworkBodies succeeds when memory-exceeded flag is set but within rate.
// Memory-exceeded means CheckRateLimit returns true (isMemoryExceeded check).
func TestV4HandleNetworkBodies_MemoryExceeded(t *testing.T) {
	capture := setupTestCapture(t)

	// Set simulated memory above hard limit to trigger memory-exceeded rejection
	capture.SetMemoryUsage(memoryHardLimit + 1)

	body := `{"bodies":[{"method":"GET","url":"/api/test","status":200}]}`
	req := httptest.NewRequest("POST", "/network-bodies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleNetworkBodies(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 when memory exceeded, got %d", rec.Code)
	}
}
