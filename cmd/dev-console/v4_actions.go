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
func (v *V4Server) AddEnhancedActions(actions []EnhancedAction) {
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
func (v *V4Server) GetEnhancedActionCount() int {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return len(v.enhancedActions)
}

// GetEnhancedActions returns filtered enhanced actions
func (v *V4Server) GetEnhancedActions(filter EnhancedActionFilter) []EnhancedAction {
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

func (v *V4Server) HandleEnhancedActions(w http.ResponseWriter, r *http.Request) {
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

	v.AddEnhancedActions(payload.Actions)
	w.WriteHeader(http.StatusOK)
}

func (h *MCPHandlerV4) toolGetEnhancedActions(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		LastN int    `json:"last_n"`
		URL   string `json:"url"`
	}
	_ = json.Unmarshal(args, &arguments) // Optional args - zero values are acceptable defaults

	actions := h.v4.GetEnhancedActions(EnhancedActionFilter{
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
