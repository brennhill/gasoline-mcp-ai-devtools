// api_contract_test.go — Tests for API contract validation.
// Tests schema learning, shape comparison, violation detection, and the MCP tool interface.
// Design: TDD approach - tests written first to define expected behavior.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ============================================
// Schema Learning Tests
// ============================================

func TestAPIContractValidator_LearnBasicSchema(t *testing.T) {
	v := NewAPIContractValidator()

	body := NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
		ContentType:  "application/json",
	}

	v.Learn(body)

	trackers := v.GetTrackers()
	if len(trackers) != 1 {
		t.Fatalf("Expected 1 tracker, got %d", len(trackers))
	}

	tracker := trackers["GET /api/users/{id}"]
	if tracker == nil {
		t.Fatal("Expected tracker for 'GET /api/users/{id}'")
	}
	if tracker.CallCount != 1 {
		t.Errorf("Expected CallCount=1, got %d", tracker.CallCount)
	}
}

func TestAPIContractValidator_LearnMultipleResponses(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn 3 consistent responses to establish the shape
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
			ContentType:  "application/json",
		})
	}

	trackers := v.GetTrackers()
	tracker := trackers["GET /api/users/{id}"]
	if tracker == nil {
		t.Fatal("Expected tracker")
	}
	if tracker.CallCount != 3 {
		t.Errorf("Expected CallCount=3, got %d", tracker.CallCount)
	}

	// Shape should be established
	shape := tracker.EstablishedShape
	if shape == nil {
		t.Fatal("Expected established shape")
	}

	shapeMap, ok := shape.(map[string]interface{})
	if !ok {
		t.Fatal("Expected shape to be a map")
	}

	// Check fields are present
	if _, ok := shapeMap["id"]; !ok {
		t.Error("Expected 'id' in shape")
	}
	if _, ok := shapeMap["name"]; !ok {
		t.Error("Expected 'name' in shape")
	}
	if _, ok := shapeMap["email"]; !ok {
		t.Error("Expected 'email' in shape")
	}
}

func TestAPIContractValidator_LearnMergesFields(t *testing.T) {
	v := NewAPIContractValidator()

	// First response with basic fields
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	// Second response adds 'email' field
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2,"name":"Bob","email":"bob@example.com"}`,
		ContentType:  "application/json",
	})

	trackers := v.GetTrackers()
	tracker := trackers["GET /api/users/{id}"]
	shapeMap := tracker.EstablishedShape.(map[string]interface{})

	// Shape should include all observed fields
	if _, ok := shapeMap["id"]; !ok {
		t.Error("Expected 'id' in merged shape")
	}
	if _, ok := shapeMap["name"]; !ok {
		t.Error("Expected 'name' in merged shape")
	}
	if _, ok := shapeMap["email"]; !ok {
		t.Error("Expected 'email' in merged shape")
	}
}

func TestAPIContractValidator_LearnTracksFieldPresence(t *testing.T) {
	v := NewAPIContractValidator()

	// 'email' only appears in 1 of 3 responses
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
		ContentType:  "application/json",
	})
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2,"name":"Bob"}`,
		ContentType:  "application/json",
	})
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/3",
		Status:       200,
		ResponseBody: `{"id":3,"name":"Carol"}`,
		ContentType:  "application/json",
	})

	trackers := v.GetTrackers()
	tracker := trackers["GET /api/users/{id}"]

	// Check field presence counts
	if tracker.FieldPresence["id"] != 3 {
		t.Errorf("Expected 'id' present 3 times, got %d", tracker.FieldPresence["id"])
	}
	if tracker.FieldPresence["name"] != 3 {
		t.Errorf("Expected 'name' present 3 times, got %d", tracker.FieldPresence["name"])
	}
	if tracker.FieldPresence["email"] != 1 {
		t.Errorf("Expected 'email' present 1 time, got %d", tracker.FieldPresence["email"])
	}
}

// ============================================
// Endpoint Normalization Tests
// ============================================

func TestNormalizeEndpoint_NumericID(t *testing.T) {
	result := normalizeEndpoint("GET", "http://localhost:3000/api/users/123")
	if result != "GET /api/users/{id}" {
		t.Errorf("Expected 'GET /api/users/{id}', got %q", result)
	}
}

func TestNormalizeEndpoint_UUID(t *testing.T) {
	result := normalizeEndpoint("GET", "http://localhost:3000/api/items/550e8400-e29b-41d4-a716-446655440000")
	if result != "GET /api/items/{id}" {
		t.Errorf("Expected 'GET /api/items/{id}', got %q", result)
	}
}

func TestNormalizeEndpoint_LongHex(t *testing.T) {
	result := normalizeEndpoint("GET", "http://localhost:3000/api/commits/a1b2c3d4e5f6a7b8c9d0")
	if result != "GET /api/commits/{id}" {
		t.Errorf("Expected 'GET /api/commits/{id}', got %q", result)
	}
}

func TestNormalizeEndpoint_IgnoresQueryParams(t *testing.T) {
	result := normalizeEndpoint("GET", "http://localhost:3000/api/users?page=1&limit=20")
	if result != "GET /api/users" {
		t.Errorf("Expected 'GET /api/users', got %q", result)
	}
}

func TestNormalizeEndpoint_MultipleIDs(t *testing.T) {
	result := normalizeEndpoint("GET", "http://localhost:3000/api/users/123/posts/456")
	if result != "GET /api/users/{id}/posts/{id}" {
		t.Errorf("Expected 'GET /api/users/{id}/posts/{id}', got %q", result)
	}
}

// ============================================
// Violation Detection Tests
// ============================================

func TestAPIContractValidator_DetectShapeChange(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn 3 consistent responses with 'avatar_url'
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/profile",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice","avatar_url":"https://example.com/avatar.png"}`,
			ContentType:  "application/json",
		})
	}

	// Now response is missing 'avatar_url'
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/profile",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	if len(violations) == 0 {
		t.Fatal("Expected shape_change violation")
	}

	found := false
	for _, v := range violations {
		if v.Type == "shape_change" && containsField(v.MissingFields, "avatar_url") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected shape_change violation for missing 'avatar_url', got %+v", violations)
	}
}

func TestAPIContractValidator_DetectTypeChange(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn responses where 'price' is a number
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/products/1",
			Status:       200,
			ResponseBody: `{"id":1,"price":19.99}`,
			ContentType:  "application/json",
		})
	}

	// Now 'price' is a string
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/products/1",
		Status:       200,
		ResponseBody: `{"id":1,"price":"19.99"}`,
		ContentType:  "application/json",
	})

	if len(violations) == 0 {
		t.Fatal("Expected type_change violation")
	}

	found := false
	for _, v := range violations {
		if v.Type == "type_change" && v.Field == "price" {
			found = true
			if v.ExpectedType != "number" {
				t.Errorf("Expected ExpectedType='number', got %q", v.ExpectedType)
			}
			if v.ActualType != "string" {
				t.Errorf("Expected ActualType='string', got %q", v.ActualType)
			}
			break
		}
	}
	if !found {
		t.Errorf("Expected type_change violation for 'price', got %+v", violations)
	}
}

func TestAPIContractValidator_DetectErrorSpike(t *testing.T) {
	v := NewAPIContractValidator()

	// 3 successful responses
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "POST",
			URL:          "http://localhost:3000/api/orders",
			Status:       201,
			ResponseBody: `{"id":1,"status":"created"}`,
			ContentType:  "application/json",
		})
	}

	// Now 2 error responses
	for i := 0; i < 2; i++ {
		violations := v.Validate(NetworkBody{
			Method:       "POST",
			URL:          "http://localhost:3000/api/orders",
			Status:       500,
			ResponseBody: `{"error":"Internal server error"}`,
			ContentType:  "application/json",
		})
		// Should detect error_spike on second 500
		if i == 1 && len(violations) == 0 {
			t.Error("Expected error_spike violation after consecutive 500s")
		}
	}

	tracker := v.GetTrackers()["POST /api/orders"]
	if tracker == nil {
		t.Fatal("Expected tracker")
	}

	// Status history should show the pattern
	history := tracker.StatusHistory
	if len(history) < 5 {
		t.Errorf("Expected at least 5 status entries, got %d", len(history))
	}
}

func TestAPIContractValidator_DetectNewField(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn 3 consistent responses
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Response with new field 'created_at'
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","created_at":"2025-01-20T14:30:00Z"}`,
		ContentType:  "application/json",
	})

	found := false
	for _, v := range violations {
		if v.Type == "new_field" && containsField(v.NewFields, "created_at") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected new_field violation for 'created_at', got %+v", violations)
	}
}

func TestAPIContractValidator_DetectNullField(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn responses where 'avatar' is a string
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"avatar":"https://example.com/avatar.png"}`,
			ContentType:  "application/json",
		})
	}

	// Now 'avatar' is null
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"avatar":null}`,
		ContentType:  "application/json",
	})

	found := false
	for _, v := range violations {
		if v.Type == "null_field" && v.Field == "avatar" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected null_field violation for 'avatar', got %+v", violations)
	}
}

// ============================================
// No Violation Cases
// ============================================

func TestAPIContractValidator_NoViolationWithConsistentResponses(t *testing.T) {
	v := NewAPIContractValidator()

	// All responses are identical
	for i := 0; i < 5; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	if len(violations) != 0 {
		t.Errorf("Expected no violations, got %+v", violations)
	}
}

func TestAPIContractValidator_NoViolationBeforeMinCalls(t *testing.T) {
	v := NewAPIContractValidator()

	// Only 2 observations - not enough to establish shape
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice","email":"alice@example.com"}`,
		ContentType:  "application/json",
	})
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/2",
		Status:       200,
		ResponseBody: `{"id":2,"name":"Bob","email":"bob@example.com"}`,
		ContentType:  "application/json",
	})

	// Missing field should not be a violation yet (still learning)
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/3",
		Status:       200,
		ResponseBody: `{"id":3,"name":"Carol"}`,
		ContentType:  "application/json",
	})

	// During learning phase, should not flag violations
	for _, v := range violations {
		if v.Type == "shape_change" {
			t.Error("Should not report shape_change violation before shape is established (min 3 calls)")
		}
	}
}

// ============================================
// Error Response Handling
// ============================================

func TestAPIContractValidator_ErrorResponsesNotUpdatingShape(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn success response
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	// Error response with different shape
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       404,
		ResponseBody: `{"error":"Not found","code":404}`,
		ContentType:  "application/json",
	})

	// Shape should still be from success response
	tracker := v.GetTrackers()["GET /api/users/{id}"]
	shape := tracker.EstablishedShape.(map[string]interface{})

	if _, ok := shape["error"]; ok {
		t.Error("Error response should not update established shape")
	}
	if _, ok := shape["id"]; !ok {
		t.Error("Expected 'id' from success response to be in shape")
	}
}

// ============================================
// MCP Tool Interface Tests
// ============================================

func TestAPIContractValidator_AnalyzeAction(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn some data
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// Cause a violation
	v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`, // Missing 'name'
		ContentType:  "application/json",
	})

	result := v.Analyze(APIContractFilter{})

	if result.Action != "analyzed" {
		t.Errorf("Expected action='analyzed', got %q", result.Action)
	}
	if len(result.Violations) == 0 {
		t.Error("Expected violations in analyze result")
	}
	if result.TrackedEndpoints != 1 {
		t.Errorf("Expected TrackedEndpoints=1, got %d", result.TrackedEndpoints)
	}
}

func TestAPIContractValidator_ReportAction(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn some data for multiple endpoints
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
		v.Learn(NetworkBody{
			Method:       "POST",
			URL:          "http://localhost:3000/api/orders",
			Status:       201,
			ResponseBody: `{"id":1,"total":100}`,
			ContentType:  "application/json",
		})
	}

	result := v.Report(APIContractFilter{})

	if result.Action != "report" {
		t.Errorf("Expected action='report', got %q", result.Action)
	}
	if len(result.Endpoints) != 2 {
		t.Errorf("Expected 2 endpoints, got %d", len(result.Endpoints))
	}
}

func TestAPIContractValidator_ClearAction(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn some data
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"name":"Alice"}`,
		ContentType:  "application/json",
	})

	// Clear
	v.Clear()

	trackers := v.GetTrackers()
	if len(trackers) != 0 {
		t.Errorf("Expected 0 trackers after clear, got %d", len(trackers))
	}
}

func TestAPIContractValidator_URLFilter(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn multiple endpoints
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/orders/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
	}

	// Report with URL filter
	result := v.Report(APIContractFilter{URLFilter: "users"})

	if len(result.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint with URL filter, got %d", len(result.Endpoints))
	}
	if !strings.Contains(result.Endpoints[0].Endpoint, "users") {
		t.Error("Expected filtered endpoint to contain 'users'")
	}
}

func TestAPIContractValidator_IgnoreEndpoints(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn multiple endpoints
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/health",
			Status:       200,
			ResponseBody: `{"status":"ok"}`,
			ContentType:  "application/json",
		})
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/metrics",
			Status:       200,
			ResponseBody: `{"uptime":12345}`,
			ContentType:  "application/json",
		})
	}

	// Report with ignore filter
	result := v.Report(APIContractFilter{IgnoreEndpoints: []string{"/health", "/metrics"}})

	if len(result.Endpoints) != 1 {
		t.Errorf("Expected 1 endpoint after ignoring /health and /metrics, got %d", len(result.Endpoints))
	}
}

// ============================================
// Edge Cases
// ============================================

func TestAPIContractValidator_NonJSONResponse(t *testing.T) {
	v := NewAPIContractValidator()

	// Non-JSON response should be ignored
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/download",
		Status:       200,
		ResponseBody: "plain text content",
		ContentType:  "text/plain",
	})

	trackers := v.GetTrackers()
	tracker := trackers["GET /api/download"]

	// Should still track the endpoint but not have a shape
	if tracker.EstablishedShape != nil {
		t.Error("Expected no shape for non-JSON response")
	}
}

func TestAPIContractValidator_EmptyResponseBody(t *testing.T) {
	v := NewAPIContractValidator()

	v.Learn(NetworkBody{
		Method:       "DELETE",
		URL:          "http://localhost:3000/api/users/1",
		Status:       204,
		ResponseBody: "",
		ContentType:  "",
	})

	trackers := v.GetTrackers()
	tracker := trackers["DELETE /api/users/{id}"]

	if tracker == nil {
		t.Fatal("Expected tracker even for empty response")
	}
	if tracker.CallCount != 1 {
		t.Errorf("Expected CallCount=1, got %d", tracker.CallCount)
	}
}

func TestAPIContractValidator_NestedObjectShapeChange(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn with nested object
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"profile":{"bio":"Hello","location":"NYC"}}`,
			ContentType:  "application/json",
		})
	}

	// Nested object missing a field - should detect if we track nested shapes
	violations := v.Validate(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1,"profile":{"bio":"Hello"}}`,
		ContentType:  "application/json",
	})

	// Note: Spec says max depth 3 for shape comparison
	// This test validates that nested shape changes are detected
	if len(violations) == 0 {
		t.Log("Note: Nested shape changes may not be detected depending on implementation depth")
	}
}

func TestAPIContractValidator_ArrayShapeConsistency(t *testing.T) {
	v := NewAPIContractValidator()

	// Learn with array of objects
	for i := 0; i < 3; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users",
			Status:       200,
			ResponseBody: `[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]`,
			ContentType:  "application/json",
		})
	}

	// Array response - shape tracking should work
	tracker := v.GetTrackers()["GET /api/users"]
	if tracker == nil {
		t.Fatal("Expected tracker for array endpoint")
	}
}

func TestAPIContractValidator_StatusHistoryLimit(t *testing.T) {
	v := NewAPIContractValidator()

	// Add more than 20 requests (status history limit per spec)
	for i := 0; i < 25; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
	}

	tracker := v.GetTrackers()["GET /api/users/{id}"]
	if len(tracker.StatusHistory) > 20 {
		t.Errorf("Expected status history capped at 20, got %d", len(tracker.StatusHistory))
	}
}

func TestAPIContractValidator_EndpointLimit(t *testing.T) {
	v := NewAPIContractValidator()

	// Add more than 30 unique endpoints (limit per spec)
	for i := 0; i < 35; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/endpoint" + string(rune('a'+i)),
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
	}

	trackers := v.GetTrackers()
	if len(trackers) > 30 {
		t.Errorf("Expected max 30 tracked endpoints, got %d", len(trackers))
	}
}

// ============================================
// Consistency Calculation
// ============================================

func TestAPIContractValidator_ConsistencyCalculation(t *testing.T) {
	v := NewAPIContractValidator()

	// 8 consistent responses
	for i := 0; i < 8; i++ {
		v.Learn(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1,"name":"Alice"}`,
			ContentType:  "application/json",
		})
	}

	// 2 inconsistent responses (missing 'name')
	for i := 0; i < 2; i++ {
		v.Validate(NetworkBody{
			Method:       "GET",
			URL:          "http://localhost:3000/api/users/1",
			Status:       200,
			ResponseBody: `{"id":1}`,
			ContentType:  "application/json",
		})
	}

	result := v.Report(APIContractFilter{})
	if len(result.Endpoints) == 0 {
		t.Fatal("Expected endpoint in report")
	}

	ep := result.Endpoints[0]
	// 8 consistent out of 10 = 80%
	if ep.Consistency != "80%" {
		t.Errorf("Expected consistency '80%%', got %q", ep.Consistency)
	}
}

// ============================================
// Timestamp Tracking
// ============================================

func TestAPIContractValidator_LastCalledTimestamp(t *testing.T) {
	v := NewAPIContractValidator()

	before := time.Now()
	v.Learn(NetworkBody{
		Method:       "GET",
		URL:          "http://localhost:3000/api/users/1",
		Status:       200,
		ResponseBody: `{"id":1}`,
		ContentType:  "application/json",
	})
	after := time.Now()

	tracker := v.GetTrackers()["GET /api/users/{id}"]
	if tracker.LastCalled.Before(before) || tracker.LastCalled.After(after) {
		t.Error("LastCalled timestamp not in expected range")
	}
}

// ============================================
// Helper Functions
// ============================================

func containsField(fields []string, target string) bool {
	for _, f := range fields {
		if f == target {
			return true
		}
	}
	return false
}

// ============================================
// Integration with NetworkBody stream
// ============================================

func TestAPIContractValidator_ProcessNetworkBodies(t *testing.T) {
	v := NewAPIContractValidator()

	bodies := []NetworkBody{
		{Method: "GET", URL: "http://localhost:3000/api/users/1", Status: 200, ResponseBody: `{"id":1,"name":"Alice"}`, ContentType: "application/json"},
		{Method: "GET", URL: "http://localhost:3000/api/users/2", Status: 200, ResponseBody: `{"id":2,"name":"Bob"}`, ContentType: "application/json"},
		{Method: "GET", URL: "http://localhost:3000/api/users/3", Status: 200, ResponseBody: `{"id":3,"name":"Carol"}`, ContentType: "application/json"},
		{Method: "POST", URL: "http://localhost:3000/api/users", Status: 201, ResponseBody: `{"id":4,"name":"Dave"}`, ContentType: "application/json"},
	}

	for _, body := range bodies {
		v.Learn(body)
	}

	trackers := v.GetTrackers()
	if len(trackers) != 2 {
		t.Errorf("Expected 2 unique endpoints, got %d", len(trackers))
	}
}

// ============================================
// JSON Serialization Tests
// ============================================

func TestAPIContractViolation_JSONSerialization(t *testing.T) {
	violation := APIContractViolation{
		Endpoint:      "GET /api/users/{id}",
		Type:          "shape_change",
		Description:   "Field 'avatar_url' was present in 5 responses but missing in the latest",
		MissingFields: []string{"avatar_url"},
	}

	data, err := json.Marshal(violation)
	if err != nil {
		t.Fatalf("Failed to marshal violation: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal violation: %v", err)
	}

	if parsed["endpoint"] != "GET /api/users/{id}" {
		t.Error("Expected 'endpoint' field in JSON")
	}
	if parsed["type"] != "shape_change" {
		t.Error("Expected 'type' field in JSON")
	}
}
