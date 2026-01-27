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
		LastN int    `json:"last_n"`
		URL   string `json:"url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	actions := h.capture.GetEnhancedActions(EnhancedActionFilter{
		LastN:     arguments.LastN,
		URLFilter: arguments.URL,
	})

	if len(actions) == 0 {
		msg := "No user actions captured"
		if h.captureOverrides != nil {
			overrides := h.captureOverrides.GetAll()
			if overrides["action_replay"] == "false" {
				msg += "\n\nAction replay capture is OFF. To enable, call:\nconfigure({action: \"capture\", settings: {action_replay: \"true\"}})"
			}
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(msg)}
	}

	summary := fmt.Sprintf("%d user action(s)", len(actions))
	rows := make([][]string, len(actions))
	for i, a := range actions {
		// Pick the best selector string
		sel := ""
		if a.Selectors != nil {
			for _, key := range []string{"testId", "ariaLabel", "role", "id", "cssPath"} {
				if v, ok := a.Selectors[key]; ok {
					if s, ok := v.(string); ok && s != "" {
						sel = s
						break
					}
				}
			}
		}
		ts := time.Unix(0, a.Timestamp*int64(time.Millisecond)).Format("15:04:05")
		rows[i] = []string{
			a.Type,
			truncate(a.URL, 60),
			truncate(sel, 40),
			truncate(a.Value, 30),
			ts,
		}
	}
	table := markdownTable([]string{"Type", "URL", "Selector", "Value", "Time"}, rows)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpMarkdownResponse(summary, table)}
}
