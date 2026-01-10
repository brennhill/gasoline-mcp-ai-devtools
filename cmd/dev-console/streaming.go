// streaming.go — Active context streaming via MCP notifications.
// When enabled, significant browser events are pushed as MCP notifications
// to stdout without requiring explicit tool calls. Provides configure_streaming
// tool for enable/disable/status, with throttling, dedup, and rate limiting.
package main

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// ============================================
// Constants
// ============================================

const (
	defaultThrottleSeconds    = 5
	defaultSeverityMin        = "warning"
	maxNotificationsPerMinute = 12
	dedupWindow               = 30 * time.Second
	maxPendingBatch           = 100
)

// mcpStdoutMu protects stdout writes from interleaving between MCP responses
// (written by the main goroutine) and streaming notifications (written by
// background goroutines via EmitAlert). Both paths must acquire this mutex.
var mcpStdoutMu sync.Mutex

// ============================================
// Types
// ============================================

// StreamConfig holds the user-configured streaming settings.
type StreamConfig struct {
	Enabled         bool     `json:"enabled"`
	Events          []string `json:"events"`
	ThrottleSeconds int      `json:"throttle_seconds"`
	URLFilter       string   `json:"url"`
	SeverityMin     string   `json:"severity_min"`
}

// StreamState manages active context streaming.
type StreamState struct {
	Config       StreamConfig
	LastNotified time.Time
	SeenMessages map[string]time.Time // dedupKey → last sent
	NotifyCount  int                  // count in current minute
	MinuteStart  time.Time
	PendingBatch []Alert
	mu           sync.Mutex
	writer       io.Writer // defaults to os.Stdout
}

// MCPNotification is the MCP notification format for streaming alerts.
type MCPNotification struct {
	JSONRPC string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  NotificationParams `json:"params"`
}

// NotificationParams holds the notification payload.
type NotificationParams struct {
	Level  string      `json:"level"`
	Logger string      `json:"logger"`
	Data   interface{} `json:"data"`
}

// ============================================
// Constructor
// ============================================

// NewStreamState creates a new StreamState with defaults.
func NewStreamState() *StreamState {
	return &StreamState{
		Config: StreamConfig{
			Enabled:         false,
			Events:          []string{"all"},
			ThrottleSeconds: defaultThrottleSeconds,
			SeverityMin:     defaultSeverityMin,
		},
		SeenMessages: make(map[string]time.Time),
		MinuteStart:  time.Now(),
		writer:       os.Stdout,
	}
}

// ============================================
// Configuration
// ============================================

// Configure handles the configure_streaming tool actions.
// Returns a map suitable for JSON serialization in tool response.
func (s *StreamState) Configure(action string, events []string, throttle int, urlFilter string, severityMin string) map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch action {
	case "enable":
		s.Config.Enabled = true
		if len(events) > 0 {
			s.Config.Events = events
		} else {
			s.Config.Events = []string{"all"}
		}
		if throttle > 0 {
			s.Config.ThrottleSeconds = throttle
		} else {
			s.Config.ThrottleSeconds = defaultThrottleSeconds
		}
		if urlFilter != "" {
			s.Config.URLFilter = urlFilter
		}
		if severityMin != "" {
			s.Config.SeverityMin = severityMin
		} else {
			s.Config.SeverityMin = defaultSeverityMin
		}
		return map[string]interface{}{
			"status": "enabled",
			"config": s.Config,
		}

	case "disable":
		s.Config.Enabled = false
		pendingCount := len(s.PendingBatch)
		s.PendingBatch = nil
		s.SeenMessages = make(map[string]time.Time)
		s.NotifyCount = 0
		return map[string]interface{}{
			"status":          "disabled",
			"pending_cleared": pendingCount,
		}

	case "status":
		return map[string]interface{}{
			"config":       s.Config,
			"notify_count": s.NotifyCount,
			"pending":      len(s.PendingBatch),
		}

	default:
		return map[string]interface{}{
			"error": "unknown action: " + action + ". Use enable, disable, or status.",
		}
	}
}

// ============================================
// Emission Filtering
// ============================================

// shouldEmit checks if an alert passes all configured filters.
func (s *StreamState) shouldEmit(alert Alert) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.Config.Enabled {
		return false
	}

	// Severity filter
	if severityRank(alert.Severity) < severityRank(s.Config.SeverityMin) {
		return false
	}

	// Category/event filter
	if !s.matchesEventFilter(alert.Category) {
		return false
	}

	return true
}

// matchesEventFilter checks if an alert category matches the configured event filters.
// Must be called with s.mu held.
func (s *StreamState) matchesEventFilter(category string) bool {
	for _, evt := range s.Config.Events {
		if evt == "all" {
			return true
		}
		if categoryMatchesEvent(category, evt) {
			return true
		}
	}
	return false
}

// categoryMatchesEvent maps alert categories to streaming event types.
func categoryMatchesEvent(category, event string) bool {
	switch event {
	case "errors":
		return category == "anomaly" || category == "threshold"
	case "network_errors":
		return category == "anomaly"
	case "performance":
		return category == "regression"
	case "regression":
		return category == "regression"
	case "anomaly":
		return category == "anomaly"
	case "ci":
		return category == "ci"
	case "security":
		return category == "threshold"
	case "user_frustration":
		return category == "anomaly"
	}
	return false
}

// ============================================
// Throttling and Rate Limiting
// ============================================

// canEmitAt checks if enough time has passed since the last notification.
func (s *StreamState) canEmitAt(now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check throttle
	if !s.LastNotified.IsZero() {
		elapsed := now.Sub(s.LastNotified)
		if elapsed < time.Duration(s.Config.ThrottleSeconds)*time.Second {
			return false
		}
	}

	// Check rate limit
	s.checkRateResetLocked(now)
	return s.NotifyCount < maxNotificationsPerMinute
}

// recordEmission records that a notification was sent.
func (s *StreamState) recordEmission(now time.Time, alert Alert) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastNotified = now
	s.checkRateResetLocked(now)
	s.NotifyCount++
}

// checkRateReset resets the per-minute counter if a new minute has started.
func (s *StreamState) checkRateReset(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checkRateResetLocked(now)
}

// checkRateResetLocked resets counter (caller must hold mu).
func (s *StreamState) checkRateResetLocked(now time.Time) {
	if now.Sub(s.MinuteStart) >= time.Minute {
		s.NotifyCount = 0
		s.MinuteStart = now
	}
}

// ============================================
// Deduplication
// ============================================

// isDuplicate checks if a dedup key was seen within the dedup window.
func (s *StreamState) isDuplicate(key string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lastSeen, ok := s.SeenMessages[key]; ok {
		if now.Sub(lastSeen) < dedupWindow {
			return true
		}
	}
	return false
}

// recordDedupKey records that a message with this key was sent.
func (s *StreamState) recordDedupKey(key string, now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.SeenMessages[key] = now

	// Prune old entries
	for k, t := range s.SeenMessages {
		if now.Sub(t) > dedupWindow {
			delete(s.SeenMessages, k)
		}
	}
}

// ============================================
// MCP Notification Format
// ============================================

// formatMCPNotification creates an MCP notification from an alert.
func formatMCPNotification(alert Alert) MCPNotification {
	return MCPNotification{
		JSONRPC: "2.0",
		Method:  "notifications/message",
		Params: NotificationParams{
			Level:  alert.Severity,
			Logger: "gasoline",
			Data: map[string]interface{}{
				"category":  alert.Category,
				"severity":  alert.Severity,
				"title":     alert.Title,
				"detail":    alert.Detail,
				"timestamp": alert.Timestamp,
				"source":    alert.Source,
			},
		},
	}
}

// ============================================
// Emission
// ============================================

// EmitAlert atomically checks all filters and emits an MCP notification if appropriate.
// Uses a single lock acquisition to prevent TOCTOU races between filter checks and emission.
func (s *StreamState) EmitAlert(alert Alert) {
	s.mu.Lock()

	// === Filter checks (inlined from shouldEmit) ===
	if !s.Config.Enabled {
		s.mu.Unlock()
		return
	}
	if severityRank(alert.Severity) < severityRank(s.Config.SeverityMin) {
		s.mu.Unlock()
		return
	}
	if !s.matchesEventFilter(alert.Category) {
		s.mu.Unlock()
		return
	}

	now := time.Now()

	// === Throttle + rate limit check (inlined from canEmitAt) ===
	canEmit := true
	if !s.LastNotified.IsZero() {
		if now.Sub(s.LastNotified) < time.Duration(s.Config.ThrottleSeconds)*time.Second {
			canEmit = false
		}
	}
	if canEmit {
		s.checkRateResetLocked(now)
		if s.NotifyCount >= maxNotificationsPerMinute {
			canEmit = false
		}
	}
	if !canEmit {
		if len(s.PendingBatch) < maxPendingBatch {
			s.PendingBatch = append(s.PendingBatch, alert)
		}
		s.mu.Unlock()
		return
	}

	// === Dedup check (inlined from isDuplicate) ===
	dedupKey := alert.Category + ":" + alert.Title
	if lastSeen, ok := s.SeenMessages[dedupKey]; ok {
		if now.Sub(lastSeen) < dedupWindow {
			s.mu.Unlock()
			return
		}
	}

	// === Record emission state ===
	s.LastNotified = now
	s.checkRateResetLocked(now)
	s.NotifyCount++
	s.SeenMessages[dedupKey] = now

	// Prune stale dedup entries
	for k, t := range s.SeenMessages {
		if now.Sub(t) > dedupWindow {
			delete(s.SeenMessages, k)
		}
	}

	w := s.writer
	s.mu.Unlock()

	if w == nil {
		return
	}

	// === Emit notification (outside StreamState lock, under stdout lock) ===
	notification := formatMCPNotification(alert)
	data, err := json.Marshal(notification)
	if err != nil {
		return
	}
	mcpStdoutMu.Lock()
	_, _ = w.Write(data)
	_, _ = w.Write([]byte{'\n'})
	mcpStdoutMu.Unlock()
}

// ============================================
// Tool Handler: configure_streaming
// ============================================

// toolConfigureStreaming handles the configure_streaming MCP tool call.
func (h *ToolHandler) toolConfigureStreaming(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Action          string   `json:"action"`
		Events          []string `json:"events"`
		ThrottleSeconds int      `json:"throttle_seconds"`
		URLFilter       string   `json:"url"`
		SeverityMin     string   `json:"severity_min"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if params.Action == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'action' is missing", "Add the 'action' parameter and call again", withParam("action"))}
	}

	result := h.streamState.Configure(params.Action, params.Events, params.ThrottleSeconds, params.URLFilter, params.SeverityMin)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Streaming configuration", result)}
}
