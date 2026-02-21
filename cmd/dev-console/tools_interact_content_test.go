// tools_interact_content_test.go — Tests for navigate include_content enrichment payloads.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dev-console/dev-console/internal/performance"
)

func TestEnrichNavigateResponse_IncludeContentAddsNavigationVitalsAndErrors(t *testing.T) {
	t.Parallel()
	h, server, cap := makeToolHandler(t)
	cap.SetTrackingStatusForTest(42, "https://example.com/page")

	// Seed one recent error for tracked tab context.
	server.addEntries([]LogEntry{{
		"level":   "error",
		"message": "TypeError: boom",
		"source":  "https://example.com/app.js",
		"url":     "https://example.com/page",
		"ts":      time.Now().UTC().Format(time.RFC3339),
		"tabId":   float64(42),
	}})

	// Seed vitals snapshot.
	cap.AddPerformanceSnapshots([]performance.PerformanceSnapshot{{
		URL:       "/page",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Timing: performance.PerformanceTiming{
			DomContentLoaded: 640,
			Load:             1180,
		},
	}})

	// Ensure nav_content summary query completes quickly if issued.
	go func() {
		deadline := time.Now().Add(300 * time.Millisecond)
		for time.Now().Before(deadline) {
			for _, q := range cap.GetPendingQueries() {
				if strings.HasPrefix(q.CorrelationID, "nav_content_") {
					cap.CompleteCommand(q.CorrelationID, json.RawMessage(`{"main_content_preview":"Welcome"}`), "")
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
	base := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpJSONResponse("Command nav complete", map[string]any{
			"correlation_id": "nav_123_456",
			"status":         "complete",
			"final":          true,
		}),
	}

	enriched := h.enrichNavigateResponse(base, req, 42)
	result := parseToolResult(t, enriched)
	if result.IsError {
		t.Fatalf("enriched response should not be isError=true, got: %s", result.Content[0].Text)
	}
	if len(result.Content) < 2 {
		t.Fatalf("expected enrichment content block, got %d blocks", len(result.Content))
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(extractJSONFromText(result.Content[1].Text)), &payload); err != nil {
		t.Fatalf("failed to parse enrichment JSON: %v", err)
	}

	if _, ok := payload["navigation"].(map[string]any); !ok {
		t.Fatalf("navigation payload missing or wrong type: %#v", payload["navigation"])
	}
	if _, ok := payload["vitals"].(map[string]any); !ok {
		t.Fatalf("vitals payload missing or wrong type: %#v", payload["vitals"])
	}
	if _, ok := payload["errors"].([]any); !ok {
		t.Fatalf("errors payload missing or wrong type: %#v", payload["errors"])
	}
}
