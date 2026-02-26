// tools_summary_pref.go — Session-level summary preference injection.
package main

import "encoding/json"

// loadSummaryPref reads the cached summary preference.
// On first call (or after invalidation), loads from session store.
func (h *ToolHandler) loadSummaryPref() bool {
	h.summaryPrefMu.RLock()
	if h.summaryPrefReady {
		val := h.summaryPrefValue
		h.summaryPrefMu.RUnlock()
		return val
	}
	h.summaryPrefMu.RUnlock()

	// Upgrade to write lock and load from store
	h.summaryPrefMu.Lock()
	defer h.summaryPrefMu.Unlock()

	// Double-check after acquiring write lock
	if h.summaryPrefReady {
		return h.summaryPrefValue
	}

	h.summaryPrefReady = true
	h.summaryPrefValue = false

	if h.sessionStoreImpl == nil {
		return false
	}

	data, err := h.sessionStoreImpl.Load("session", "response_mode")
	if err != nil || len(data) == 0 {
		return false
	}

	var pref struct {
		Summary bool `json:"summary"`
	}
	if err := json.Unmarshal(data, &pref); err != nil {
		return false
	}
	h.summaryPrefValue = pref.Summary
	return h.summaryPrefValue
}

// invalidateSummaryPref clears the cached preference, forcing a re-read.
func (h *ToolHandler) invalidateSummaryPref() {
	h.summaryPrefMu.Lock()
	defer h.summaryPrefMu.Unlock()
	h.summaryPrefReady = false
	h.summaryPrefValue = false
}

// maybeInjectSummary adds "summary":true to args when:
// 1. Session preference is set
// 2. Args don't already contain "summary" or "full" keys
// Parses args at most once to avoid redundant JSON overhead.
func (h *ToolHandler) maybeInjectSummary(args json.RawMessage) json.RawMessage {
	if !h.loadSummaryPref() {
		return args
	}

	if len(args) == 0 || string(args) == "null" {
		return json.RawMessage(`{"summary":true}`)
	}

	// Single parse: check for existing keys and inject in one pass
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return args
	}
	if _, ok := m["summary"]; ok {
		return args
	}
	if _, ok := m["full"]; ok {
		return args
	}

	m["summary"] = json.RawMessage(`true`)
	// Error impossible: simple map of JSON values
	result, _ := json.Marshal(m)
	return result
}

// argHasKey checks if a JSON object contains a specific key.
func argHasKey(args json.RawMessage, key string) bool {
	if len(args) == 0 || string(args) == "null" {
		return false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(args, &m); err != nil {
		return false
	}
	_, ok := m[key]
	return ok
}
