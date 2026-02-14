// tools_interact_helpers_test.go — Tests for queueComposableSubtitle and queueStateNavigation.
package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Test environment
// ============================================

type interactHelpersTestEnv struct {
	handler *ToolHandler
	server  *Server
	capture *capture.Capture
}

func newInteractHelpersTestEnv(t *testing.T) *interactHelpersTestEnv {
	t.Helper()
	logFile := filepath.Join(t.TempDir(), "test-interact-helpers.jsonl")
	server, err := NewServer(logFile, 100)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	t.Cleanup(func() { server.Close() })
	cap := capture.NewCapture()
	mcpHandler := NewToolHandler(server, cap)
	handler := mcpHandler.toolHandler.(*ToolHandler)
	return &interactHelpersTestEnv{handler: handler, server: server, capture: cap}
}

// enablePilot activates pilot mode using the test helper.
func (e *interactHelpersTestEnv) enablePilot(t *testing.T) {
	t.Helper()
	e.capture.SetPilotEnabled(true)
}

// ============================================
// queueComposableSubtitle
// ============================================

func TestQueueComposableSubtitle_QueuesPendingQuery(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueComposableSubtitle(req, "Test subtitle text")

	// Verify a pending query was created
	queries := env.capture.GetPendingQueries()
	found := false
	for _, q := range queries {
		if q.Type == "subtitle" {
			found = true
			// Verify the params contain the text
			var params map[string]string
			if err := json.Unmarshal(q.Params, &params); err != nil {
				t.Fatalf("failed to parse subtitle params: %v", err)
			}
			if params["text"] != "Test subtitle text" {
				t.Errorf("expected text 'Test subtitle text', got %q", params["text"])
			}
			break
		}
	}
	if !found {
		t.Error("expected a pending query of type 'subtitle' to be created")
	}
}

func TestQueueComposableSubtitle_CorrelationIDHasPrefix(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueComposableSubtitle(req, "text")

	queries := env.capture.GetPendingQueries()
	for _, q := range queries {
		if q.Type == "subtitle" {
			if len(q.CorrelationID) < 9 || q.CorrelationID[:9] != "subtitle_" {
				t.Errorf("expected correlation_id to start with 'subtitle_', got %q", q.CorrelationID)
			}
			return
		}
	}
	t.Error("subtitle query not found")
}

func TestQueueComposableSubtitle_EmptyText(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	// Empty text is valid (clears the subtitle)
	env.handler.queueComposableSubtitle(req, "")

	queries := env.capture.GetPendingQueries()
	found := false
	for _, q := range queries {
		if q.Type == "subtitle" {
			found = true
			var params map[string]string
			if err := json.Unmarshal(q.Params, &params); err != nil {
				t.Fatalf("failed to parse params: %v", err)
			}
			if params["text"] != "" {
				t.Errorf("expected empty text, got %q", params["text"])
			}
		}
	}
	if !found {
		t.Error("expected subtitle query even for empty text")
	}
}

func TestQueueComposableSubtitle_UniqueCorrelationIDs(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueComposableSubtitle(req, "first")
	env.handler.queueComposableSubtitle(req, "second")

	queries := env.capture.GetPendingQueries()
	ids := make(map[string]bool)
	for _, q := range queries {
		if q.Type == "subtitle" {
			if ids[q.CorrelationID] {
				t.Errorf("duplicate correlation_id: %q", q.CorrelationID)
			}
			ids[q.CorrelationID] = true
		}
	}
	if len(ids) < 2 {
		t.Errorf("expected at least 2 unique subtitle correlation IDs, got %d", len(ids))
	}
}

// ============================================
// queueStateNavigation
// ============================================

func TestQueueStateNavigation_QueuesBrowserAction(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	stateData := map[string]any{
		"url":   "https://example.com/page",
		"title": "Test Page",
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueStateNavigation(req, stateData)

	// Should have queued a browser_action query
	queries := env.capture.GetPendingQueries()
	found := false
	for _, q := range queries {
		if q.Type == "browser_action" {
			found = true
			var params map[string]any
			if err := json.Unmarshal(q.Params, &params); err != nil {
				t.Fatalf("failed to parse nav params: %v", err)
			}
			if params["action"] != "navigate" {
				t.Errorf("expected action 'navigate', got %v", params["action"])
			}
			if params["url"] != "https://example.com/page" {
				t.Errorf("expected url 'https://example.com/page', got %v", params["url"])
			}
			break
		}
	}
	if !found {
		t.Error("expected a pending query of type 'browser_action'")
	}

	// Should have mutated stateData
	if stateData["navigation_queued"] != true {
		t.Error("expected navigation_queued=true in stateData")
	}
	if _, ok := stateData["correlation_id"].(string); !ok {
		t.Error("expected correlation_id string in stateData")
	}
}

func TestQueueStateNavigation_SkipsWhenPilotDisabled(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	// Do NOT enable pilot

	stateData := map[string]any{
		"url":   "https://example.com/page",
		"title": "Test Page",
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueStateNavigation(req, stateData)

	queries := env.capture.GetPendingQueries()
	for _, q := range queries {
		if q.Type == "browser_action" {
			t.Error("should not queue browser_action when pilot is disabled")
		}
	}

	if _, ok := stateData["navigation_queued"]; ok {
		t.Error("stateData should not have navigation_queued when pilot is disabled")
	}
}

func TestQueueStateNavigation_SkipsWhenURLEmpty(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	stateData := map[string]any{
		"url":   "",
		"title": "No URL",
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueStateNavigation(req, stateData)

	queries := env.capture.GetPendingQueries()
	for _, q := range queries {
		if q.Type == "browser_action" {
			t.Error("should not queue browser_action when URL is empty")
		}
	}
}

func TestQueueStateNavigation_SkipsWhenURLMissing(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	stateData := map[string]any{
		"title": "No URL key",
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueStateNavigation(req, stateData)

	queries := env.capture.GetPendingQueries()
	for _, q := range queries {
		if q.Type == "browser_action" {
			t.Error("should not queue browser_action when URL key is missing")
		}
	}
}

func TestQueueStateNavigation_SkipsWhenURLNotString(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	stateData := map[string]any{
		"url": 12345, // not a string
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueStateNavigation(req, stateData)

	queries := env.capture.GetPendingQueries()
	for _, q := range queries {
		if q.Type == "browser_action" {
			t.Error("should not queue browser_action when URL is not a string")
		}
	}
}

func TestQueueStateNavigation_CorrelationIDHasNavPrefix(t *testing.T) {
	t.Parallel()
	env := newInteractHelpersTestEnv(t)
	env.enablePilot(t)

	stateData := map[string]any{
		"url": "https://example.com",
	}
	req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	env.handler.queueStateNavigation(req, stateData)

	corrID, ok := stateData["correlation_id"].(string)
	if !ok {
		t.Fatal("expected correlation_id to be set")
	}
	if len(corrID) < 4 || corrID[:4] != "nav_" {
		t.Errorf("expected correlation_id to start with 'nav_', got %q", corrID)
	}
}
