// server_routes_debug_usage.go — Debug endpoints for telemetry usage inspection.
// Why: Exposes UsageCounter state and beacon payload for smoke testing the analytics pipeline.
// Gated behind KABOOM_DEBUG=1 environment variable — not registered in production.

package main

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

// debugEndpointsEnabled returns true when KABOOM_DEBUG=1 is set.
func debugEndpointsEnabled() bool {
	return os.Getenv("KABOOM_DEBUG") == "1"
}

// handleDebugUsage returns the current UsageCounter snapshot without resetting.
// GET /debug/usage → {"counts": {"observe:page": 3, "interact:click": 1, ...}}
func handleDebugUsage(mcp *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		counter := mcp.GetUsageCounter()
		if counter == nil {
			writeJSON(w, http.StatusOK, map[string]any{"counts": map[string]int{}})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"counts": counter.Peek()})
	}
}

// handleDebugBeaconFlush forces an immediate SwapAndReset on the UsageCounter
// and returns the beacon payload that would be sent (with iid, sid, props).
// Also fires the real beacon so the full pipeline is exercised.
// POST /debug/beacon-flush → {"payload": {"event":"usage_summary","iid":"...","sid":"...","props":{...},...}}
func handleDebugBeaconFlush(mcp *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		counter := mcp.GetUsageCounter()
		if counter == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"payload":  nil,
				"flushed":  0,
				"message":  "no usage counter available",
			})
			return
		}
		snapshot := counter.SwapAndReset()
		if len(snapshot) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{
				"payload":  nil,
				"flushed":  0,
				"message":  "no activity since last flush",
			})
			return
		}

		// Build the payload to return for inspection (does not fire a real beacon).
		payload := telemetry.BuildUsageSummaryPayload(0, snapshot)

		writeJSON(w, http.StatusOK, map[string]any{
			"payload": payload,
			"flushed": len(snapshot),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
