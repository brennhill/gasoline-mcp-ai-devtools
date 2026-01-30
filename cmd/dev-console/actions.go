// actions.go — User action recording buffer and MCP tool handler.
// Captures browser interactions (click, input, navigate, scroll, select)
// with smart selectors (testId, role, ariaLabel, text, id, cssPath).
// Design: Fixed-size ring buffer. Sensitive input values are redacted
// at capture time. Selectors prioritized for Playwright compatibility.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ============================================
// Enhanced Actions
// ============================================

// AddEnhancedActions adds enhanced actions to the buffer
func (c *Capture) AddEnhancedActions(actions []EnhancedAction) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Enforce memory limits before adding
	c.enforceMemory()

	c.actionTotalAdded += int64(len(actions))
	now := time.Now()
	for i := range actions {
		// Redact password values on ingest
		if actions[i].InputType == "password" && actions[i].Value != "[redacted]" {
			actions[i].Value = "[redacted]"
		}
		c.enhancedActions = append(c.enhancedActions, actions[i])
		c.actionAddedAt = append(c.actionAddedAt, now)
	}

	// Enforce max count (respecting minimal mode)
	capacity := c.effectiveActionCapacity()
	if len(c.enhancedActions) > capacity {
		keep := len(c.enhancedActions) - capacity
		newActions := make([]EnhancedAction, capacity)
		copy(newActions, c.enhancedActions[keep:])
		c.enhancedActions = newActions
		newAddedAt := make([]time.Time, capacity)
		copy(newAddedAt, c.actionAddedAt[keep:])
		c.actionAddedAt = newAddedAt
	}
}

// GetEnhancedActionCount returns the current number of buffered actions
func (c *Capture) GetEnhancedActionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.enhancedActions)
}

// GetEnhancedActions returns filtered enhanced actions
func (c *Capture) GetEnhancedActions(filter EnhancedActionFilter) []EnhancedAction {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var filtered []EnhancedAction
	for i := range c.enhancedActions {
		// TTL filtering: skip entries older than TTL
		if c.TTL > 0 && i < len(c.actionAddedAt) && isExpiredByTTL(c.actionAddedAt[i], c.TTL) {
			continue
		}
		if filter.URLFilter != "" && !strings.Contains(c.enhancedActions[i].URL, filter.URLFilter) {
			continue
		}
		filtered = append(filtered, c.enhancedActions[i])
	}

	// Apply lastN (return most recent N)
	if filter.LastN > 0 && len(filtered) > filter.LastN {
		filtered = filtered[len(filtered)-filter.LastN:]
	}

	return filtered
}

func (c *Capture) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	body, ok := c.readIngestBody(w, r)
	if !ok {
		return
	}
	var payload struct {
		Actions []EnhancedAction `json:"actions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !c.recordAndRecheck(w, len(payload.Actions)) {
		return
	}
	c.AddEnhancedActions(payload.Actions)
	w.WriteHeader(http.StatusOK)
}

func (h *ToolHandler) toolGetEnhancedActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastN             int    `json:"last_n"`
		URL               string `json:"url"`
		AfterCursor       string `json:"after_cursor"`
		BeforeCursor      string `json:"before_cursor"`
		SinceCursor       string `json:"since_cursor"`
		RestartOnEviction bool   `json:"restart_on_eviction"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	// Acquire read lock to access raw buffer and total counter
	h.capture.mu.RLock()
	defer h.capture.mu.RUnlock()

	// Enrich entries with sequence numbers BEFORE filtering
	enriched := EnrichActionEntries(h.capture.enhancedActions, h.capture.actionTotalAdded)

	// Apply TTL and URL filters (preserving sequences)
	var filtered []ActionEntryWithSequence
	for i, e := range enriched {
		// TTL filtering: skip entries older than TTL
		if h.capture.TTL > 0 && i < len(h.capture.actionAddedAt) && isExpiredByTTL(h.capture.actionAddedAt[i], h.capture.TTL) {
			continue
		}
		// URL filter
		if arguments.URL != "" && !strings.Contains(e.Entry.URL, arguments.URL) {
			continue
		}
		filtered = append(filtered, e)
	}

	// Determine limit: use cursor limit if specified, otherwise last_n for backward compatibility
	limit := 0
	if arguments.LastN > 0 {
		limit = arguments.LastN
	}

	// Apply cursor-based pagination
	result, metadata, err := ApplyActionCursorPagination(
		filtered,
		arguments.AfterCursor,
		arguments.BeforeCursor,
		arguments.SinceCursor,
		limit,
		arguments.RestartOnEviction,
	)

	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrCursorExpired,
			err.Error(),
			"Use restart_on_eviction=true to auto-restart from oldest available, or reduce the time between pagination calls to prevent buffer overflow",
		)}
	}

	// Handle empty result
	if len(result) == 0 {
		msg := "No user actions captured"
		if h.captureOverrides != nil {
			overrides := h.captureOverrides.GetAll()
			if overrides["action_replay"] == "false" {
				msg += "\n\nAction replay capture is OFF. To enable, call:\nconfigure({action: \"capture\", settings: {action_replay: \"true\"}})"
			}
		}

		// Return JSON format even for empty result to maintain consistency
		data := map[string]interface{}{
			"actions": []map[string]interface{}{},
			"count":   0,
			"total":   metadata.Total,
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(msg, data)}
	}

	// Serialize actions to JSON format
	jsonActions := make([]map[string]interface{}, len(result))
	for i, e := range result {
		jsonActions[i] = SerializeActionEntryWithSequence(e)
	}

	// Build response summary
	summary := fmt.Sprintf("%d user action(s)", metadata.Count)
	if metadata.Total > metadata.Count {
		summary += fmt.Sprintf(" (total in buffer: %d)", metadata.Total)
	}

	// Build response with cursor metadata
	data := map[string]interface{}{
		"actions": jsonActions,
		"count":   metadata.Count,
		"total":   metadata.Total,
	}

	if metadata.Cursor != "" {
		data["cursor"] = metadata.Cursor
	}
	if metadata.OldestTimestamp != "" {
		data["oldest_timestamp"] = metadata.OldestTimestamp
	}
	if metadata.NewestTimestamp != "" {
		data["newest_timestamp"] = metadata.NewestTimestamp
	}
	if metadata.HasMore {
		data["has_more"] = metadata.HasMore
	}
	if metadata.CursorRestarted {
		data["cursor_restarted"] = metadata.CursorRestarted
		data["original_cursor"] = metadata.OriginalCursor
		data["warning"] = metadata.Warning
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, data)}
}
