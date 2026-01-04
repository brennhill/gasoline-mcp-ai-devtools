// actions.go — User action recording buffer and MCP tool handler.
// Captures browser interactions (click, input, navigate, scroll, select)
// with smart selectors (testId, role, ariaLabel, text, id, cssPath).
// Design: Fixed-size ring buffer. Sensitive input values are redacted
// at capture time. Selectors prioritized for Playwright compatibility.
package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ============================================
// Enhanced Actions
// ============================================

// AddEnhancedActions adds enhanced actions to the buffer
func (v *Capture) AddEnhancedActions(actions []EnhancedAction) {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Enforce memory limits before adding
	v.enforceMemory()

	v.actionTotalAdded += int64(len(actions))
	now := time.Now()
	for i := range actions {
		// Redact password values on ingest
		if actions[i].InputType == "password" && actions[i].Value != "[redacted]" {
			actions[i].Value = "[redacted]"
		}
		v.enhancedActions = append(v.enhancedActions, actions[i])
		v.actionAddedAt = append(v.actionAddedAt, now)
	}

	// Enforce max count (respecting minimal mode)
	capacity := v.effectiveActionCapacity()
	if len(v.enhancedActions) > capacity {
		v.enhancedActions = v.enhancedActions[len(v.enhancedActions)-capacity:]
		v.actionAddedAt = v.actionAddedAt[len(v.actionAddedAt)-capacity:]
	}
}

// GetEnhancedActionCount returns the current number of buffered actions
func (v *Capture) GetEnhancedActionCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.enhancedActions)
}

// GetEnhancedActions returns filtered enhanced actions
func (v *Capture) GetEnhancedActions(filter EnhancedActionFilter) []EnhancedAction {
	v.mu.RLock()
	defer v.mu.RUnlock()

	var filtered []EnhancedAction
	for i := range v.enhancedActions {
		if filter.URLFilter != "" && !strings.Contains(v.enhancedActions[i].URL, filter.URLFilter) {
			continue
		}
		filtered = append(filtered, v.enhancedActions[i])
	}

	// Apply lastN (return most recent N)
	if filter.LastN > 0 && len(filtered) > filter.LastN {
		filtered = filtered[len(filtered)-filter.LastN:]
	}

	return filtered
}

func (v *Capture) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
	body, ok := v.readIngestBody(w, r)
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
	if !v.recordAndRecheck(w, len(payload.Actions)) {
		return
	}
	v.AddEnhancedActions(payload.Actions)
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

	var contentText string
	if len(actions) == 0 {
		contentText = "No enhanced actions captured"
	} else {
		actionsJSON, _ := json.Marshal(actions)
		contentText = string(actionsJSON)
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpTextResponse(contentText)}
}
