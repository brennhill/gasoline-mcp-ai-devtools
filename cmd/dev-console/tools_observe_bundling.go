// tools_observe_bundling.go — Error bundling: assembles complete debugging context per error.
// Each bundle contains: error + recent network requests + recent actions + recent logs.
// One observe call replaces 3-4 separate calls. Pure Go-side join, no extension changes.
package main

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// timedEntry pairs a log entry with its parsed timestamp.
type timedEntry struct {
	data LogEntry
	ts   time.Time
}

// bundleContext holds all data sources needed for window-joining bundles.
type bundleContext struct {
	networkBodies    []capture.NetworkBody
	waterfallEntries []capture.NetworkWaterfallEntry
	actions          []capture.EnhancedAction
	logs             []timedEntry
	windowSeconds    int
}

func (h *ToolHandler) toolGetErrorBundles(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Limit         int    `json:"limit"`
		WindowSeconds int    `json:"window_seconds"`
		URL           string `json:"url"`
	}
	lenientUnmarshal(args, &params)
	if params.Limit <= 0 {
		params.Limit = 5
	}
	if params.WindowSeconds <= 0 {
		params.WindowSeconds = 3
	}
	if params.WindowSeconds > 10 {
		params.WindowSeconds = 10
	}

	errors, logs := h.collectErrorsAndLogs(params.Limit, params.URL)

	ctx := bundleContext{
		networkBodies:    h.capture.GetNetworkBodies(),
		waterfallEntries: h.capture.GetNetworkWaterfallEntries(),
		actions:          h.capture.GetAllEnhancedActions(),
		logs:             logs,
		windowSeconds:    params.WindowSeconds,
	}

	bundles := buildBundles(errors, ctx)

	var newestEntry time.Time
	if len(errors) > 0 {
		newestEntry = errors[0].ts
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Error bundles", map[string]any{
		"bundles":  bundles,
		"count":    len(bundles),
		"metadata": buildResponseMetadata(h.capture, newestEntry),
	})}
}

// collectErrorsAndLogs copies errors and logs from the server entries under a single read lock.
func (h *ToolHandler) collectErrorsAndLogs(limit int, urlFilter string) ([]timedEntry, []timedEntry) {
	h.server.mu.RLock()
	defer h.server.mu.RUnlock()

	var errors, logs []timedEntry
	for i := len(h.server.entries) - 1; i >= 0; i-- {
		entry := h.server.entries[i]
		ts := parseEntryTimestamp(entry)
		if ts.IsZero() {
			continue
		}
		entryType, _ := entry["type"].(string)
		if entryType == "lifecycle" || entryType == "tracking" || entryType == "extension" {
			continue
		}
		level, _ := entry["level"].(string)
		if level == "error" {
			if urlFilter != "" {
				entryURL, _ := entry["url"].(string)
				if !containsIgnoreCase(entryURL, urlFilter) {
					continue
				}
			}
			if len(errors) < limit {
				errors = append(errors, timedEntry{data: entry, ts: ts})
			}
		} else {
			logs = append(logs, timedEntry{data: entry, ts: ts})
		}
	}
	return errors, logs
}

// parseEntryTimestamp parses the timestamp from a log entry, trying RFC3339 then RFC3339Nano.
// Checks both "timestamp" (daemon-generated entries) and "ts" (extension-generated entries).
func parseEntryTimestamp(entry LogEntry) time.Time {
	tsStr, _ := entry["timestamp"].(string)
	if tsStr == "" {
		tsStr, _ = entry["ts"].(string)
	}
	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		ts, err = time.Parse(time.RFC3339Nano, tsStr)
		if err != nil {
			return time.Time{}
		}
	}
	return ts
}

// buildBundles creates a debugging bundle for each error by window-joining related data.
func buildBundles(errors []timedEntry, ctx bundleContext) []map[string]any {
	window := time.Duration(ctx.windowSeconds) * time.Second
	bundles := make([]map[string]any, 0, len(errors))

	for _, e := range errors {
		windowStart := e.ts.Add(-window)
		bundles = append(bundles, map[string]any{
			"error":                  errorEntryToMap(e.data),
			"network":               matchNetworkBodies(ctx.networkBodies, windowStart, e.ts),
			"waterfall":             matchWaterfall(ctx.waterfallEntries, windowStart, e.ts),
			"actions":               matchActions(ctx.actions, windowStart, e.ts),
			"logs":                  matchLogs(ctx.logs, windowStart, e.ts),
			"context_window_seconds": ctx.windowSeconds,
		})
	}
	return bundles
}

func errorEntryToMap(data LogEntry) map[string]any {
	return map[string]any{
		"message": data["message"], "source": data["source"],
		"url": data["url"], "line": data["line"],
		"column": data["column"], "stack": data["stack"],
		"timestamp": data["timestamp"],
	}
}

func matchNetworkBodies(bodies []capture.NetworkBody, start, end time.Time) []map[string]any {
	matched := make([]map[string]any, 0)
	for _, nb := range bodies {
		nbTs := parseTimestampString(nb.Timestamp)
		if nbTs.IsZero() || !nbTs.After(start) || nbTs.After(end) {
			continue
		}
		matched = append(matched, map[string]any{
			"method": nb.Method, "url": nb.URL, "status": nb.Status,
			"duration": nb.Duration, "content_type": nb.ContentType,
			"response_body": nb.ResponseBody, "timestamp": nb.Timestamp,
		})
	}
	return matched
}

func matchWaterfall(entries []capture.NetworkWaterfallEntry, start, end time.Time) []map[string]any {
	matched := make([]map[string]any, 0)
	for _, w := range entries {
		if w.Timestamp.IsZero() || !w.Timestamp.After(start) || w.Timestamp.After(end) {
			continue
		}
		matched = append(matched, map[string]any{
			"url": w.URL, "initiator_type": w.InitiatorType,
			"duration_ms": w.Duration, "transfer_size": w.TransferSize,
			"timestamp": w.Timestamp.Format(time.RFC3339),
		})
	}
	return matched
}

func matchActions(actions []capture.EnhancedAction, start, end time.Time) []map[string]any {
	matched := make([]map[string]any, 0)
	for _, a := range actions {
		aTs := time.UnixMilli(a.Timestamp)
		if !aTs.After(start) || aTs.After(end) {
			continue
		}
		actionMap := map[string]any{"type": a.Type, "timestamp": aTs.Format(time.RFC3339)}
		if a.URL != "" {
			actionMap["url"] = a.URL
		}
		if css, ok := a.Selectors["css"].(string); ok {
			actionMap["selector"] = css
		}
		if a.Value != "" {
			actionMap["value"] = a.Value
		}
		matched = append(matched, actionMap)
	}
	return matched
}

func matchLogs(logs []timedEntry, start, end time.Time) []map[string]any {
	matched := make([]map[string]any, 0)
	for _, l := range logs {
		if !l.ts.After(start) || l.ts.After(end) {
			continue
		}
		matched = append(matched, map[string]any{
			"level": l.data["level"], "message": l.data["message"],
			"timestamp": l.data["timestamp"],
		})
	}
	return matched
}

// parseTimestampString tries RFC3339 then RFC3339Nano, returning zero time on failure.
func parseTimestampString(s string) time.Time {
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		ts, _ = time.Parse(time.RFC3339Nano, s)
	}
	return ts
}
