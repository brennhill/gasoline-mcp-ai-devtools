package main

import (
	"encoding/json"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// ============================================
// Path Parameterization Tests
// ============================================

func TestPathParameterize_UUIDReplacement(t *testing.T) {
	result := parameterizePath("/api/users/550e8400-e29b-41d4-a716-446655440000/posts")
	expected := "/api/users/{uuid}/posts"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestPathParameterize_NumericIDReplacement(t *testing.T) {
	result := parameterizePath("/api/users/123/posts")
	expected := "/api/users/{id}/posts"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestPathParameterize_StaticPathUnchanged(t *testing.T) {
	result := parameterizePath("/api/users/list")
	expected := "/api/users/list"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestPathParameterize_MixedUUIDAndID(t *testing.T) {
	result := parameterizePath("/api/orgs/550e8400-e29b-41d4-a716-446655440000/users/42/settings")
	expected := "/api/orgs/{uuid}/users/{id}/settings"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestPathParameterize_HexHashReplacement(t *testing.T) {
	result := parameterizePath("/api/commits/a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6")
	expected := "/api/commits/{hash}"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestPathParameterize_ShortSegmentNotReplaced(t *testing.T) {
	// Short hex segments (< 16 chars) should not be replaced as hash
	result := parameterizePath("/api/items/abc123")
	// "abc123" is not purely numeric and not a UUID and not long enough for hash
	// It should be kept as-is since it's not a recognized pattern
	if result != "/api/items/abc123" {
		t.Errorf("Expected '/api/items/abc123', got %q", result)
	}
}

// ============================================
// Accumulator Tests
// ============================================

func TestSchemaStore_FirstObservation(t *testing.T) {
	store := NewSchemaStore()

	body := NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
		Duration:     50,
	}

	store.Observe(body)

	schema := store.BuildSchema(SchemaFilter{})
	if len(schema.Endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint, got %d", len(schema.Endpoints))
	}
	ep := schema.Endpoints[0]
	if ep.Method != "GET" {
		t.Errorf("Expected method GET, got %s", ep.Method)
	}
	if ep.PathPattern != "/api/users/{id}" {
		t.Errorf("Expected path pattern '/api/users/{id}', got %s", ep.PathPattern)
	}
	if ep.ObservationCount != 1 {
		t.Errorf("Expected 1 observation, got %d", ep.ObservationCount)
	}
}

func TestSchemaStore_RepeatedObservation(t *testing.T) {
	store := NewSchemaStore()

	for i := 0; i < 5; i++ {
		store.Observe(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
			Duration:     50,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	if len(schema.Endpoints) != 1 {
		t.Fatalf("Expected 1 endpoint, got %d", len(schema.Endpoints))
	}
	if schema.Endpoints[0].ObservationCount != 5 {
		t.Errorf("Expected 5 observations, got %d", schema.Endpoints[0].ObservationCount)
	}
}

// ============================================
// Response Body Shape Extraction
// ============================================

func TestSchemaStore_ResponseBodyShape(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","active":true,"tags":["admin"],"meta":{"role":"owner"},"score":3.14}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	if len(ep.ResponseShapes) == 0 {
		t.Fatal("Expected response shapes")
	}

	shape, ok := ep.ResponseShapes[200]
	if !ok {
		t.Fatal("Expected response shape for status 200")
	}

	// Check field types
	tests := map[string]string{
		"id":     "integer",
		"name":   "string",
		"active": "boolean",
		"tags":   "array",
		"meta":   "object",
		"score":  "number",
	}

	for field, expectedType := range tests {
		fs, exists := shape.Fields[field]
		if !exists {
			t.Errorf("Expected field %q in response shape", field)
			continue
		}
		if fs.Type != expectedType {
			t.Errorf("Field %q: expected type %q, got %q", field, expectedType, fs.Type)
		}
	}
}

func TestSchemaStore_NullFieldType(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"avatar":null}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]
	shape := ep.ResponseShapes[200]

	fs, exists := shape.Fields["avatar"]
	if !exists {
		t.Fatal("Expected field 'avatar'")
	}
	if fs.Type != "null" {
		t.Errorf("Expected type 'null', got %q", fs.Type)
	}
}

// ============================================
// Request Body Shape with Format Detection
// ============================================

func TestSchemaStore_RequestBodyWithEmailFormat(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "POST",
		URL:          "http://localhost:3000/api/users",
		Status:       201,
		RequestBody:  `{"email":"alice@example.com","name":"Alice"}`,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
		Duration:     100,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	if ep.RequestShape == nil {
		t.Fatal("Expected request shape")
	}

	emailField, exists := ep.RequestShape.Fields["email"]
	if !exists {
		t.Fatal("Expected field 'email'")
	}
	if emailField.Format != "email" {
		t.Errorf("Expected format 'email', got %q", emailField.Format)
	}
}

// ============================================
// Latency / Timing Stats Tests
// ============================================

func TestSchemaStore_TimingStats(t *testing.T) {
	store := NewSchemaStore()

	durations := []int{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for _, d := range durations {
		store.Observe(NetworkBody{
			Method:   "GET",
			URL:      "http://localhost:3000/api/health",
			Status:   200,
			Duration: d,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	// Average should be 55
	if math.Abs(ep.Timing.Avg-55.0) > 0.1 {
		t.Errorf("Expected avg ~55, got %f", ep.Timing.Avg)
	}

	// P50 should be ~50 or ~55
	if ep.Timing.P50 < 40 || ep.Timing.P50 > 60 {
		t.Errorf("Expected P50 around 50, got %f", ep.Timing.P50)
	}

	// P95 should be ~95 or higher
	if ep.Timing.P95 < 90 {
		t.Errorf("Expected P95 >= 90, got %f", ep.Timing.P95)
	}

	// Max should be 100
	if ep.Timing.Max != 100.0 {
		t.Errorf("Expected max 100, got %f", ep.Timing.Max)
	}
}

// ============================================
// Query Parameter Tracking
// ============================================

func TestSchemaStore_QueryParameters(t *testing.T) {
	store := NewSchemaStore()

	// 10 requests with "page" and "limit" query params
	for i := 1; i <= 10; i++ {
		store.Observe(NetworkBody{
			Method:   "GET",
			URL:      "http://localhost:3000/api/users?page=1&limit=20",
			Status:   200,
			Duration: 50,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	if len(ep.QueryParams) == 0 {
		t.Fatal("Expected query parameters")
	}

	foundPage := false
	foundLimit := false
	for _, qp := range ep.QueryParams {
		if qp.Name == "page" {
			foundPage = true
			if !qp.Required {
				t.Error("Expected 'page' to be required (present in >90%)")
			}
		}
		if qp.Name == "limit" {
			foundLimit = true
			if !qp.Required {
				t.Error("Expected 'limit' to be required (present in >90%)")
			}
		}
	}
	if !foundPage {
		t.Error("Expected query param 'page'")
	}
	if !foundLimit {
		t.Error("Expected query param 'limit'")
	}
}

func TestSchemaStore_QueryParamOptional(t *testing.T) {
	store := NewSchemaStore()

	// 10 requests, only 5 have "sort" param (50% < 90% threshold)
	for i := 0; i < 10; i++ {
		url := "http://localhost:3000/api/users?page=1"
		if i < 5 {
			url += "&sort=name"
		}
		store.Observe(NetworkBody{
			Method:   "GET",
			URL:      url,
			Status:   200,
			Duration: 50,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	for _, qp := range ep.QueryParams {
		if qp.Name == "sort" {
			if qp.Required {
				t.Error("Expected 'sort' to be optional (present in <90%)")
			}
			return
		}
	}
	t.Error("Expected 'sort' query parameter to be tracked")
}

func TestSchemaStore_QueryParamTypeInference(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:   "GET",
		URL:      "http://localhost:3000/api/users?page=1&active=true&name=alice",
		Status:   200,
		Duration: 50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	for _, qp := range ep.QueryParams {
		switch qp.Name {
		case "page":
			if qp.Type != "integer" {
				t.Errorf("Expected 'page' type 'integer', got %q", qp.Type)
			}
		case "active":
			if qp.Type != "boolean" {
				t.Errorf("Expected 'active' type 'boolean', got %q", qp.Type)
			}
		case "name":
			if qp.Type != "string" {
				t.Errorf("Expected 'name' type 'string', got %q", qp.Type)
			}
		}
	}
}

// ============================================
// Max Endpoint Cap
// ============================================

func TestSchemaStore_MaxEndpoints(t *testing.T) {
	store := NewSchemaStore()

	// Add 200 unique endpoints
	for i := 0; i < 200; i++ {
		store.Observe(NetworkBody{
			Method:   "GET",
			URL:      "http://localhost:3000/api/resource" + string(rune('A'+i%26)) + "/" + intToStr(i),
			Status:   200,
			Duration: 50,
		})
	}

	// 201st should be ignored
	store.Observe(NetworkBody{
		Method:   "DELETE",
		URL:      "http://localhost:3000/api/brand-new-endpoint",
		Status:   200,
		Duration: 50,
	})

	schema := store.BuildSchema(SchemaFilter{})

	if len(schema.Endpoints) > maxSchemaEndpoints {
		t.Errorf("Expected max %d endpoints, got %d", maxSchemaEndpoints, len(schema.Endpoints))
	}
}

// Helper to convert int to string without importing strconv
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

// ============================================
// Required Field Detection
// ============================================

func TestSchemaStore_RequiredFieldDetection(t *testing.T) {
	store := NewSchemaStore()

	// 10 observations where "id" is always present but "avatar" only 5 times
	for i := 0; i < 10; i++ {
		body := `{"id":1,"name":"Alice"}`
		if i < 5 {
			body = `{"id":1,"name":"Alice","avatar":"http://example.com/pic.jpg"}`
		}
		store.Observe(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: body,
			ContentType:  "application/json",
			Duration:     50,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]
	shape := ep.ResponseShapes[200]

	idField := shape.Fields["id"]
	if !idField.Required {
		t.Error("Expected 'id' to be required (present in 100% of observations)")
	}

	nameField := shape.Fields["name"]
	if !nameField.Required {
		t.Error("Expected 'name' to be required (present in 100% of observations)")
	}

	avatarField := shape.Fields["avatar"]
	if avatarField.Required {
		t.Error("Expected 'avatar' to be optional (present in 50% of observations)")
	}
}

// ============================================
// Type Voting (Majority Wins)
// ============================================

func TestSchemaStore_TypeVoting(t *testing.T) {
	store := NewSchemaStore()

	// 7 observations where "score" is a number, 3 where it's null
	for i := 0; i < 10; i++ {
		body := `{"score":3.14}`
		if i >= 7 {
			body = `{"score":null}`
		}
		store.Observe(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/scores/1",
			Status:       200,
			ResponseBody: body,
			ContentType:  "application/json",
			Duration:     50,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]
	shape := ep.ResponseShapes[200]

	scoreField := shape.Fields["score"]
	if scoreField.Type != "number" {
		t.Errorf("Expected majority type 'number', got %q", scoreField.Type)
	}
}

// ============================================
// Format Detection Tests
// ============================================

func TestSchemaStore_UUIDFormatDetection(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/items/1",
		Status:       200,
		ResponseBody: `{"ref":"550e8400-e29b-41d4-a716-446655440000"}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]
	shape := ep.ResponseShapes[200]

	refField := shape.Fields["ref"]
	if refField.Format != "uuid" {
		t.Errorf("Expected format 'uuid', got %q", refField.Format)
	}
}

func TestSchemaStore_DatetimeFormatDetection(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/items/1",
		Status:       200,
		ResponseBody: `{"createdAt":"2024-01-15T10:30:00.000Z"}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]
	shape := ep.ResponseShapes[200]

	field := shape.Fields["createdAt"]
	if field.Format != "datetime" {
		t.Errorf("Expected format 'datetime', got %q", field.Format)
	}
}

func TestSchemaStore_EmailFormatDetection(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/items/1",
		Status:       200,
		ResponseBody: `{"contact":"alice@example.com"}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]
	shape := ep.ResponseShapes[200]

	field := shape.Fields["contact"]
	if field.Format != "email" {
		t.Errorf("Expected format 'email', got %q", field.Format)
	}
}

func TestSchemaStore_URLFormatDetection(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/items/1",
		Status:       200,
		ResponseBody: `{"website":"https://example.com/page"}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]
	shape := ep.ResponseShapes[200]

	field := shape.Fields["website"]
	if field.Format != "url" {
		t.Errorf("Expected format 'url', got %q", field.Format)
	}
}

func TestSchemaStore_IntegerVsFloat(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/items/1",
		Status:       200,
		ResponseBody: `{"count":42,"price":19.99}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]
	shape := ep.ResponseShapes[200]

	countField := shape.Fields["count"]
	if countField.Type != "integer" {
		t.Errorf("Expected type 'integer' for count, got %q", countField.Type)
	}

	priceField := shape.Fields["price"]
	if priceField.Type != "number" {
		t.Errorf("Expected type 'number' for price, got %q", priceField.Type)
	}
}

// ============================================
// Filter Tests
// ============================================

func TestSchemaStore_URLFilter(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/users/1", Status: 200, Duration: 50})
	store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/products/1", Status: 200, Duration: 50})
	store.Observe(NetworkBody{Method: "POST", URL: "http://localhost:3000/api/users", Status: 201, Duration: 100})

	schema := store.BuildSchema(SchemaFilter{URLFilter: "users"})
	if len(schema.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints matching 'users', got %d", len(schema.Endpoints))
	}
}

func TestSchemaStore_MinObservationsFilter(t *testing.T) {
	store := NewSchemaStore()

	// Observe /api/users 5 times
	for i := 0; i < 5; i++ {
		store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/users/1", Status: 200, Duration: 50})
	}
	// Observe /api/rare once
	store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/rare", Status: 200, Duration: 50})

	schema := store.BuildSchema(SchemaFilter{MinObservations: 3})
	if len(schema.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint with >= 3 observations, got %d", len(schema.Endpoints))
	}
	if schema.Endpoints[0].PathPattern != "/api/users/{id}" {
		t.Errorf("Expected '/api/users/{id}', got %s", schema.Endpoints[0].PathPattern)
	}
}

// ============================================
// Multiple Status Codes
// ============================================

func TestSchemaStore_MultipleStatusCodes(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
		Duration:     50,
	})
	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/999",
		Status:       404,
		ResponseBody: `{"error":"not found"}`,
		ContentType:  "application/json",
		Duration:     20,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	if len(ep.ResponseShapes) != 2 {
		t.Errorf("Expected 2 response shapes (200 and 404), got %d", len(ep.ResponseShapes))
	}

	shape200, ok := ep.ResponseShapes[200]
	if !ok {
		t.Fatal("Expected shape for status 200")
	}
	if _, exists := shape200.Fields["name"]; !exists {
		t.Error("Expected 'name' field in 200 response")
	}

	shape404, ok := ep.ResponseShapes[404]
	if !ok {
		t.Fatal("Expected shape for status 404")
	}
	if _, exists := shape404.Fields["error"]; !exists {
		t.Error("Expected 'error' field in 404 response")
	}
}

// ============================================
// Auth Pattern Detection
// ============================================

func TestSchemaStore_AuthPatternDetected(t *testing.T) {
	store := NewSchemaStore()

	// Login endpoint
	store.Observe(NetworkBody{
		Method:       "POST",
		URL:          "http://localhost:3000/api/auth/login",
		Status:       200,
		RequestBody:  `{"email":"alice@example.com","password":"[redacted]"}`,
		ResponseBody: `{"token":"eyJhbGciOiJIUzI1NiJ9"}`,
		ContentType:  "application/json",
		Duration:     200,
	})
	// Normal requests
	for i := 0; i < 5; i++ {
		store.Observe(NetworkBody{
			Method:   "GET",
			URL:      "http://localhost:3000/api/users",
			Status:   200,
			Duration: 50,
		})
	}
	// 401 response
	store.Observe(NetworkBody{
		Method:   "GET",
		URL:      "http://localhost:3000/api/protected",
		Status:   401,
		Duration: 10,
	})

	schema := store.BuildSchema(SchemaFilter{})

	if schema.AuthPattern == nil {
		t.Fatal("Expected auth pattern to be detected")
	}
	if schema.AuthPattern.Type != "bearer" {
		t.Errorf("Expected auth type 'bearer', got %q", schema.AuthPattern.Type)
	}
}

func TestSchemaStore_NoAuthPattern(t *testing.T) {
	store := NewSchemaStore()

	// No auth/login endpoints and no 401s
	store.Observe(NetworkBody{
		Method:   "GET",
		URL:      "http://localhost:3000/api/public",
		Status:   200,
		Duration: 50,
	})
	store.Observe(NetworkBody{
		Method:   "GET",
		URL:      "http://localhost:3000/api/health",
		Status:   200,
		Duration: 10,
	})

	schema := store.BuildSchema(SchemaFilter{})

	if schema.AuthPattern != nil {
		t.Error("Expected no auth pattern when no auth endpoints or 401s exist")
	}
}

// ============================================
// WebSocket Schema Inference
// ============================================

func TestSchemaStore_WebSocketMessages(t *testing.T) {
	store := NewSchemaStore()

	store.ObserveWebSocket(WebSocketEvent{
		URL:       "ws://localhost:3000/ws",
		Direction: "incoming",
		Data:      `{"type":"ping","ts":1234}`,
	})
	store.ObserveWebSocket(WebSocketEvent{
		URL:       "ws://localhost:3000/ws",
		Direction: "incoming",
		Data:      `{"type":"update","data":{"id":1}}`,
	})
	store.ObserveWebSocket(WebSocketEvent{
		URL:       "ws://localhost:3000/ws",
		Direction: "outgoing",
		Data:      `{"action":"subscribe","channel":"users"}`,
	})

	schema := store.BuildSchema(SchemaFilter{})

	if len(schema.WebSockets) == 0 {
		t.Fatal("Expected WebSocket schemas")
	}

	ws := schema.WebSockets[0]
	if ws.URL != "ws://localhost:3000/ws" {
		t.Errorf("Expected URL 'ws://localhost:3000/ws', got %q", ws.URL)
	}
	if ws.TotalMessages != 3 {
		t.Errorf("Expected 3 total messages, got %d", ws.TotalMessages)
	}

	// Check type field detection
	if len(ws.MessageTypes) == 0 {
		t.Error("Expected detected message types")
	}
}

// ============================================
// Coverage Statistics
// ============================================

func TestSchemaStore_ErrorRate(t *testing.T) {
	store := NewSchemaStore()

	// 8 successful, 2 errors
	for i := 0; i < 8; i++ {
		store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/data", Status: 200, Duration: 50})
	}
	store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/data", Status: 500, Duration: 50})
	store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/data", Status: 404, Duration: 50})

	schema := store.BuildSchema(SchemaFilter{})

	// Error rate should be 20% (2/10)
	expectedRate := 20.0
	if math.Abs(schema.Coverage.ErrorRate-expectedRate) > 0.1 {
		t.Errorf("Expected error rate ~20%%, got %f%%", schema.Coverage.ErrorRate)
	}
}

// ============================================
// Concurrent Access
// ============================================

func TestSchemaStore_ConcurrentObservations(t *testing.T) {
	store := NewSchemaStore()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			store.Observe(NetworkBody{
				Method:       "GET",
				URL:          "http://localhost:3000/api/items/" + intToStr(idx%10),
				Status:       200,
				ResponseBody: `{"id":1}`,
				ContentType:  "application/json",
				Duration:     50,
			})
		}(i)
	}
	wg.Wait()

	// Should not panic, and should have observations
	schema := store.BuildSchema(SchemaFilter{})
	if len(schema.Endpoints) == 0 {
		t.Error("Expected endpoints after concurrent observations")
	}

	// All 100 observations should be counted
	total := 0
	for _, ep := range schema.Endpoints {
		total += ep.ObservationCount
	}
	if total != 100 {
		t.Errorf("Expected 100 total observations, got %d", total)
	}
}

// ============================================
// OpenAPI Stub Output
// ============================================

func TestSchemaStore_OpenAPIStubFormat(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
		Duration:     50,
	})
	store.Observe(NetworkBody{
		Method:       "POST",
		URL:          "http://localhost:3000/api/users",
		Status:       201,
		RequestBody:  `{"name":"Bob"}`,
		ResponseBody: `{"id":2,"name":"Bob"}`,
		ContentType:  "application/json",
		Duration:     100,
	})

	yaml := store.BuildOpenAPIStub(SchemaFilter{})

	// Should be valid YAML structure
	if !strings.Contains(yaml, "openapi: \"3.0.0\"") {
		t.Error("Expected OpenAPI version in output")
	}
	if !strings.Contains(yaml, "paths:") {
		t.Error("Expected 'paths:' section in output")
	}
	if !strings.Contains(yaml, "/api/users/{id}") {
		t.Error("Expected '/api/users/{id}' path in output")
	}
	if !strings.Contains(yaml, "/api/users") {
		t.Error("Expected '/api/users' path in output")
	}
	if !strings.Contains(yaml, "get:") || !strings.Contains(yaml, "post:") {
		t.Error("Expected both get and post methods in output")
	}
}

// ============================================
// Latency Sample Cap
// ============================================

func TestSchemaStore_MaxLatencySamples(t *testing.T) {
	store := NewSchemaStore()

	// Add 150 observations - should cap at 100 latency samples
	for i := 0; i < 150; i++ {
		store.Observe(NetworkBody{
			Method:   "GET",
			URL:      "http://localhost:3000/api/test",
			Status:   200,
			Duration: i + 1,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	// Timing should still be computed from collected samples
	if ep.Timing.Max == 0 {
		t.Error("Expected timing stats to be present")
	}
	if ep.ObservationCount != 150 {
		t.Errorf("Expected 150 observations counted, got %d", ep.ObservationCount)
	}
}

// ============================================
// Max Query Param Values
// ============================================

func TestSchemaStore_MaxQueryParamValues(t *testing.T) {
	store := NewSchemaStore()

	// 15 unique values for "page" param - should cap at 10
	for i := 0; i < 15; i++ {
		store.Observe(NetworkBody{
			Method:   "GET",
			URL:      "http://localhost:3000/api/items?page=" + intToStr(i),
			Status:   200,
			Duration: 50,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	for _, qp := range ep.QueryParams {
		if qp.Name == "page" {
			if len(qp.ObservedValues) > maxQueryParamValues {
				t.Errorf("Expected max %d observed values, got %d", maxQueryParamValues, len(qp.ObservedValues))
			}
			return
		}
	}
	t.Error("Expected 'page' query parameter")
}

// ============================================
// Path Parameter Detection
// ============================================

func TestSchemaStore_PathParameters(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:   "GET",
		URL:      "http://localhost:3000/api/users/42/posts/100",
		Status:   200,
		Duration: 50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	if len(ep.PathParams) != 2 {
		t.Fatalf("Expected 2 path params, got %d", len(ep.PathParams))
	}

	// First param at position 2 (api/users/{id}/posts/{id})
	found := false
	for _, pp := range ep.PathParams {
		if pp.Name == "id" && pp.Type == "integer" {
			found = true
		}
	}
	if !found {
		t.Error("Expected path param with name 'id' and type 'integer'")
	}
}

// ============================================
// MCP Tool Integration
// ============================================

func TestMCPGetAPISchema(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	// Add some network bodies that will trigger schema inference
	capture.AddNetworkBodies([]NetworkBody{
		{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
			Duration:     50,
		},
	})

	// Observe in schema store
	capture.schemaStore.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"api"}}`),
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

	if len(result.Content) == 0 {
		t.Fatal("Expected content in response")
	}

	var schema APISchema
	if err := json.Unmarshal([]byte(result.Content[0].Text), &schema); err != nil {
		t.Fatalf("Expected valid JSON schema, got error: %v", err)
	}

	if len(schema.Endpoints) == 0 {
		t.Error("Expected endpoints in schema")
	}
}

func TestMCPGetAPISchemaWithURLFilter(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.schemaStore.Observe(NetworkBody{
		Method: "GET", URL: "http://localhost:3000/api/users/1", Status: 200, Duration: 50,
	})
	capture.schemaStore.Observe(NetworkBody{
		Method: "GET", URL: "http://localhost:3000/api/products/1", Status: 200, Duration: 50,
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"api","url_filter":"users"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	var schema APISchema
	json.Unmarshal([]byte(result.Content[0].Text), &schema)

	if len(schema.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint matching 'users', got %d", len(schema.Endpoints))
	}
}

func TestMCPGetAPISchemaOpenAPIFormat(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	capture.schemaStore.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
		Duration:     50,
	})

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/call",
		Params: json.RawMessage(`{"name":"analyze","arguments":{"target":"api","format":"openapi_stub"}}`),
	})

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.Unmarshal(resp.Result, &result)

	text := result.Content[0].Text
	if !strings.Contains(text, "openapi:") {
		t.Error("Expected OpenAPI stub format output")
	}
}

// ============================================
// Non-JSON Body Handling
// ============================================

func TestSchemaStore_NonJSONBodyIgnoredForShape(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/page",
		Status:       200,
		ResponseBody: "<html><body>Hello</body></html>",
		ContentType:  "text/html",
		Duration:     50,
	})

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	// Should still track the endpoint for timing/status
	if ep.ObservationCount != 1 {
		t.Errorf("Expected 1 observation, got %d", ep.ObservationCount)
	}

	// But no response shape fields
	if shape, ok := ep.ResponseShapes[200]; ok && len(shape.Fields) > 0 {
		t.Error("Expected no fields for non-JSON response")
	}
}

// ============================================
// Sorted by Observation Count
// ============================================

func TestSchemaStore_SortedByObservationCount(t *testing.T) {
	store := NewSchemaStore()

	// /api/popular gets 10 observations
	for i := 0; i < 10; i++ {
		store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/popular", Status: 200, Duration: 50})
	}
	// /api/rare gets 1 observation
	store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/rare", Status: 200, Duration: 50})
	// /api/medium gets 5 observations
	for i := 0; i < 5; i++ {
		store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/medium", Status: 200, Duration: 50})
	}

	schema := store.BuildSchema(SchemaFilter{})

	if len(schema.Endpoints) != 3 {
		t.Fatalf("Expected 3 endpoints, got %d", len(schema.Endpoints))
	}

	if schema.Endpoints[0].ObservationCount != 10 {
		t.Errorf("First endpoint should have most observations (10), got %d", schema.Endpoints[0].ObservationCount)
	}
	if schema.Endpoints[1].ObservationCount != 5 {
		t.Errorf("Second endpoint should have 5 observations, got %d", schema.Endpoints[1].ObservationCount)
	}
	if schema.Endpoints[2].ObservationCount != 1 {
		t.Errorf("Third endpoint should have 1 observation, got %d", schema.Endpoints[2].ObservationCount)
	}
}

// ============================================
// Max Actual Paths Tracked
// ============================================

func TestSchemaStore_MaxActualPaths(t *testing.T) {
	store := NewSchemaStore()

	// 25 different actual paths for the same pattern
	for i := 0; i < 25; i++ {
		store.Observe(NetworkBody{
			Method:   "GET",
			URL:      "http://localhost:3000/api/users/" + intToStr(i),
			Status:   200,
			Duration: 50,
		})
	}

	schema := store.BuildSchema(SchemaFilter{})
	ep := schema.Endpoints[0]

	// Should track last observed path
	if ep.LastPath == "" {
		t.Error("Expected last path to be set")
	}
}

// ============================================
// Integration: Observe from AddNetworkBodies
// ============================================

func TestCapture_SchemaStoreIntegration(t *testing.T) {
	capture := setupTestCapture(t)

	// Adding network bodies should also trigger schema observation
	capture.AddNetworkBodies([]NetworkBody{
		{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
			Duration:     50,
		},
		{
			Method:       "POST",
			URL:          "http://localhost:3000/api/users",
			Status:       201,
			RequestBody:  `{"name":"Bob"}`,
			ResponseBody: `{"id":2}`,
			ContentType:  "application/json",
			Duration:     100,
		},
	})

	// Give async goroutine time to process
	time.Sleep(50 * time.Millisecond)

	schema := capture.schemaStore.BuildSchema(SchemaFilter{})
	if len(schema.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints from integration, got %d", len(schema.Endpoints))
	}
}

// ============================================
// Coverage Statistics
// ============================================

func TestSchemaStore_CoverageStats(t *testing.T) {
	store := NewSchemaStore()

	store.Observe(NetworkBody{Method: "GET", URL: "http://localhost:3000/api/users", Status: 200, Duration: 50})
	store.Observe(NetworkBody{Method: "POST", URL: "http://localhost:3000/api/users", Status: 201, Duration: 100})
	store.Observe(NetworkBody{Method: "DELETE", URL: "http://localhost:3000/api/users/1", Status: 204, Duration: 30})

	schema := store.BuildSchema(SchemaFilter{})

	if schema.Coverage.TotalEndpoints != 3 {
		t.Errorf("Expected 3 total endpoints, got %d", schema.Coverage.TotalEndpoints)
	}

	if schema.Coverage.Methods["GET"] != 1 {
		t.Errorf("Expected 1 GET endpoint, got %d", schema.Coverage.Methods["GET"])
	}
	if schema.Coverage.Methods["POST"] != 1 {
		t.Errorf("Expected 1 POST endpoint, got %d", schema.Coverage.Methods["POST"])
	}
	if schema.Coverage.Methods["DELETE"] != 1 {
		t.Errorf("Expected 1 DELETE endpoint, got %d", schema.Coverage.Methods["DELETE"])
	}
}

// ============================================
// Tool Listed in tools/list
// ============================================

func TestMCPToolsListIncludesAnalyze(t *testing.T) {
	server, _ := setupTestServer(t)
	capture := setupTestCapture(t)
	mcp := setupToolHandler(t, server, capture)

	mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 1, Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}`),
	})

	resp := mcp.HandleRequest(JSONRPCRequest{
		JSONRPC: "2.0", ID: 2, Method: "tools/list",
	})

	var result struct {
		Tools []MCPTool `json:"tools"`
	}
	json.Unmarshal(resp.Result, &result)

	found := false
	for _, tool := range result.Tools {
		if tool.Name == "analyze" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'analyze' tool to be listed")
	}
}
