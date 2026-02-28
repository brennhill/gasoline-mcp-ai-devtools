// tools_configure_network_recording.go — Passive network traffic recording with start/stop.

package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// recordingSnapshot holds the captured state from a recording session.
type recordingSnapshot struct {
	Active    bool
	StartTime time.Time
	Domain    string
	Method    string
}

// networkRecordingState tracks an active network recording session.
type networkRecordingState struct {
	mu        sync.Mutex
	active    bool
	startTime time.Time
	domain    string // optional domain filter
	method    string // optional HTTP method filter
}

// tryStart atomically checks if recording is inactive and starts it.
// Returns (startTime, true) on success, or (zero, false) if already active.
func (s *networkRecordingState) tryStart(domain, method string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active {
		return time.Time{}, false
	}
	s.active = true
	s.startTime = time.Now()
	s.domain = domain
	s.method = method
	return s.startTime, true
}

// stop atomically stops recording and returns a snapshot of the session state.
// Returns (snapshot, true) if recording was active, or (zero, false) if not.
func (s *networkRecordingState) stop() (recordingSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active {
		return recordingSnapshot{}, false
	}
	snap := recordingSnapshot{
		Active:    true,
		StartTime: s.startTime,
		Domain:    s.domain,
		Method:    s.method,
	}
	s.active = false
	s.startTime = time.Time{}
	s.domain = ""
	s.method = ""
	return snap, true
}

// info returns a snapshot of the current recording state.
func (s *networkRecordingState) info() recordingSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return recordingSnapshot{
		Active:    s.active,
		StartTime: s.startTime,
		Domain:    s.domain,
		Method:    s.method,
	}
}

// toolConfigureNetworkRecording handles configure(what="network_recording").
func (h *ToolHandler) toolConfigureNetworkRecording(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Operation string `json:"operation"`
		Domain    string `json:"domain"`
		Method    string `json:"method"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidJSON,
				"Invalid JSON arguments: "+err.Error(),
				"Fix JSON syntax and call again",
			)}
		}
	}

	switch params.Operation {
	case "start":
		startedAt, ok := h.networkRecording.tryStart(params.Domain, params.Method)
		if !ok {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"Network recording already active",
				"Stop the current recording first with operation='stop'.",
			)}
		}
		result := map[string]any{
			"status":     "recording",
			"started_at": startedAt.Format(time.RFC3339),
		}
		if params.Domain != "" {
			result["domain_filter"] = params.Domain
		}
		if params.Method != "" {
			result["method_filter"] = params.Method
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network recording started", result)}

	case "stop":
		snap, wasActive := h.networkRecording.stop()
		if !wasActive {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"No active network recording",
				"Start a recording first with operation='start'.",
			)}
		}

		// Collect network bodies captured since start time
		bodies := h.capture.GetNetworkBodies()
		var recorded []map[string]any
		for _, b := range bodies {
			if !matchesRecordingFilter(b, snap.StartTime, snap.Domain, snap.Method) {
				continue
			}
			entry := map[string]any{
				"method": b.Method,
				"url":    b.URL,
				"status": b.Status,
			}
			if b.RequestBody != "" {
				entry["request_body"] = b.RequestBody
			}
			if b.ResponseBody != "" {
				entry["response_body"] = b.ResponseBody
			}
			if b.ContentType != "" {
				entry["content_type"] = b.ContentType
			}
			if b.Duration > 0 {
				entry["duration_ms"] = b.Duration
			}
			if b.HasAuthHeader {
				entry["has_auth_header"] = true
			}
			if b.Timestamp != "" {
				entry["timestamp"] = b.Timestamp
			}
			recorded = append(recorded, entry)
		}

		duration := time.Since(snap.StartTime)
		result := map[string]any{
			"status":      "stopped",
			"duration_ms": int(duration.Milliseconds()),
			"requests":    recorded,
			"count":       len(recorded),
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network recording stopped", result)}

	case "status", "":
		snap := h.networkRecording.info()
		result := map[string]any{
			"active": snap.Active,
		}
		if snap.Active {
			result["started_at"] = snap.StartTime.Format(time.RFC3339)
			result["duration_ms"] = int(time.Since(snap.StartTime).Milliseconds())
			if snap.Domain != "" {
				result["domain_filter"] = snap.Domain
			}
			if snap.Method != "" {
				result["method_filter"] = snap.Method
			}
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Network recording status", result)}

	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Unknown operation: "+params.Operation,
			"Use 'start', 'stop', or 'status'.",
		)}
	}
}

// matchesRecordingFilter checks if a network body matches recording filters.
func matchesRecordingFilter(b types.NetworkBody, startTime time.Time, domain, method string) bool {
	// Filter by timestamp — only include entries captured after recording started
	if b.Timestamp != "" {
		ts, err := time.Parse(time.RFC3339Nano, b.Timestamp)
		if err != nil {
			// Try millisecond epoch format (extension may send numeric timestamps)
			if msEpoch, numErr := strconv.ParseInt(b.Timestamp, 10, 64); numErr == nil {
				ts = time.UnixMilli(msEpoch)
				err = nil
			}
		}
		if err == nil && ts.Before(startTime) {
			return false
		}
		// If both RFC3339 and epoch parsing fail, include the entry (best-effort)
	}

	// Filter by domain
	if domain != "" && !strings.Contains(strings.ToLower(b.URL), strings.ToLower(domain)) {
		return false
	}

	// Filter by HTTP method
	if method != "" && !strings.EqualFold(b.Method, method) {
		return false
	}

	return true
}
