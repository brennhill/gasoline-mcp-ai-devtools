// Purpose: Owns streaming.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// streaming.go — Active context streaming via MCP notifications.
// When enabled, significant browser events are pushed as MCP notifications
// to stdout without requiring explicit tool calls. Provides configure_streaming
// tool for enable/disable/status, with throttling, dedup, and rate limiting.
package main

import (
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/types"
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

// Removed mcpStdoutMu - notifications write through a single stream writer.

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
	PendingBatch []types.Alert
	mu           sync.Mutex
	writer       io.Writer // defaults to os.Stdout (for testing)
}

// MCPNotification is the MCP notification format for streaming alerts.
type MCPNotification struct {
	JSONRPC string             `json:"jsonrpc"`
	Method  string             `json:"method"`
	Params  NotificationParams `json:"params"`
}

// NotificationParams holds the notification payload.
type NotificationParams struct {
	Level  string `json:"level"`
	Logger string `json:"logger"`
	Data   any    `json:"data"`
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
		writer:       nil, // CRITICAL: Never write to stdout in MCP mode (causes parse errors in IDEs)
	}
}

// ============================================
// Configuration
// ============================================

// applyEnableConfig updates the stream config with provided values or defaults.
// Caller must hold s.mu.
func (s *StreamState) applyEnableConfig(events []string, throttle int, urlFilter string, severityMin string) {
	s.Config.Enabled = true
	s.Config.Events = orDefault(events, []string{"all"})
	s.Config.ThrottleSeconds = orDefaultInt(throttle, defaultThrottleSeconds)
	if urlFilter != "" {
		s.Config.URLFilter = urlFilter
	}
	s.Config.SeverityMin = orDefaultStr(severityMin, defaultSeverityMin)
}

// orDefault returns val if non-empty, otherwise def.
func orDefault(val []string, def []string) []string {
	if len(val) > 0 {
		return val
	}
	return def
}

// orDefaultInt returns val if positive, otherwise def.
func orDefaultInt(val int, def int) int {
	if val > 0 {
		return val
	}
	return def
}

// orDefaultStr returns val if non-empty, otherwise def.
func orDefaultStr(val string, def string) string {
	if val != "" {
		return val
	}
	return def
}

// Configure handles the configure_streaming tool actions.
// Returns a map suitable for JSON serialization in tool response.
func (s *StreamState) Configure(action string, events []string, throttle int, urlFilter string, severityMin string) map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch action {
	case "enable":
		s.applyEnableConfig(events, throttle, urlFilter, severityMin)
		return map[string]any{"status": "enabled", "config": s.Config}

	case "disable":
		s.Config.Enabled = false
		pendingCount := len(s.PendingBatch)
		s.PendingBatch = nil
		s.SeenMessages = make(map[string]time.Time)
		s.NotifyCount = 0
		return map[string]any{"status": "disabled", "pending_cleared": pendingCount}

	case "status":
		return map[string]any{
			"config":       s.Config,
			"notify_count": s.NotifyCount,
			"pending":      len(s.PendingBatch),
		}

	default:
		return map[string]any{
			"error": "unknown action: " + action + ". Use enable, disable, or status.",
		}
	}
}

// ============================================
// Emission Filtering
// ============================================

// shouldEmit checks if an alert passes all configured filters.
func (s *StreamState) shouldEmit(alert types.Alert) bool {
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

// eventCategoryMap maps streaming event types to the set of matching alert categories.
var eventCategoryMap = map[string]map[string]bool{
	"errors":           {"anomaly": true, "threshold": true},
	"network_errors":   {"anomaly": true},
	"performance":      {"regression": true},
	"regression":       {"regression": true},
	"anomaly":          {"anomaly": true},
	"ci":               {"ci": true},
	"security":         {"threshold": true},
	"user_frustration": {"anomaly": true},
}

// categoryMatchesEvent maps alert categories to streaming event types.
func categoryMatchesEvent(category, event string) bool {
	cats, ok := eventCategoryMap[event]
	return ok && cats[category]
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
func (s *StreamState) recordEmission(now time.Time, alert types.Alert) {
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
func formatMCPNotification(alert types.Alert) MCPNotification {
	return MCPNotification{
		JSONRPC: "2.0",
		Method:  "notifications/message",
		Params: NotificationParams{
			Level:  alert.Severity,
			Logger: "gasoline",
			Data: map[string]any{
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

// passesFiltersLocked checks enabled, severity, and event filters.
// Caller must hold s.mu.
func (s *StreamState) passesFiltersLocked(alert types.Alert) bool {
	if !s.Config.Enabled {
		return false
	}
	if severityRank(alert.Severity) < severityRank(s.Config.SeverityMin) {
		return false
	}
	return s.matchesEventFilter(alert.Category)
}

// canEmitAtLocked checks throttle and rate limit constraints.
// Caller must hold s.mu.
func (s *StreamState) canEmitAtLocked(now time.Time) bool {
	if !s.LastNotified.IsZero() {
		if now.Sub(s.LastNotified) < time.Duration(s.Config.ThrottleSeconds)*time.Second {
			return false
		}
	}
	s.checkRateResetLocked(now)
	return s.NotifyCount < maxNotificationsPerMinute
}

// isDuplicateLocked checks if this alert was recently emitted.
// Caller must hold s.mu.
func (s *StreamState) isDuplicateLocked(dedupKey string, now time.Time) bool {
	if lastSeen, ok := s.SeenMessages[dedupKey]; ok {
		return now.Sub(lastSeen) < dedupWindow
	}
	return false
}

// recordEmissionLocked updates emission state and prunes stale dedup entries.
// Caller must hold s.mu.
func (s *StreamState) recordEmissionLocked(dedupKey string, now time.Time) {
	s.LastNotified = now
	s.checkRateResetLocked(now)
	s.NotifyCount++
	s.SeenMessages[dedupKey] = now

	for k, t := range s.SeenMessages {
		if now.Sub(t) > dedupWindow {
			delete(s.SeenMessages, k)
		}
	}
}

// EmitAlert atomically checks all filters and emits an MCP notification if appropriate.
// Uses a single lock acquisition to prevent TOCTOU races between filter checks and emission.
func (s *StreamState) EmitAlert(alert types.Alert) {
	s.mu.Lock()

	if !s.passesFiltersLocked(alert) {
		s.mu.Unlock()
		return
	}

	now := time.Now()
	if !s.canEmitAtLocked(now) {
		if len(s.PendingBatch) < maxPendingBatch {
			s.PendingBatch = append(s.PendingBatch, alert)
		}
		s.mu.Unlock()
		return
	}

	dedupKey := alert.Category + ":" + alert.Title
	if s.isDuplicateLocked(dedupKey, now) {
		s.mu.Unlock()
		return
	}

	s.recordEmissionLocked(dedupKey, now)
	w := s.writer
	s.mu.Unlock()

	notification := formatMCPNotification(alert)
	if w != nil {
		data, err := json.Marshal(notification)
		if err == nil {
			_, _ = w.Write(data)
			_, _ = w.Write([]byte{'\n'})
		}
	}
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
