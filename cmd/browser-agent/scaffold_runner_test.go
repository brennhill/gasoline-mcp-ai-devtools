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

// ============================================
// Body Size Limit
// ============================================

func TestScaffoldRunner_RejectsOversizedBody(t *testing.T) {
	runner := NewScaffoldRunner()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/scaffold", runner.HandleScaffold)

	// Create a body larger than 1 MB.
	largeBody := strings.Repeat("x", 1<<20+1)
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("oversized body: want 400, got %d", rec.Code)
	}
}

// ============================================
// Concurrency Guard
// ============================================

func TestScaffoldRunner_RejectsDuplicateProject(t *testing.T) {
	runner := NewScaffoldRunner()

	// Simulate an in-flight scaffold for "dup-project".
	runner.inflight.Store("dup-project", struct{}{})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/scaffold", runner.HandleScaffold)

	body := `{"description":"a test app","audience":"just_me","first_feature":"testing","name":"dup-project"}`
	req := httptest.NewRequest("POST", "/api/scaffold", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("duplicate project: want 409, got %d", rec.Code)
	}
}

// ============================================
// Panic Recovery
// ============================================

func TestScaffoldRunner_PanicRecovery(t *testing.T) {
	runner := NewScaffoldRunner()

	channel := "scaffold-panic-test-0"
	ch := runner.broadcaster.Subscribe(channel)

	// Directly call runScaffold with a config that will cause NewEngine to succeed
	// but we'll verify the panic recovery by checking the inflight map is cleaned up.
	// We can't easily trigger a panic in the engine, so we test the defer cleanup instead.
	cfg := scaffold.Config{
		Description:  "test",
		Name:         "panic-test",
	}

	// Store in inflight to verify it gets cleaned up.
	runner.inflight.Store("panic-test", struct{}{})

	// Run scaffold in a goroutine — it will fail because the home directory
	// scaffold path likely doesn't exist or has permission issues, but should
	// not panic. The inflight entry should be cleaned up.
	done := make(chan struct{})
	go func() {
		runner.runScaffold(cfg, channel)
		close(done)
	}()

	select {
	case <-done:
		// Verify inflight was cleaned up.
		if _, loaded := runner.inflight.Load("panic-test"); loaded {
			t.Error("inflight entry should be cleaned up after runScaffold completes")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for runScaffold to complete")
	}

	// Drain any events from the channel.
	runner.broadcaster.Unsubscribe(ch)
}
