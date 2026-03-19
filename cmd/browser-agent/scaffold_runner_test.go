// scaffold_runner_test.go — Tests for the scaffold runner that wires API to engine.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/scaffold"
)

// ============================================
// Scaffold Runner: API → Engine integration
// ============================================

func TestScaffoldRunner_AcceptsAndTracksJob(t *testing.T) {
	runner := NewScaffoldRunner()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/scaffold", runner.HandleScaffold)

	body := `{"description":"a test app","audience":"just_me","first_feature":"testing","name":"test-runner-app"}`
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp scaffoldResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Channel == "" {
		t.Error("channel should not be empty")
	}
	if resp.Status != "accepted" {
		t.Errorf("status: want 'accepted', got %q", resp.Status)
	}
}

func TestScaffoldRunner_BroadcastsProgress(t *testing.T) {
	runner := NewScaffoldRunner()

	// Subscribe to any channel starting with "scaffold-".
	ch := runner.broadcaster.Subscribe("scaffold-test-app-0")

	// Manually broadcast an event.
	runner.broadcaster.Broadcast("scaffold-test-app-0", scaffold.StepEvent{
		Step:   "create_project",
		Status: "running",
		Label:  "Creating project",
	})

	select {
	case evt := <-ch:
		if evt.Step != "create_project" {
			t.Errorf("want step 'create_project', got %q", evt.Step)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for broadcast")
	}
}

func TestScaffoldRunner_RejectsInvalidInput(t *testing.T) {
	runner := NewScaffoldRunner()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/scaffold", runner.HandleScaffold)

	body := `{"description":"","name":""}`
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("want 400, got %d", rec.Code)
	}
}
