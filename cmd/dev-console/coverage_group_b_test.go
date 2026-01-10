// coverage_group_b_test.go — Coverage group B: api_schema.go, tools.go, queries.go, streaming.go
// Targets 0%-coverage functions to increase overall coverage.
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

// ============================================
// api_schema.go: ObserveWebSocket
// ============================================

func TestCoverageGroupB_ObserveWebSocket_Basic(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.ObserveWebSocket(WebSocketEvent{
		URL:       "wss://echo.example.com/ws",
		Direction: "incoming",
		Data:      `{"type":"ping","action":"heartbeat"}`,
	})
	store.ObserveWebSocket(WebSocketEvent{
		URL:       "wss://echo.example.com/ws",
		Direction: "outgoing",
		Data:      `{"type":"pong"}`,
	})
	store.ObserveWebSocket(WebSocketEvent{
		URL:       "wss://echo.example.com/ws",
		Direction: "incoming",
		Data:      `not json`,
	})

	schema := store.BuildSchema(SchemaFilter{})
	if len(schema.WebSockets) != 1 {
		t.Fatalf("Expected 1 WebSocket schema, got %d", len(schema.WebSockets))
	}
	ws := schema.WebSockets[0]
	if ws.URL != "wss://echo.example.com/ws" {
		t.Errorf("Expected URL wss://echo.example.com/ws, got %s", ws.URL)
	}
	if ws.TotalMessages != 3 {
		t.Errorf("Expected 3 total messages, got %d", ws.TotalMessages)
	}
	if ws.IncomingCount != 2 {
		t.Errorf("Expected 2 incoming, got %d", ws.IncomingCount)
	}
	if ws.OutgoingCount != 1 {
		t.Errorf("Expected 1 outgoing, got %d", ws.OutgoingCount)
	}
	// Should detect "ping", "pong", "heartbeat" message types
	if len(ws.MessageTypes) < 2 {
		t.Errorf("Expected at least 2 message types, got %d: %v", len(ws.MessageTypes), ws.MessageTypes)
	}
}

func TestCoverageGroupB_ObserveWebSocket_EmptyURL(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.ObserveWebSocket(WebSocketEvent{
		URL:       "",
		Direction: "incoming",
		Data:      `{"type":"test"}`,
	})

	schema := store.BuildSchema(SchemaFilter{})
	if len(schema.WebSockets) != 0 {
		t.Errorf("Expected 0 WebSocket schemas for empty URL, got %d", len(schema.WebSockets))
	}
}

func TestCoverageGroupB_ObserveWebSocket_CapEnforced(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	// Fill up to maxWSSchemaConns unique URLs
	for i := 0; i < maxWSSchemaConns+5; i++ {
		store.ObserveWebSocket(WebSocketEvent{
			URL:       "wss://example.com/ws/" + intToString(i),
			Direction: "incoming",
			Data:      `{"type":"test"}`,
		})
	}

	store.mu.RLock()
	count := len(store.wsSchemas)
	store.mu.RUnlock()

	if count > maxWSSchemaConns {
		t.Errorf("Expected at most %d WS schemas, got %d", maxWSSchemaConns, count)
	}
}

// ============================================
// api_schema.go: BuildSchema (with comprehensive data)
// ============================================

func TestCoverageGroupB_BuildSchema_Comprehensive(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	// Observe multiple endpoints with various data
	for i := 0; i < 5; i++ {
		store.Observe(NetworkBody{
			URL:          "https://api.example.com/users/123?page=1&active=true",
			Method:       "GET",
			Status:       200,
			Duration:     50 + i*10,
			ContentType:  "application/json",
			ResponseBody: `{"id":123,"name":"Alice","email":"alice@example.com","created_at":"2024-01-01T00:00:00Z","active":true,"score":98.5,"tags":["admin","user"],"profile":{"avatar":"http://img.example.com/a.jpg"}}`,
		})
	}

	// Observe POST with request body
	for i := 0; i < 3; i++ {
		store.Observe(NetworkBody{
			URL:          "https://api.example.com/users",
			Method:       "POST",
			Status:       201,
			Duration:     100 + i*20,
			ContentType:  "application/json",
			RequestBody:  `{"name":"Bob","email":"bob@example.com","active":true}`,
			ResponseBody: `{"id":456,"name":"Bob"}`,
		})
	}

	// Observe an endpoint with UUID path segment
	store.Observe(NetworkBody{
		URL:          "https://api.example.com/orders/550e8400-e29b-41d4-a716-446655440000/items",
		Method:       "GET",
		Status:       200,
		Duration:     30,
		ContentType:  "application/json",
		ResponseBody: `{"items":[]}`,
	})

	// Observe an error response
	store.Observe(NetworkBody{
		URL:          "https://api.example.com/users/999",
		Method:       "GET",
		Status:       404,
		Duration:     5,
		ContentType:  "text/plain",
		ResponseBody: "Not Found",
	})

	// Observe a 500 error
	store.Observe(NetworkBody{
		URL:      "https://api.example.com/users/123",
		Method:   "GET",
		Status:   500,
		Duration: 200,
	})

	// Build schema with no filter
	schema := store.BuildSchema(SchemaFilter{})
	if schema.Coverage.TotalEndpoints == 0 {
		t.Fatal("Expected endpoints in schema")
	}
	if schema.Coverage.Methods["GET"] == 0 {
		t.Error("Expected GET method in coverage")
	}
	if schema.Coverage.Methods["POST"] == 0 {
		t.Error("Expected POST method in coverage")
	}
	if schema.Coverage.ErrorRate <= 0 {
		t.Error("Expected non-zero error rate due to 404/500 responses")
	}
	if schema.Coverage.AvgResponseMs <= 0 {
		t.Error("Expected non-zero average response time")
	}

	// Check endpoint details
	for _, ep := range schema.Endpoints {
		if ep.Method == "GET" && strings.Contains(ep.PathPattern, "/users/{id}") {
			// Should have timing stats
			if ep.Timing.Avg <= 0 {
				t.Error("Expected non-zero average timing for GET /users/{id}")
			}
			if ep.Timing.P50 <= 0 {
				t.Error("Expected non-zero P50 timing")
			}
			if ep.Timing.P95 <= 0 {
				t.Error("Expected non-zero P95 timing")
			}
			if ep.Timing.Max <= 0 {
				t.Error("Expected non-zero Max timing")
			}
			// Should have path params
			if len(ep.PathParams) == 0 {
				t.Error("Expected path params for /users/{id}")
			}
			// Should have query params
			if len(ep.QueryParams) == 0 {
				t.Error("Expected query params (page, active)")
			}
		}
		if ep.Method == "POST" && ep.PathPattern == "/users" {
			// Should have request shape
			if ep.RequestShape == nil {
				t.Error("Expected request shape for POST /users")
			} else {
				if _, ok := ep.RequestShape.Fields["name"]; !ok {
					t.Error("Expected 'name' field in request shape")
				}
				if _, ok := ep.RequestShape.Fields["email"]; !ok {
					t.Error("Expected 'email' field in request shape")
				}
			}
		}
	}
}

func TestCoverageGroupB_BuildSchema_URLFilter(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.Observe(NetworkBody{URL: "https://api.example.com/users", Method: "GET", Status: 200, Duration: 10})
	store.Observe(NetworkBody{URL: "https://cdn.example.com/style.css", Method: "GET", Status: 200, Duration: 5})

	schema := store.BuildSchema(SchemaFilter{URLFilter: "/users"})
	if schema.Coverage.TotalEndpoints != 1 {
		t.Errorf("Expected 1 endpoint with URL filter, got %d", schema.Coverage.TotalEndpoints)
	}
}

func TestCoverageGroupB_BuildSchema_MinObservations(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.Observe(NetworkBody{URL: "https://api.example.com/rare", Method: "GET", Status: 200})
	for i := 0; i < 5; i++ {
		store.Observe(NetworkBody{URL: "https://api.example.com/common", Method: "GET", Status: 200})
	}

	schema := store.BuildSchema(SchemaFilter{MinObservations: 3})
	if schema.Coverage.TotalEndpoints != 1 {
		t.Errorf("Expected 1 endpoint with min_observations=3, got %d", schema.Coverage.TotalEndpoints)
	}
}

// ============================================
// api_schema.go: BuildOpenAPIStub
// ============================================

func TestCoverageGroupB_BuildOpenAPIStub(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	// Observe with request body, response body, query params, and path params
	for i := 0; i < 3; i++ {
		store.Observe(NetworkBody{
			URL:          "https://api.example.com/users/42?sort=name&active=true",
			Method:       "GET",
			Status:       200,
			Duration:     50,
			ContentType:  "application/json",
			ResponseBody: `{"id":42,"name":"Alice","active":true}`,
		})
	}
	store.Observe(NetworkBody{
		URL:          "https://api.example.com/users",
		Method:       "POST",
		Status:       201,
		Duration:     100,
		ContentType:  "application/json",
		RequestBody:  `{"name":"Bob","email":"bob@example.com"}`,
		ResponseBody: `{"id":99,"name":"Bob"}`,
	})
	// Observe an endpoint without response body (just status code)
	store.Observe(NetworkBody{
		URL:    "https://api.example.com/health",
		Method: "GET",
		Status: 204,
	})

	stub := store.BuildOpenAPIStub(SchemaFilter{})

	if !strings.Contains(stub, "openapi: \"3.0.0\"") {
		t.Error("Expected OpenAPI version header")
	}
	if !strings.Contains(stub, "paths:") {
		t.Error("Expected paths section")
	}
	if !strings.Contains(stub, "get:") {
		t.Error("Expected GET method in stub")
	}
	if !strings.Contains(stub, "post:") {
		t.Error("Expected POST method in stub")
	}
	if !strings.Contains(stub, "responses:") {
		t.Error("Expected responses section")
	}
	if !strings.Contains(stub, "requestBody:") {
		t.Error("Expected requestBody section for POST")
	}
	if !strings.Contains(stub, "parameters:") {
		t.Error("Expected parameters section")
	}
	if !strings.Contains(stub, "\"200\"") || !strings.Contains(stub, "\"201\"") {
		t.Error("Expected status code 200 and 201 in responses")
	}
}

// ============================================
// api_schema.go: majorityType
// ============================================

func TestCoverageGroupB_MajorityType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]int
		expected string
	}{
		{"single type", map[string]int{"string": 5}, "string"},
		{"majority integer", map[string]int{"integer": 10, "string": 3}, "integer"},
		{"majority boolean", map[string]int{"boolean": 7, "number": 2, "string": 1}, "boolean"},
		{"empty map defaults to string", map[string]int{}, "string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := majorityType(tt.input)
			if result != tt.expected {
				t.Errorf("majorityType(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================
// api_schema.go: computeTimingStats
// ============================================

func TestCoverageGroupB_ComputeTimingStats(t *testing.T) {
	t.Parallel()

	t.Run("empty", func(t *testing.T) {
		stats := computeTimingStats(nil)
		if stats.Avg != 0 || stats.P50 != 0 || stats.P95 != 0 || stats.Max != 0 {
			t.Errorf("Expected all zeros for empty latencies, got %+v", stats)
		}
	})

	t.Run("single value", func(t *testing.T) {
		stats := computeTimingStats([]float64{42.0})
		if stats.Avg != 42.0 {
			t.Errorf("Expected avg=42, got %f", stats.Avg)
		}
		if stats.P50 != 42.0 {
			t.Errorf("Expected p50=42, got %f", stats.P50)
		}
		if stats.Max != 42.0 {
			t.Errorf("Expected max=42, got %f", stats.Max)
		}
	})

	t.Run("multiple values", func(t *testing.T) {
		latencies := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
		stats := computeTimingStats(latencies)
		if stats.Avg != 55.0 {
			t.Errorf("Expected avg=55, got %f", stats.Avg)
		}
		if stats.Max != 100.0 {
			t.Errorf("Expected max=100, got %f", stats.Max)
		}
		if stats.P50 <= 0 {
			t.Errorf("Expected positive P50, got %f", stats.P50)
		}
		if stats.P95 <= 0 {
			t.Errorf("Expected positive P95, got %f", stats.P95)
		}
	})
}

// ============================================
// api_schema.go: percentile
// ============================================

func TestCoverageGroupB_Percentile(t *testing.T) {
	t.Parallel()

	t.Run("empty slice", func(t *testing.T) {
		result := percentile(nil, 0.5)
		if result != 0 {
			t.Errorf("Expected 0 for empty slice, got %f", result)
		}
	})

	t.Run("single element", func(t *testing.T) {
		result := percentile([]float64{42.0}, 0.5)
		if result != 42.0 {
			t.Errorf("Expected 42.0, got %f", result)
		}
	})

	t.Run("exact index", func(t *testing.T) {
		sorted := []float64{10, 20, 30, 40, 50}
		result := percentile(sorted, 0.5) // index = 0.5 * 4 = 2.0 -> sorted[2] = 30
		if result != 30.0 {
			t.Errorf("Expected 30.0, got %f", result)
		}
	})

	t.Run("interpolated", func(t *testing.T) {
		sorted := []float64{10, 20, 30, 40}
		result := percentile(sorted, 0.5) // index = 0.5 * 3 = 1.5 -> interpolate between 20 and 30
		if result != 25.0 {
			t.Errorf("Expected 25.0, got %f", result)
		}
	})
}

// ============================================
// api_schema.go: detectAuthPattern
// ============================================

func TestCoverageGroupB_DetectAuthPattern_WithAuthEndpoint(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.Observe(NetworkBody{URL: "https://api.example.com/auth/login", Method: "POST", Status: 200})
	store.Observe(NetworkBody{URL: "https://api.example.com/health", Method: "GET", Status: 200})
	store.Observe(NetworkBody{URL: "https://api.example.com/api/users", Method: "GET", Status: 200})

	schema := store.BuildSchema(SchemaFilter{})
	if schema.AuthPattern == nil {
		t.Fatal("Expected auth pattern to be detected with /auth/login endpoint")
	}
	if schema.AuthPattern.Type != "bearer" {
		t.Errorf("Expected bearer type, got %s", schema.AuthPattern.Type)
	}
	if schema.AuthPattern.Header != "Authorization" {
		t.Errorf("Expected Authorization header, got %s", schema.AuthPattern.Header)
	}
	// /auth/login and /health should be public paths
	if len(schema.AuthPattern.PublicPaths) == 0 {
		t.Error("Expected public paths to include auth and health endpoints")
	}
}

func TestCoverageGroupB_DetectAuthPattern_With401(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.Observe(NetworkBody{URL: "https://api.example.com/api/secret", Method: "GET", Status: 401})
	store.Observe(NetworkBody{URL: "https://api.example.com/api/data", Method: "GET", Status: 200})

	schema := store.BuildSchema(SchemaFilter{})
	if schema.AuthPattern == nil {
		t.Fatal("Expected auth pattern to be detected with 401 response")
	}
}

func TestCoverageGroupB_DetectAuthPattern_NoAuth(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.Observe(NetworkBody{URL: "https://api.example.com/api/data", Method: "GET", Status: 200})
	store.Observe(NetworkBody{URL: "https://api.example.com/api/items", Method: "GET", Status: 200})

	schema := store.BuildSchema(SchemaFilter{})
	if schema.AuthPattern != nil {
		t.Error("Expected no auth pattern when no auth endpoints or 401 responses")
	}
}

// ============================================
// api_schema.go: buildEndpoint, buildQueryParams, buildFields (via BuildSchema)
// ============================================

func TestCoverageGroupB_BuildEndpoint_QueryParamTypes(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	// Observe with various query param types
	for i := 0; i < 5; i++ {
		store.Observe(NetworkBody{
			URL:    "https://api.example.com/search?page=" + intToString(i+1) + "&active=true&q=test",
			Method: "GET",
			Status: 200,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	for _, ep := range schema.Endpoints {
		if ep.PathPattern == "/search" {
			for _, qp := range ep.QueryParams {
				switch qp.Name {
				case "page":
					if qp.Type != "integer" {
						t.Errorf("Expected page param type 'integer', got '%s'", qp.Type)
					}
				case "active":
					if qp.Type != "boolean" {
						t.Errorf("Expected active param type 'boolean', got '%s'", qp.Type)
					}
				case "q":
					if qp.Type != "string" {
						t.Errorf("Expected q param type 'string', got '%s'", qp.Type)
					}
				}
				// page and active should be required (present in 100% of observations)
				if qp.Name == "page" && !qp.Required {
					t.Error("Expected page param to be required")
				}
			}
			return
		}
	}
	t.Error("Did not find /search endpoint")
}

func TestCoverageGroupB_BuildFields_TypeInference(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	for i := 0; i < 5; i++ {
		store.Observe(NetworkBody{
			URL:          "https://api.example.com/data",
			Method:       "GET",
			Status:       200,
			ContentType:  "application/json",
			ResponseBody: `{"count":42,"name":"test","active":true,"score":3.14,"items":[],"meta":{},"id":null}`,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	for _, ep := range schema.Endpoints {
		if ep.PathPattern == "/data" {
			shape := ep.ResponseShapes[200]
			if shape == nil {
				t.Fatal("Expected response shape for status 200")
			}
			checks := map[string]string{
				"count":  "integer",
				"name":   "string",
				"active": "boolean",
				"score":  "number",
				"items":  "array",
				"meta":   "object",
				"id":     "null",
			}
			for field, expectedType := range checks {
				fs, ok := shape.Fields[field]
				if !ok {
					t.Errorf("Expected field '%s' in response shape", field)
					continue
				}
				if fs.Type != expectedType {
					t.Errorf("Field '%s': expected type '%s', got '%s'", field, expectedType, fs.Type)
				}
				// All fields present in all 5 observations should be required
				if !fs.Required {
					t.Errorf("Field '%s': expected required=true", field)
				}
			}
			return
		}
	}
	t.Error("Did not find /data endpoint")
}

// ============================================
// api_schema.go: inferTypeAndFormat + inferStringFormat
// ============================================

func TestCoverageGroupB_InferStringFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"550e8400-e29b-41d4-a716-446655440000", "uuid"},
		{"2024-01-01T00:00:00Z", "datetime"},
		{"alice@example.com", "email"},
		{"https://example.com/page", "url"},
		{"http://example.com", "url"},
		{"just a string", ""},
		{"12345", ""},
	}

	for _, tt := range tests {
		result := inferStringFormat(tt.input)
		if result != tt.expected {
			t.Errorf("inferStringFormat(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCoverageGroupB_InferTypeAndFormat(t *testing.T) {
	t.Parallel()

	typ, fmt := inferTypeAndFormat(nil)
	if typ != "null" || fmt != "" {
		t.Errorf("nil: expected null/'', got %s/%s", typ, fmt)
	}

	typ, fmt = inferTypeAndFormat(true)
	if typ != "boolean" || fmt != "" {
		t.Errorf("bool: expected boolean/'', got %s/%s", typ, fmt)
	}

	typ, fmt = inferTypeAndFormat(42.0)
	if typ != "integer" || fmt != "" {
		t.Errorf("42.0: expected integer/'', got %s/%s", typ, fmt)
	}

	typ, fmt = inferTypeAndFormat(3.14)
	if typ != "number" || fmt != "" {
		t.Errorf("3.14: expected number/'', got %s/%s", typ, fmt)
	}

	typ, fmt = inferTypeAndFormat("hello")
	if typ != "string" {
		t.Errorf("string: expected string, got %s", typ)
	}

	typ, fmt = inferTypeAndFormat([]interface{}{1, 2})
	if typ != "array" || fmt != "" {
		t.Errorf("array: expected array/'', got %s/%s", typ, fmt)
	}

	typ, fmt = inferTypeAndFormat(map[string]interface{}{"a": 1})
	if typ != "object" || fmt != "" {
		t.Errorf("object: expected object/'', got %s/%s", typ, fmt)
	}
}

// ============================================
// api_schema.go: parameterizePath (hex hash)
// ============================================

func TestCoverageGroupB_ParameterizePath_HexHash(t *testing.T) {
	t.Parallel()

	result := parameterizePath("/commits/abcdef0123456789abcdef01")
	if !strings.Contains(result, "{hash}") {
		t.Errorf("Expected {hash} in parameterized path, got %s", result)
	}
}

// ============================================
// tools.go: prependTrackingWarning
// ============================================

func TestCoverageGroupB_PrependTrackingWarning(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Create a simple MCP response
	originalResp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  mcpTextResponse("Original content"),
	}

	warningHint := "WARNING: No tab is being tracked."
	result := mcp.toolHandler.prependTrackingWarning(originalResp, warningHint)

	var toolResult MCPToolResult
	if err := json.Unmarshal(result.Result, &toolResult); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if len(toolResult.Content) < 2 {
		t.Fatalf("Expected at least 2 content blocks, got %d", len(toolResult.Content))
	}

	if toolResult.Content[0].Text != warningHint {
		t.Errorf("Expected warning as first block, got %q", toolResult.Content[0].Text)
	}
	if toolResult.Content[1].Text != "Original content" {
		t.Errorf("Expected original content as second block, got %q", toolResult.Content[1].Text)
	}
}

// ============================================
// tools.go: toolConfigureStreamingWrapper
// ============================================

func TestCoverageGroupB_ToolConfigureStreamingWrapper(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}

	// Test with streaming_action -> action rewriting
	args := json.RawMessage(`{"streaming_action":"enable","events":["errors"]}`)
	resp := mcp.toolHandler.toolConfigureStreamingWrapper(req, args)

	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}

	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "enabled") {
		t.Errorf("Expected streaming to be enabled, got: %s", text)
	}

	// Test with direct action
	args = json.RawMessage(`{"action":"status"}`)
	resp = mcp.toolHandler.toolConfigureStreamingWrapper(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}

	// Test with invalid JSON
	args = json.RawMessage(`{invalid}`)
	resp = mcp.toolHandler.toolConfigureStreamingWrapper(req, args)
	text = extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "Invalid JSON") {
		t.Errorf("Expected invalid JSON error, got: %s", text)
	}
}

// ============================================
// tools.go: toolValidateAPI
// ============================================

func TestCoverageGroupB_ToolValidateAPI(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add some network bodies to analyze
	capture.AddNetworkBodies([]NetworkBody{
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, ContentType: "application/json", ResponseBody: `{"id":1,"name":"Alice"}`},
		{URL: "https://api.example.com/users", Method: "GET", Status: 200, ContentType: "application/json", ResponseBody: `{"id":2,"name":"Bob"}`},
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}

	// Test "analyze" operation
	args := json.RawMessage(`{"operation":"analyze"}`)
	resp := mcp.toolHandler.toolValidateAPI(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error for analyze, got %v", resp.Error)
	}

	// Test "report" operation
	args = json.RawMessage(`{"operation":"report"}`)
	resp = mcp.toolHandler.toolValidateAPI(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error for report, got %v", resp.Error)
	}

	// Test "clear" operation
	args = json.RawMessage(`{"operation":"clear"}`)
	resp = mcp.toolHandler.toolValidateAPI(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error for clear, got %v", resp.Error)
	}
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "cleared") {
		t.Errorf("Expected 'cleared' in response, got: %s", text)
	}

	// Test invalid operation
	args = json.RawMessage(`{"operation":"invalid"}`)
	resp = mcp.toolHandler.toolValidateAPI(req, args)
	text = extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "operation") {
		t.Errorf("Expected error about operation parameter, got: %s", text)
	}
}

// ============================================
// tools.go: toolGenerateSRI
// ============================================

func TestCoverageGroupB_ToolGenerateSRI(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}

	// Call with no network bodies (empty)
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolGenerateSRI(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}
}

// ============================================
// tools.go: toolObserveCommandResult
// ============================================

func TestCoverageGroupB_ToolObserveCommandResult(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}

	// Test missing correlation_id
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolObserveCommandResult(req, args)
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("Expected missing correlation_id error, got: %s", text)
	}

	// Test with non-existent correlation_id
	args = json.RawMessage(`{"correlation_id":"corr-nonexistent"}`)
	resp = mcp.toolHandler.toolObserveCommandResult(req, args)
	text = extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "not found") {
		t.Errorf("Expected 'not found' error, got: %s", text)
	}

	// Store a command result and retrieve it
	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-test-123",
		Status:        "complete",
		Result:        json.RawMessage(`{"success":true}`),
	})

	args = json.RawMessage(`{"correlation_id":"corr-test-123"}`)
	resp = mcp.toolHandler.toolObserveCommandResult(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}
	text = extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "corr-test-123") {
		t.Errorf("Expected correlation ID in response, got: %s", text)
	}
}

// ============================================
// tools.go: toolObservePendingCommands
// ============================================

func TestCoverageGroupB_ToolObservePendingCommands(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Store some commands in various states
	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-pending-1",
		Status:        "pending",
	})
	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-complete-1",
		Status:        "complete",
		Result:        json.RawMessage(`{"done":true}`),
	})

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolObservePendingCommands(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}

	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "Pending:") {
		t.Errorf("Expected 'Pending:' in response summary, got: %s", text)
	}
}

// ============================================
// tools.go: toolObserveFailedCommands
// ============================================

func TestCoverageGroupB_ToolObserveFailedCommands_Empty(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolObserveFailedCommands(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "No failed commands") {
		t.Errorf("Expected 'No failed commands' message, got: %s", text)
	}
}

func TestCoverageGroupB_ToolObserveFailedCommands_WithFailed(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Directly add a failed command
	capture.resultsMu.Lock()
	capture.failedCommands = append(capture.failedCommands, &CommandResult{
		CorrelationID: "corr-failed-1",
		Status:        "expired",
		Error:         "Result expired after 60s",
		CompletedAt:   time.Now(),
	})
	capture.resultsMu.Unlock()

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolObserveFailedCommands(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "1 recent failed") {
		t.Errorf("Expected '1 recent failed' in response, got: %s", text)
	}
}

// ============================================
// tools.go: toolInteract (missing action, unknown action)
// ============================================

func TestCoverageGroupB_ToolInteract_MissingAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolInteract(req, args)
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "action") {
		t.Errorf("Expected missing action error, got: %s", text)
	}
}

func TestCoverageGroupB_ToolInteract_UnknownAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"action":"fly_to_moon"}`)
	resp := mcp.toolHandler.toolInteract(req, args)
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "Unknown interact action") {
		t.Errorf("Expected unknown action error, got: %s", text)
	}
}

func TestCoverageGroupB_ToolInteract_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{invalid}`)
	resp := mcp.toolHandler.toolInteract(req, args)
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "Invalid JSON") {
		t.Errorf("Expected Invalid JSON error, got: %s", text)
	}
}

// ============================================
// tools.go: checkTrackingStatus
// ============================================

func TestCoverageGroupB_CheckTrackingStatus_NotReported(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// No tracking update has been reported yet
	enabled, hint := mcp.toolHandler.checkTrackingStatus()
	if !enabled {
		t.Error("Expected enabled=true when extension hasn't reported yet")
	}
	if hint != "" {
		t.Errorf("Expected empty hint, got: %s", hint)
	}
}

func TestCoverageGroupB_CheckTrackingStatus_Disabled(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Simulate extension reported tracking as disabled
	capture.mu.Lock()
	capture.trackingUpdated = time.Now()
	capture.trackingEnabled = false
	capture.mu.Unlock()

	enabled, hint := mcp.toolHandler.checkTrackingStatus()
	if enabled {
		t.Error("Expected enabled=false when tracking is disabled")
	}
	if !strings.Contains(hint, "WARNING") {
		t.Errorf("Expected WARNING in hint, got: %s", hint)
	}
}

func TestCoverageGroupB_CheckTrackingStatus_Enabled(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Simulate extension reported tracking as enabled
	capture.mu.Lock()
	capture.trackingUpdated = time.Now()
	capture.trackingEnabled = true
	capture.mu.Unlock()

	enabled, hint := mcp.toolHandler.checkTrackingStatus()
	if !enabled {
		t.Error("Expected enabled=true when tracking is active")
	}
	if hint != "" {
		t.Errorf("Expected empty hint when tracking is active, got: %s", hint)
	}
}

// ============================================
// tools.go: toolVerifyFix
// ============================================

func TestCoverageGroupB_ToolVerifyFix(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}

	// Test with "start" action
	args := json.RawMessage(`{"action":"start","description":"Fix login bug"}`)
	resp := mcp.toolHandler.toolVerifyFix(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}
}

// ============================================
// queries.go: GetPollingLog
// ============================================

func TestCoverageGroupB_GetPollingLog_Empty(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	log := capture.GetPollingLog()
	if len(log) != 0 {
		t.Errorf("Expected empty polling log, got %d entries", len(log))
	}
}

func TestCoverageGroupB_GetPollingLog_WithEntries(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Add some polling log entries
	capture.mu.Lock()
	capture.logPollingActivity(PollingLogEntry{
		Timestamp:  time.Now(),
		Endpoint:   "pending-queries",
		Method:     "GET",
		SessionID:  "session-1",
		QueryCount: 2,
	})
	capture.logPollingActivity(PollingLogEntry{
		Timestamp: time.Now(),
		Endpoint:  "settings",
		Method:    "POST",
		SessionID: "session-1",
	})
	capture.mu.Unlock()

	log := capture.GetPollingLog()
	if len(log) != 2 {
		t.Errorf("Expected 2 polling log entries, got %d", len(log))
	}
}

// ============================================
// queries.go: GetHTTPDebugLog
// ============================================

func TestCoverageGroupB_GetHTTPDebugLog_Empty(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	log := capture.GetHTTPDebugLog()
	if len(log) != 0 {
		t.Errorf("Expected empty HTTP debug log, got %d entries", len(log))
	}
}

func TestCoverageGroupB_GetHTTPDebugLog_WithEntries(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.mu.Lock()
	capture.logHTTPDebugEntry(HTTPDebugEntry{
		Timestamp:      time.Now(),
		Endpoint:       "/pending-queries",
		Method:         "GET",
		SessionID:      "session-1",
		ResponseStatus: 200,
		DurationMs:     5,
	})
	capture.logHTTPDebugEntry(HTTPDebugEntry{
		Timestamp:      time.Now(),
		Endpoint:       "/console-log",
		Method:         "POST",
		ResponseStatus: 200,
		DurationMs:     3,
		RequestBody:    `{"entries":[]}`,
	})
	capture.mu.Unlock()

	log := capture.GetHTTPDebugLog()
	if len(log) != 2 {
		t.Errorf("Expected 2 HTTP debug log entries, got %d", len(log))
	}
}

// ============================================
// queries.go: WaitForResultLegacy
// ============================================

func TestCoverageGroupB_WaitForResultLegacy_Timeout(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#test"}`),
	})

	_, err := capture.WaitForResultLegacy(id, 100*time.Millisecond)
	if err == nil {
		t.Fatal("Expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}

func TestCoverageGroupB_WaitForResultLegacy_Success(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "dom",
		Params: json.RawMessage(`{"selector":"#test"}`),
	})

	// Set result in a separate goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		capture.SetQueryResult(id, json.RawMessage(`{"found":true}`))
	}()

	result, err := capture.WaitForResultLegacy(id, 2*time.Second)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(string(result), "found") {
		t.Errorf("Expected result to contain 'found', got: %s", string(result))
	}
}

// ============================================
// queries.go: HandleStateResult, HandleExecuteResult, HandleHighlightResult
// ============================================

func TestCoverageGroupB_HandleStateResult(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "state",
		Params: json.RawMessage(`{"action":"save"}`),
	})

	body := `{"id":"` + id + `","result":{"saved":true}}`
	req := httptest.NewRequest("POST", "/state-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleStateResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestCoverageGroupB_HandleExecuteResult(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "execute",
		Params: json.RawMessage(`{"code":"return 1+1"}`),
	})

	body := `{"id":"` + id + `","result":{"value":2}}`
	req := httptest.NewRequest("POST", "/execute-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleExecuteResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestCoverageGroupB_HandleHighlightResult(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	id := capture.CreatePendingQuery(PendingQuery{
		Type:   "highlight",
		Params: json.RawMessage(`{"selector":"#btn"}`),
	})

	body := `{"id":"` + id + `","result":{"highlighted":true}}`
	req := httptest.NewRequest("POST", "/highlight-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleHighlightResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}
}

func TestCoverageGroupB_HandleResult_CorrelationID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Test the correlation_id path
	body := `{"correlation_id":"corr-async-1","status":"complete","result":{"done":true}}`
	req := httptest.NewRequest("POST", "/state-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleStateResult(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rec.Code)
	}

	// Verify the result was stored
	result := capture.GetCommandResult("corr-async-1")
	if result == nil {
		t.Fatal("Expected command result to be stored")
	}
	if result.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", result.Status)
	}
}

func TestCoverageGroupB_HandleResult_BadJSON(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	req := httptest.NewRequest("POST", "/execute-result", bytes.NewBufferString(`{invalid}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleExecuteResult(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for bad JSON, got %d", rec.Code)
	}
}

func TestCoverageGroupB_HandleResult_MissingID(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Neither id nor correlation_id
	body := `{"result":{"value":1}}`
	req := httptest.NewRequest("POST", "/state-result", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	capture.HandleStateResult(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing ID, got %d", rec.Code)
	}
}

// ============================================
// queries.go: GetCommandResult
// ============================================

func TestCoverageGroupB_GetCommandResult_NotFound(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	result := capture.GetCommandResult("nonexistent")
	if result != nil {
		t.Error("Expected nil for nonexistent correlation ID")
	}
}

func TestCoverageGroupB_GetCommandResult_Found(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-found-1",
		Status:        "complete",
		Result:        json.RawMessage(`{"value":42}`),
	})

	result := capture.GetCommandResult("corr-found-1")
	if result == nil {
		t.Fatal("Expected result for existing correlation ID")
	}
	if result.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", result.Status)
	}
}

// ============================================
// queries.go: GetPendingCommands
// ============================================

func TestCoverageGroupB_GetPendingCommands_AllStates(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-p1",
		Status:        "pending",
	})
	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-c1",
		Status:        "complete",
		Result:        json.RawMessage(`{"done":true}`),
	})
	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-t1",
		Status:        "timeout",
		Error:         "timed out",
	})

	// Add a failed command directly
	capture.resultsMu.Lock()
	capture.failedCommands = append(capture.failedCommands, &CommandResult{
		CorrelationID: "corr-f1",
		Status:        "expired",
		Error:         "expired",
	})
	capture.resultsMu.Unlock()

	pending, completed, failed := capture.GetPendingCommands()
	if len(pending) != 1 {
		t.Errorf("Expected 1 pending, got %d", len(pending))
	}
	if len(completed) != 2 { // "complete" and "timeout" are both in completed
		t.Errorf("Expected 2 completed (includes timeout), got %d", len(completed))
	}
	if len(failed) != 1 {
		t.Errorf("Expected 1 failed, got %d", len(failed))
	}
}

// ============================================
// queries.go: GetFailedCommands
// ============================================

func TestCoverageGroupB_GetFailedCommands(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Initially empty
	failed := capture.GetFailedCommands()
	if len(failed) != 0 {
		t.Errorf("Expected 0 failed commands initially, got %d", len(failed))
	}

	// Add some failed commands
	capture.resultsMu.Lock()
	capture.failedCommands = append(capture.failedCommands,
		&CommandResult{CorrelationID: "f1", Status: "expired", Error: "expired 1"},
		&CommandResult{CorrelationID: "f2", Status: "expired", Error: "expired 2"},
	)
	capture.resultsMu.Unlock()

	failed = capture.GetFailedCommands()
	if len(failed) != 2 {
		t.Errorf("Expected 2 failed commands, got %d", len(failed))
	}
}

// ============================================
// queries.go: GetLastPollAt
// ============================================

func TestCoverageGroupB_GetLastPollAt(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Initially zero
	lastPoll := capture.GetLastPollAt()
	if !lastPoll.IsZero() {
		t.Error("Expected zero time initially for GetLastPollAt")
	}

	// Simulate a poll
	now := time.Now()
	capture.mu.Lock()
	capture.lastPollAt = now
	capture.mu.Unlock()

	lastPoll = capture.GetLastPollAt()
	if lastPoll.IsZero() {
		t.Error("Expected non-zero time after poll")
	}
	if !lastPoll.Equal(now) {
		t.Errorf("Expected lastPollAt=%v, got %v", now, lastPoll)
	}
}

// ============================================
// queries.go: SetCommandResult (pending status)
// ============================================

func TestCoverageGroupB_SetCommandResult_PendingThenComplete(t *testing.T) {
	t.Parallel()
	capture := setupTestCapture(t)

	// Set as pending first
	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-lifecycle-1",
		Status:        "pending",
	})

	result := capture.GetCommandResult("corr-lifecycle-1")
	if result == nil {
		t.Fatal("Expected pending result")
	}
	if result.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", result.Status)
	}

	// Now complete it
	capture.SetCommandResult(CommandResult{
		CorrelationID: "corr-lifecycle-1",
		Status:        "complete",
		Result:        json.RawMessage(`{"value":99}`),
	})

	result = capture.GetCommandResult("corr-lifecycle-1")
	if result == nil {
		t.Fatal("Expected completed result")
	}
	if result.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", result.Status)
	}
}

// ============================================
// streaming.go: toolConfigureStreaming
// ============================================

func TestCoverageGroupB_ToolConfigureStreaming_MissingAction(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{}`)
	resp := mcp.toolHandler.toolConfigureStreaming(req, args)
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "action") {
		t.Errorf("Expected missing action error, got: %s", text)
	}
}

func TestCoverageGroupB_ToolConfigureStreaming_Enable(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"action":"enable","events":["errors","ci"],"throttle_seconds":10,"severity_min":"error"}`)
	resp := mcp.toolHandler.toolConfigureStreaming(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "enabled") {
		t.Errorf("Expected 'enabled' in response, got: %s", text)
	}
}

func TestCoverageGroupB_ToolConfigureStreaming_Disable(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}

	// Enable first
	args := json.RawMessage(`{"action":"enable"}`)
	mcp.toolHandler.toolConfigureStreaming(req, args)

	// Then disable
	args = json.RawMessage(`{"action":"disable"}`)
	resp := mcp.toolHandler.toolConfigureStreaming(req, args)
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "disabled") {
		t.Errorf("Expected 'disabled' in response, got: %s", text)
	}
}

func TestCoverageGroupB_ToolConfigureStreaming_Status(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{"action":"status"}`)
	resp := mcp.toolHandler.toolConfigureStreaming(req, args)
	if resp.Error != nil {
		t.Fatalf("Expected no error, got %v", resp.Error)
	}
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "config") {
		t.Errorf("Expected 'config' in status response, got: %s", text)
	}
}

func TestCoverageGroupB_ToolConfigureStreaming_InvalidJSON(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
	}
	args := json.RawMessage(`{bad json}`)
	resp := mcp.toolHandler.toolConfigureStreaming(req, args)
	text := extractMCPTextFromResp(t, resp)
	if !strings.Contains(text, "Invalid JSON") {
		t.Errorf("Expected Invalid JSON error, got: %s", text)
	}
}

// ============================================
// api_schema.go: Observe with response shape tracking for non-JSON
// ============================================

func TestCoverageGroupB_Observe_NonJSONStatusTracking(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	// Observe a non-JSON response - endpoint should still be tracked
	store.Observe(NetworkBody{
		URL:         "https://api.example.com/file",
		Method:      "GET",
		Status:      200,
		ContentType: "text/html",
	})

	schema := store.BuildSchema(SchemaFilter{})
	if len(schema.Endpoints) == 0 {
		t.Fatal("Expected at least 1 endpoint")
	}

	found := false
	for _, ep := range schema.Endpoints {
		if ep.PathPattern == "/file" {
			found = true
			if ep.ObservationCount != 1 {
				t.Errorf("Expected 1 observation, got %d", ep.ObservationCount)
			}
			// Note: ResponseShapes won't contain status 200 because buildEndpoint
			// only includes shapes with fields (non-JSON has no parsed fields).
			// But the accumulator still tracks the status code internally.
			break
		}
	}
	if !found {
		t.Error("Did not find /file endpoint")
	}
}

// ============================================
// api_schema.go: Observe endpoint cap
// ============================================

func TestCoverageGroupB_Observe_EndpointCap(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	// Add more than maxSchemaEndpoints unique endpoints
	for i := 0; i < maxSchemaEndpoints+10; i++ {
		store.Observe(NetworkBody{
			URL:    "https://api.example.com/endpoint-" + intToString(i),
			Method: "GET",
			Status: 200,
		})
	}

	store.mu.RLock()
	count := len(store.accumulators)
	store.mu.RUnlock()

	if count > maxSchemaEndpoints {
		t.Errorf("Expected at most %d endpoints, got %d", maxSchemaEndpoints, count)
	}
}

// ============================================
// api_schema.go: isJSONContentType
// ============================================

func TestCoverageGroupB_IsJSONContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bool
	}{
		{"application/json", true},
		{"application/json; charset=utf-8", true},
		{"text/html", false},
		{"text/plain", false},
		{"", true}, // empty content type defaults to trying JSON
	}

	for _, tt := range tests {
		result := isJSONContentType(tt.input)
		if result != tt.expected {
			t.Errorf("isJSONContentType(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

// ============================================
// api_schema.go: Observe with bad URL
// ============================================

func TestCoverageGroupB_Observe_BadURL(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	// Observe with invalid URL - should not panic
	store.Observe(NetworkBody{
		URL:    "://invalid",
		Method: "GET",
		Status: 200,
	})

	// Should still work without crashing
	schema := store.BuildSchema(SchemaFilter{})
	_ = schema
}

// ============================================
// api_schema.go: Observe request body that's not JSON
// ============================================

func TestCoverageGroupB_Observe_NonJSONRequestBody(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		URL:         "https://api.example.com/upload",
		Method:      "POST",
		Status:      200,
		ContentType: "application/json",
		RequestBody: "not-valid-json",
	})

	schema := store.BuildSchema(SchemaFilter{})
	for _, ep := range schema.Endpoints {
		if ep.PathPattern == "/upload" {
			if ep.RequestShape != nil {
				t.Error("Expected no request shape for invalid JSON body")
			}
			return
		}
	}
}

// ============================================
// api_schema.go: BuildSchema with WebSockets in output
// ============================================

func TestCoverageGroupB_BuildSchema_WithWebSockets(t *testing.T) {
	t.Parallel()
	store := NewSchemaStore()

	store.ObserveWebSocket(WebSocketEvent{
		URL:       "wss://ws.example.com/chat",
		Direction: "incoming",
		Data:      `{"type":"message","text":"hello"}`,
	})
	store.ObserveWebSocket(WebSocketEvent{
		URL:       "wss://ws.example.com/chat",
		Direction: "outgoing",
		Data:      `{"type":"typing"}`,
	})

	schema := store.BuildSchema(SchemaFilter{})
	if len(schema.WebSockets) != 1 {
		t.Fatalf("Expected 1 WebSocket entry, got %d", len(schema.WebSockets))
	}
	ws := schema.WebSockets[0]
	if ws.IncomingCount != 1 || ws.OutgoingCount != 1 {
		t.Errorf("Expected 1 incoming, 1 outgoing, got %d/%d", ws.IncomingCount, ws.OutgoingCount)
	}
	// MessageTypes should contain "message" and "typing"
	found := map[string]bool{}
	for _, mt := range ws.MessageTypes {
		found[mt] = true
	}
	if !found["message"] || !found["typing"] {
		t.Errorf("Expected message types [message, typing], got %v", ws.MessageTypes)
	}
}

// ============================================
// api_schema.go: MCP tool handler get_api_schema
// ============================================

func TestCoverageGroupB_ToolGetAPISchema_DefaultFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Observe some data through the schema store
	capture.schemaStore.Observe(NetworkBody{
		URL:          "https://api.example.com/users",
		Method:       "GET",
		Status:       200,
		ContentType:  "application/json",
		ResponseBody: `{"id":1}`,
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"api"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "endpoints") {
		t.Errorf("Expected endpoints in schema response, got: %s", text)
	}
}

func TestCoverageGroupB_ToolGetAPISchema_OpenAPIFormat(t *testing.T) {
	t.Parallel()
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.schemaStore.Observe(NetworkBody{
		URL:          "https://api.example.com/users",
		Method:       "GET",
		Status:       200,
		ContentType:  "application/json",
		ResponseBody: `{"id":1}`,
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/call",
		Params:  json.RawMessage(`{"name":"observe","arguments":{"what":"api","format":"openapi_stub"}}`),
	})

	text := extractMCPText(t, resp)
	if !strings.Contains(text, "openapi") {
		t.Errorf("Expected OpenAPI content, got: %s", text)
	}
}

// ============================================
// Helper: extractMCPTextFromResp extracts text from first content block
// ============================================

func extractMCPTextFromResp(t *testing.T, resp JSONRPCResponse) string {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("Failed to parse MCP result: %v", err)
	}
	if len(result.Content) == 0 {
		return ""
	}
	return result.Content[0].Text
}
