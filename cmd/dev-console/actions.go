package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// ============================================
// Enhanced Actions (v5)
// ============================================

// AddEnhancedActions adds enhanced actions to the buffer
func (v *Capture) AddEnhancedActions(actions []EnhancedAction) {
	v.mu.Lock()
	defer v.mu.Unlock()

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

	// Enforce max count
	if len(v.enhancedActions) > maxEnhancedActions {
		v.enhancedActions = v.enhancedActions[len(v.enhancedActions)-maxEnhancedActions:]
		v.actionAddedAt = v.actionAddedAt[len(v.actionAddedAt)-maxEnhancedActions:]
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
	// Check rate limit and circuit breaker
	if v.CheckRateLimit() {
		v.WriteRateLimitResponse(w)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload struct {
		Actions []EnhancedAction `json:"actions"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Record batch size for rate limiting
	v.RecordEvents(len(payload.Actions))

	// Re-check after recording
	if v.CheckRateLimit() {
		v.WriteRateLimitResponse(w)
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

	result := map[string]interface{}{
		"content": []map[string]string{
			{"type": "text", "text": contentText},
		},
	}
	resultJSON, _ := json.Marshal(result)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: resultJSON}
}
