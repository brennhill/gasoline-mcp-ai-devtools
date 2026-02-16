// dashboard.go — Serves the embedded HTML dashboard at GET / and the status API.
package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

//go:embed dashboard.html
var dashboardHTML []byte

//go:embed diagnostics.html
var diagnosticsHTML []byte

// handleDashboard serves the HTML dashboard for browser access.
// If the client sends Accept: application/json, falls back to the JSON discovery response.
func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "Not found"})
		return
	}
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	// Content negotiation: JSON for programmatic clients, HTML for browsers
	accept := r.Header.Get("Accept")
	if accept == "application/json" || (!strings.Contains(accept, "text/html") && strings.Contains(accept, "application/json")) {
		jsonResponse(w, http.StatusOK, map[string]string{
			"name":    "gasoline",
			"version": version,
			"health":  "/health",
			"logs":    "/logs",
		})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(dashboardHTML); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] failed to write dashboard response: %v\n", err)
	}
}

// handleStatusAPI serves GET /api/status with aggregated data for the dashboard.
func handleStatusAPI(server *Server, cap *capture.Capture, mcpHandler *MCPHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
			return
		}

		resp := map[string]any{
			"version":        version,
			"uptime_seconds": int(time.Since(startTime).Seconds()),
			"pid":            os.Getpid(),
			"platform":       runtime.GOOS + "/" + runtime.GOARCH,
		}

		buffers := map[string]any{
			"console_entries":  server.getEntryCount(),
			"console_capacity": server.maxEntries,
		}

		if cap != nil {
			snap := cap.GetHealthSnapshot()

			resp["extension_connected"] = cap.IsExtensionConnected()
			resp["pilot_enabled"] = snap.PilotEnabled
			if !snap.LastPollTime.IsZero() {
				resp["last_poll_at"] = snap.LastPollTime.Format(time.RFC3339)
			}

			buffers["network_entries"] = snap.NetworkBodyCount
			buffers["network_capacity"] = capture.MaxNetworkBodies
			buffers["websocket_entries"] = snap.WebSocketCount
			buffers["websocket_capacity"] = capture.MaxWSEvents
			buffers["action_entries"] = snap.ActionCount
			buffers["action_capacity"] = capture.MaxEnhancedActions

			resp["recent_commands"] = buildRecentCommands(cap.GetHTTPDebugLog())
		} else {
			resp["extension_connected"] = false
			resp["pilot_enabled"] = false
		}

		resp["buffers"] = buffers

		if mcpHandler != nil && mcpHandler.toolHandler != nil {
			if th, ok := mcpHandler.toolHandler.(*ToolHandler); ok && th.healthMetrics != nil {
				resp["audit"] = th.healthMetrics.buildAuditInfo()
			}
		}

		jsonResponse(w, http.StatusOK, resp)
	}
}

// handleDiagnosticsHTML serves the diagnostics HTML page.
func handleDiagnosticsHTML(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(diagnosticsHTML); err != nil {
		fmt.Fprintf(os.Stderr, "[gasoline] failed to write diagnostics HTML response: %v\n", err)
	}
}

// recentCommand is a simplified view of an HTTP debug entry for the dashboard.
type recentCommand struct {
	Timestamp  time.Time `json:"timestamp"`
	Endpoint   string    `json:"endpoint"`
	Method     string    `json:"method"`
	Status     int       `json:"status"`
	DurationMs int64     `json:"duration_ms"`
}

// buildRecentCommands filters and sorts HTTP debug entries for the dashboard.
// Returns the most recent entries (newest first), excluding empty circular buffer slots.
func buildRecentCommands(entries []capture.HTTPDebugEntry) []recentCommand {
	var result []recentCommand
	for _, e := range entries {
		if e.Timestamp.IsZero() {
			continue
		}
		result = append(result, recentCommand{
			Timestamp:  e.Timestamp,
			Endpoint:   e.Endpoint,
			Method:     e.Method,
			Status:     e.ResponseStatus,
			DurationMs: e.DurationMs,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if len(result) > 15 {
		result = result[:15]
	}
	return result
}
