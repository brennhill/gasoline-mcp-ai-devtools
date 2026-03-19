// scaffold_api_test.go — Tests for POST /api/scaffold endpoint.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// Scaffold API: POST /api/scaffold
// ============================================

func TestScaffoldAPI_ValidRequest_Returns202(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	body := `{"description":"a todo app","audience":"just_me","first_feature":"drag and drop","name":"todo-app"}`
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("POST /api/scaffold: want 202, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var resp scaffoldResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("POST /api/scaffold: decode response: %v", err)
	}

	if resp.Status != "accepted" {
		t.Errorf("POST /api/scaffold: want status 'accepted', got %q", resp.Status)
	}
	if resp.Channel == "" {
		t.Error("POST /api/scaffold: channel must not be empty")
	}
	if !strings.HasPrefix(resp.Channel, "scaffold-todo-app-") {
		t.Errorf("POST /api/scaffold: channel %q should start with 'scaffold-todo-app-'", resp.Channel)
	}
}

func TestScaffoldAPI_MissingName_Returns400(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	body := `{"description":"a todo app","audience":"just_me","first_feature":"drag and drop"}`
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /api/scaffold missing name: want 400, got %d", rec.Code)
	}
}

func TestScaffoldAPI_MissingDescription_Returns400(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	body := `{"audience":"just_me","first_feature":"drag and drop","name":"todo-app"}`
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /api/scaffold missing description: want 400, got %d", rec.Code)
	}
}

func TestScaffoldAPI_EmptyBody_Returns400(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /api/scaffold empty body: want 400, got %d", rec.Code)
	}
}

func TestScaffoldAPI_InvalidJSON_Returns400(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /api/scaffold invalid json: want 400, got %d", rec.Code)
	}
}

func TestScaffoldAPI_GET_MethodNotAllowed(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	req := httptest.NewRequest("GET", "/api/scaffold", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/scaffold: want 405, got %d", rec.Code)
	}
}

func TestScaffoldAPI_InvalidAudience_Returns400(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	body := `{"description":"a todo app","audience":"aliens","first_feature":"drag and drop","name":"todo-app"}`
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /api/scaffold invalid audience: want 400, got %d", rec.Code)
	}
}

func TestScaffoldAPI_ResponseHasSnakeCaseFields(t *testing.T) {
	mux := http.NewServeMux()
	registerScaffoldRoutes(mux)

	body := `{"description":"a todo app","audience":"just_me","first_feature":"drag and drop","name":"todo-app"}`
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	raw := rec.Body.String()
	if !strings.Contains(raw, `"status"`) {
		t.Error("response should use snake_case field 'status'")
	}
	if !strings.Contains(raw, `"channel"`) {
		t.Error("response should use snake_case field 'channel'")
	}
}
