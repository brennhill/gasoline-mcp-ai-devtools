// stream.go — StreamState: active context streaming via MCP notifications.
package streaming

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/types"
)

// ============================================
// Constructor
// ============================================

// NewStreamState creates a new StreamState with defaults.
func NewStreamState() *StreamState {
	return &StreamState{
		Config: StreamConfig{
			Enabled:         false,
			Events:          []string{"all"},
			ThrottleSeconds: DefaultThrottleSeconds,
			SeverityMin:     DefaultSeverityMin,
		},
		SeenMessages: make(map[string]time.Time),
		MinuteStart:  time.Now(),
		Writer:       nil, // CRITICAL: Never write to stdout in MCP mode (causes parse errors in IDEs)
	}
}

// ============================================
// Configuration
// ============================================

// Configure handles streaming configuration actions (enable/disable/status).
func (s *StreamState) Configure(action string, events []string, throttle int, urlFilter string, severityMin string) map[string]any {
	s.Mu.Lock()
	defer s.Mu.Unlock()

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

// applyEnableConfig updates the stream config. Caller must hold s.Mu.
func (s *StreamState) applyEnableConfig(events []string, throttle int, urlFilter string, severityMin string) {
	s.Config.Enabled = true
	s.Config.Events = orDefault(events, []string{"all"})
	s.Config.ThrottleSeconds = orDefaultInt(throttle, DefaultThrottleSeconds)
	if urlFilter != "" {
		s.Config.URLFilter = urlFilter
	}
	s.Config.SeverityMin = orDefaultStr(severityMin, DefaultSeverityMin)
}

func orDefault(val []string, def []string) []string {
	if len(val) > 0 {
		return val
	}
	return def
}

func orDefaultInt(val int, def int) int {
	if val > 0 {
		return val
	}
	return def
}

func orDefaultStr(val string, def string) string {
	if val != "" {
		return val
	}
	return def
}

// ============================================
// Emission Filtering
// ============================================

// ShouldEmit checks if an alert passes all configured filters.
func (s *StreamState) ShouldEmit(alert types.Alert) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !s.Config.Enabled {
		return false
	}
	if SeverityRank(alert.Severity) < SeverityRank(s.Config.SeverityMin) {
		return false
	}
	return s.matchesEventFilter(alert.Category)
}

// matchesEventFilter checks if a category matches the configured event filters.
// Must be called with s.Mu held.
func (s *StreamState) matchesEventFilter(category string) bool {
	for _, evt := range s.Config.Events {
		if evt == "all" {
			return true
		}
		if CategoryMatchesEvent(category, evt) {
			return true
		}
	}
	return false
}

// eventCategoryMap maps streaming event types to matching alert categories.
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

// CategoryMatchesEvent maps alert categories to streaming event types.
func CategoryMatchesEvent(category, event string) bool {
	cats, ok := eventCategoryMap[event]
	return ok && cats[category]
}

// ============================================
// Throttling and Rate Limiting
// ============================================

// CanEmitAt checks if enough time has passed since the last notification.
func (s *StreamState) CanEmitAt(now time.Time) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !s.LastNotified.IsZero() {
		elapsed := now.Sub(s.LastNotified)
		if elapsed < time.Duration(s.Config.ThrottleSeconds)*time.Second {
			return false
		}
	}
	s.checkRateResetLocked(now)
	return s.NotifyCount < MaxNotificationsPerMinute
}

// RecordEmission records that a notification was sent.
func (s *StreamState) RecordEmission(now time.Time, alert types.Alert) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	s.LastNotified = now
	s.checkRateResetLocked(now)
	s.NotifyCount++
}

// CheckRateReset resets the per-minute counter if a new minute has started.
func (s *StreamState) CheckRateReset(now time.Time) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.checkRateResetLocked(now)
}

// checkRateResetLocked resets counter (caller must hold Mu).
func (s *StreamState) checkRateResetLocked(now time.Time) {
	if now.Sub(s.MinuteStart) >= time.Minute {
		s.NotifyCount = 0
		s.MinuteStart = now
	}
}

// ============================================
// Deduplication
// ============================================

// IsDuplicate checks if a dedup key was seen within the dedup window.
func (s *StreamState) IsDuplicate(key string, now time.Time) bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if lastSeen, ok := s.SeenMessages[key]; ok {
		if now.Sub(lastSeen) < DedupWindow {
			return true
		}
	}
	return false
}

// RecordDedupKey records that a message with this key was sent.
func (s *StreamState) RecordDedupKey(key string, now time.Time) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	s.SeenMessages[key] = now

	for k, t := range s.SeenMessages {
		if now.Sub(t) > DedupWindow {
			delete(s.SeenMessages, k)
		}
	}
}

// ============================================
// Emission
// ============================================

// passesFiltersLocked checks enabled, severity, and event filters.
// Caller must hold s.Mu.
func (s *StreamState) passesFiltersLocked(alert types.Alert) bool {
	if !s.Config.Enabled {
		return false
	}
	if SeverityRank(alert.Severity) < SeverityRank(s.Config.SeverityMin) {
		return false
	}
	return s.matchesEventFilter(alert.Category)
}

// canEmitAtLocked checks throttle and rate limit constraints.
// Caller must hold s.Mu.
func (s *StreamState) canEmitAtLocked(now time.Time) bool {
	if !s.LastNotified.IsZero() {
		if now.Sub(s.LastNotified) < time.Duration(s.Config.ThrottleSeconds)*time.Second {
			return false
		}
	}
	s.checkRateResetLocked(now)
	return s.NotifyCount < MaxNotificationsPerMinute
}

// isDuplicateLocked checks if this alert was recently emitted.
// Caller must hold s.Mu.
func (s *StreamState) isDuplicateLocked(dedupKey string, now time.Time) bool {
	if lastSeen, ok := s.SeenMessages[dedupKey]; ok {
		return now.Sub(lastSeen) < DedupWindow
	}
	return false
}

// recordEmissionLocked updates emission state and prunes stale dedup entries.
// Caller must hold s.Mu.
func (s *StreamState) recordEmissionLocked(dedupKey string, now time.Time) {
	s.LastNotified = now
	s.checkRateResetLocked(now)
	s.NotifyCount++
	s.SeenMessages[dedupKey] = now

	for k, t := range s.SeenMessages {
		if now.Sub(t) > DedupWindow {
			delete(s.SeenMessages, k)
		}
	}
}

// EmitAlert atomically checks all filters and emits an MCP notification if appropriate.
func (s *StreamState) EmitAlert(alert types.Alert) {
	s.Mu.Lock()

	if !s.passesFiltersLocked(alert) {
		s.Mu.Unlock()
		return
	}

	now := time.Now()
	if !s.canEmitAtLocked(now) {
		if len(s.PendingBatch) < MaxPendingBatch {
			s.PendingBatch = append(s.PendingBatch, alert)
		}
		s.Mu.Unlock()
		return
	}

	dedupKey := alert.Category + ":" + alert.Title
	if s.isDuplicateLocked(dedupKey, now) {
		s.Mu.Unlock()
		return
	}

	s.recordEmissionLocked(dedupKey, now)
	w := s.Writer
	s.Mu.Unlock()

	notification := FormatMCPNotification(alert)
	if w != nil {
		data, err := json.Marshal(notification)
		if err == nil {
			_, _ = w.Write(data)
			_, _ = w.Write([]byte{'\n'})
		}
	}
}

// FormatMCPNotification creates an MCP notification from an alert.
func FormatMCPNotification(alert types.Alert) MCPNotification {
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
