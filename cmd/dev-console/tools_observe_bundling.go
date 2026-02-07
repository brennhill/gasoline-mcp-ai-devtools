// tools_observe_bundling.go — Error bundling: assembles complete debugging context per error.
// Each bundle contains: error + recent network requests + recent actions + recent logs.
// One observe call replaces 3-4 separate calls. Pure Go-side join, no extension changes.
package main

import (
	"encoding/json"
	"time"
)

func (h *ToolHandler) toolGetErrorBundles(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit         int `json:"limit"`
		WindowSeconds int `json:"window_seconds"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &params)
	}
	if params.Limit <= 0 {
		params.Limit = 5
	}
	if params.WindowSeconds <= 0 {
		params.WindowSeconds = 3
	}
	if params.WindowSeconds > 10 {
		params.WindowSeconds = 10
	}
	window := time.Duration(params.WindowSeconds) * time.Second

	// Phase 1: Copy errors and logs from Server.entries under one RLock
	h.server.mu.RLock()
	type errorEntry struct {
		data LogEntry
		ts   time.Time
	}
	type logEntry struct {
		data LogEntry
		ts   time.Time
	}
	var errors []errorEntry
	var logs []logEntry

	for i := len(h.server.entries) - 1; i >= 0; i-- {
		entry := h.server.entries[i]
		level, _ := entry["level"].(string)
		tsStr, _ := entry["timestamp"].(string)
		ts, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			// Also try RFC3339Nano
			ts, err = time.Parse(time.RFC3339Nano, tsStr)
			if err != nil {
				continue // skip entries without parseable timestamps
			}
		}

		// Skip lifecycle/tracking/extension entries
		entryType, _ := entry["type"].(string)
		if entryType == "lifecycle" || entryType == "tracking" || entryType == "extension" {
			continue
		}

		if level == "error" {
			if len(errors) < params.Limit {
				errors = append(errors, errorEntry{data: entry, ts: ts})
			}
		} else {
			logs = append(logs, logEntry{data: entry, ts: ts})
		}
	}
	h.server.mu.RUnlock()

	// Phase 2: Copy network bodies and actions from Capture (via existing getters)
	networkBodies := h.capture.GetNetworkBodies()
	actions := h.capture.GetAllEnhancedActions()

	// Phase 3: For each error, window-join by timestamp
	bundles := make([]map[string]any, 0, len(errors))

	for _, e := range errors {
		windowStart := e.ts.Add(-window)

		// Build error object
		errObj := map[string]any{
			"message":   e.data["message"],
			"source":    e.data["source"],
			"url":       e.data["url"],
			"line":      e.data["line"],
			"column":    e.data["column"],
			"stack":     e.data["stack"],
			"timestamp": e.data["timestamp"],
		}

		// Find matching network bodies within window
		matchedNetwork := make([]map[string]any, 0)
		for _, nb := range networkBodies {
			nbTs, err := time.Parse(time.RFC3339, nb.Timestamp)
			if err != nil {
				nbTs, err = time.Parse(time.RFC3339Nano, nb.Timestamp)
				if err != nil {
					continue
				}
			}
			if nbTs.After(windowStart) && !nbTs.After(e.ts) {
				matchedNetwork = append(matchedNetwork, map[string]any{
					"method":        nb.Method,
					"url":           nb.URL,
					"status":        nb.Status,
					"duration":      nb.Duration,
					"content_type":  nb.ContentType,
					"response_body": nb.ResponseBody,
					"timestamp":     nb.Timestamp,
				})
			}
		}

		// Find matching actions within window
		matchedActions := make([]map[string]any, 0)
		for _, a := range actions {
			aTs := time.UnixMilli(a.Timestamp)
			if aTs.After(windowStart) && !aTs.After(e.ts) {
				actionMap := map[string]any{
					"type":      a.Type,
					"timestamp": aTs.Format(time.RFC3339),
				}
				if a.URL != "" {
					actionMap["url"] = a.URL
				}
				if css, ok := a.Selectors["css"].(string); ok {
					actionMap["selector"] = css
				}
				if a.Value != "" {
					actionMap["value"] = a.Value
				}
				matchedActions = append(matchedActions, actionMap)
			}
		}

		// Find matching logs within window (non-error entries only)
		matchedLogs := make([]map[string]any, 0)
		for _, l := range logs {
			if l.ts.After(windowStart) && !l.ts.After(e.ts) {
				matchedLogs = append(matchedLogs, map[string]any{
					"level":     l.data["level"],
					"message":   l.data["message"],
					"timestamp": l.data["timestamp"],
				})
			}
		}

		bundle := map[string]any{
			"error":                  errObj,
			"network":               matchedNetwork,
			"actions":               matchedActions,
			"logs":                  matchedLogs,
			"context_window_seconds": params.WindowSeconds,
		}
		bundles = append(bundles, bundle)
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Error bundles", map[string]any{
		"bundles": bundles,
		"count":   len(bundles),
	})}
}
