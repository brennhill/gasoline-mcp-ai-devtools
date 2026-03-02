// Purpose: Implements server health, diagnostics, and shutdown HTTP endpoints.
// Why: Keeps operational endpoint behavior separate from route registration wiring.

package main

import (
	"net/http"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// handleHealth serves the /health endpoint with server status and version info.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	logFileSize := int64(0)
	if fi, err := os.Stat(s.logFile); err == nil {
		logFileSize = fi.Size()
	}

	availVer := getAvailableVersion()

	resp := map[string]any{
		"status":       "ok",
		"service-name": "gasoline-agentic-browser",
		"name":         "gasoline-agentic-browser", // legacy compatibility
		"version":      version,
		"logs": map[string]any{
			"entries":       s.getEntryCount(),
			"max_entries":   s.maxEntries,
			"log_file":      s.logFile,
			"log_file_size": logFileSize,
			"dropped_count": s.getLogDropCount(),
		},
	}
	successReads, failedReads := snapshotFastPathResourceReadCounters()
	resp["bridge_fastpath"] = map[string]any{
		"resources_read_success": successReads,
		"resources_read_failure": failedReads,
	}
	if availVer != "" {
		resp["available_version"] = availVer
	}
	if info := buildUpgradeInfo(); info != nil {
		resp["upgrade_pending"] = info
	}
	if cap != nil {
		extStatus := cap.GetExtensionStatus()
		pilotStatus, _ := cap.GetPilotStatus().(map[string]any)
		pilotState, _ := pilotStatus["state"].(string)
		securityMode, productionParity, rewrites := cap.GetSecurityMode()
		resp["capture"] = map[string]any{
			"available":           true,
			"pilot_enabled":       cap.IsPilotActionAllowed(),
			"pilot_state":         pilotState,
			"extension_connected": cap.IsExtensionConnected(),
			"extension_last_seen": extStatus["last_seen"],
			"extension_client_id": extStatus["client_id"],
			"security_mode":       securityMode,
			"production_parity":   productionParity,
			"insecure_rewrites":   rewrites,
		}
	}
	jsonResponse(w, http.StatusOK, resp)
}

// handleShutdown initiates a graceful server shutdown via SIGTERM.
func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	_ = s.appendToFile([]LogEntry{{
		"type":      "lifecycle",
		"event":     "shutdown_requested",
		"source":    "http",
		"pid":       os.Getpid(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}})

	jsonResponse(w, http.StatusOK, map[string]string{
		"status":  "shutting_down",
		"message": "Server shutdown initiated",
	})

	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	util.SafeGo(func() {
		time.Sleep(100 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	})
}

// lastConsoleEvent returns a summary of the most recent console log entry.
func (s *Server) lastConsoleEvent() map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.entries) == 0 {
		return nil
	}
	last := s.entries[len(s.entries)-1]
	args := last["args"]
	if argsSlice, ok := args.([]any); ok && len(argsSlice) > 0 {
		if str, ok := argsSlice[0].(string); ok && len(str) > 100 {
			args = str[:100] + "..."
		} else {
			args = argsSlice[0]
		}
	}
	return map[string]any{
		"level":   last["level"],
		"message": args,
		"ts":      last["ts"],
	}
}

// handleDiagnostics serves the /diagnostics endpoint with debug information.
func (s *Server) handleDiagnostics(w http.ResponseWriter, r *http.Request, cap *capture.Capture) {
	if r.Method != "GET" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	now := time.Now()
	resp := map[string]any{
		"generated_at":   now.Format(time.RFC3339),
		"version":        version,
		"uptime_seconds": int(now.Sub(startTime).Seconds()),
		"system": map[string]any{
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"go_version": runtime.Version(),
			"goroutines": runtime.NumGoroutine(),
		},
		"logs": map[string]any{
			"entries":     s.getEntryCount(),
			"max_entries": s.maxEntries,
			"log_file":    s.logFile,
		},
	}

	if cap != nil {
		appendCaptureDiagnostics(resp, cap)
	}

	lastEvents := map[string]any{}
	if evt := s.lastConsoleEvent(); evt != nil {
		lastEvents["console"] = evt
	}
	resp["last_events"] = lastEvents

	if cap != nil {
		httpDebugLog := cap.GetHTTPDebugLog()
		resp["http_debug_log"] = map[string]any{
			"count":   len(httpDebugLog),
			"entries": httpDebugLog,
		}
	}

	jsonResponse(w, http.StatusOK, resp)
}

// appendCaptureDiagnostics adds capture-related diagnostic fields to response map.
func appendCaptureDiagnostics(resp map[string]any, cap *capture.Capture) {
	snap := cap.GetHealthSnapshot()
	health := cap.GetHealthStatus()

	resp["buffers"] = map[string]any{
		"websocket_events": snap.WebSocketCount,
		"network_bodies":   snap.NetworkBodyCount,
		"actions":          snap.ActionCount,
		"pending_queries":  snap.PendingQueryCount,
		"query_results":    snap.QueryResultCount,
	}

	wsStatus := cap.GetWebSocketStatus(capture.WebSocketStatusFilter{})
	conns := make([]map[string]any, 0, len(wsStatus.Connections))
	for _, c := range wsStatus.Connections {
		conns = append(conns, map[string]any{
			"id":  c.ID,
			"url": c.URL,
		})
	}
	resp["websocket_connections"] = conns

	resp["config"] = map[string]any{
		"query_timeout": snap.QueryTimeout.String(),
	}

	const defaultTraceLimit = 25
	traces := cap.GetRecentCommandTraces(defaultTraceLimit)
	traceEntries := make([]map[string]any, 0, len(traces))
	for _, trace := range traces {
		if trace == nil {
			continue
		}
		traceID := trace.TraceID
		if traceID == "" {
			traceID = trace.CorrelationID
		}
		traceEntries = append(traceEntries, map[string]any{
			"trace_id":       traceID,
			"correlation_id": trace.CorrelationID,
			"query_id":       trace.QueryID,
			"status":         trace.Status,
			"timeline":       trace.TraceTimeline,
			"events":         trace.TraceEvents,
			"created_at":     trace.CreatedAt.Format(time.RFC3339),
			"updated_at":     trace.UpdatedAt.Format(time.RFC3339),
		})
	}
	resp["command_traces"] = map[string]any{
		"count":   len(traceEntries),
		"limit":   defaultTraceLimit,
		"entries": traceEntries,
	}

	lastPoll := any(nil)
	if !snap.LastPollTime.IsZero() {
		lastPoll = snap.LastPollTime.Format(time.RFC3339)
	}
	resp["extension"] = map[string]any{
		"polling":       !snap.LastPollTime.IsZero(),
		"last_poll_at":  lastPoll,
		"ext_session":   snap.ExtSessionID,
		"pilot_enabled": snap.PilotEnabled,
	}

	resp["circuit"] = map[string]any{
		"open":         snap.CircuitOpen,
		"current_rate": health.CurrentRate,
		"reason":       snap.CircuitReason,
	}
}
