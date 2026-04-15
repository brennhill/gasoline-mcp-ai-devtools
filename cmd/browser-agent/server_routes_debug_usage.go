// server_routes_debug_usage.go — Debug endpoints for telemetry usage inspection.
// Why: Exposes UsageTracker state and beacon payload for smoke testing the analytics pipeline.
// Gated behind KABOOM_DEBUG=1 environment variable — not registered in production.

package main

import (
	"net/http"
	"os"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// debugEndpointsEnabled returns true when KABOOM_DEBUG=1 is set.
func debugEndpointsEnabled() bool {
	return os.Getenv("KABOOM_DEBUG") == "1"
}

// handleDebugUsage returns the current UsageTracker snapshot without resetting.
// GET /debug/usage → {"counts": {"observe:page": 3, "interact:click": 1, ...}}
func handleDebugUsage(mcp *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tracker := mcp.GetUsageTracker()
		if tracker == nil {
			jsonResponse(w, http.StatusOK, map[string]any{"counts": map[string]int{}})
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{"counts": tracker.Peek()})
	}
}

// handleDebugBeaconFlush forces an immediate SwapAndReset on the UsageTracker
// and returns the beacon payload that would be sent (with iid, sid, tool_stats).
// Does NOT fire a real beacon — returns the payload for inspection only.
// POST /debug/beacon-flush → {"payload": {...}, "flushed": true}
func handleDebugBeaconFlush(mcp *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tracker := mcp.GetUsageTracker()
		if tracker == nil {
			jsonResponse(w, http.StatusOK, map[string]any{
				"payload": nil,
				"flushed": 0,
				"message": "no usage tracker available",
			})
			return
		}
		snapshot := tracker.SwapAndReset()
		if snapshot == nil {
			jsonResponse(w, http.StatusOK, map[string]any{
				"payload": nil,
				"flushed": 0,
				"message": "no activity since last flush",
			})
			return
		}

		// Build the payload to return for inspection (does not fire a real beacon).
		payload := telemetry.BuildUsageSummaryPayload(0, snapshot)

		jsonResponse(w, http.StatusOK, map[string]any{
			"payload": payload,
			"flushed": len(snapshot.ToolStats),
		})
	}
}
