// Purpose: Builds compact summary objects for network bodies, waterfall, and WebSocket data.
// Why: Separates network/event summary construction from error, log, and bundle summaries.
package observe

import (
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
)

// buildNetworkBodiesSummary returns {total, by_status_group, by_method, top_urls, metadata}.
func buildNetworkBodiesSummary(bodies []capture.NetworkBody, meta ResponseMetadata) map[string]any {
	byStatus := make(map[string]int)
	byMethod := make(map[string]int)
	seenURLs := make(map[string]bool)
	urls := make([]string, 0)

	for _, b := range bodies {
		// Status grouping: 2xx, 3xx, 4xx, 5xx
		group := statusGroup(b.Status)
		byStatus[group]++

		method := b.Method
		if method == "" {
			method = "GET"
		}
		byMethod[method]++

		url := b.URL
		if len([]rune(url)) > 80 {
			url = string([]rune(url)[:80]) + "..."
		}
		if !seenURLs[url] {
			seenURLs[url] = true
			urls = append(urls, url)
		}
	}

	topN := 5
	if len(urls) < topN {
		topN = len(urls)
	}

	return map[string]any{
		"total":           len(bodies),
		"by_status_group": byStatus,
		"by_method":       byMethod,
		"recent_urls":     urls[:topN],
		"metadata":        meta,
	}
}

// statusGroup converts an HTTP status code to a group string (2xx, 3xx, etc.).
func statusGroup(status int) string {
	switch {
	case status >= 200 && status < 300:
		return "2xx"
	case status >= 300 && status < 400:
		return "3xx"
	case status >= 400 && status < 500:
		return "4xx"
	case status >= 500 && status < 600:
		return "5xx"
	default:
		return "other"
	}
}

// buildWSEventsSummary returns {total, by_direction, by_event_type, connection_count, metadata}.
func buildWSEventsSummary(events []capture.WebSocketEvent, meta ResponseMetadata) map[string]any {
	byDirection := make(map[string]int)
	byEvent := make(map[string]int)
	connIDs := make(map[string]bool)

	for _, e := range events {
		if e.Direction != "" {
			byDirection[e.Direction]++
		}
		if e.Event != "" {
			byEvent[e.Event]++
		}
		if e.ID != "" {
			connIDs[e.ID] = true
		}
	}

	return map[string]any{
		"total":            len(events),
		"by_direction":     byDirection,
		"by_event_type":    byEvent,
		"connection_count": len(connIDs),
		"metadata":         meta,
	}
}

// buildActionsSummary returns {total, by_type, time_range, metadata}.
func buildActionsSummary(actions []capture.EnhancedAction, meta ResponseMetadata) map[string]any {
	byType := make(map[string]int)
	var firstTS, lastTS int64
	hasTS := false

	for _, a := range actions {
		byType[a.Type]++
		if !hasTS || a.Timestamp < firstTS {
			firstTS = a.Timestamp
			hasTS = true
		}
		if a.Timestamp > lastTS {
			lastTS = a.Timestamp
		}
	}

	result := map[string]any{
		"total":    len(actions),
		"by_type":  byType,
		"metadata": meta,
	}
	if hasTS {
		result["time_range"] = map[string]string{
			"first": time.UnixMilli(firstTS).Format(time.RFC3339),
			"last":  time.UnixMilli(lastTS).Format(time.RFC3339),
		}
	}
	return result
}

// buildHistorySummary returns {total, by_type, unique_urls, metadata}.
func buildHistorySummary(entries []historyEntry, meta ResponseMetadata) map[string]any {
	byType := make(map[string]int)
	urls := make(map[string]bool)

	for _, e := range entries {
		if e.Type != "" {
			byType[e.Type]++
		}
		if e.ToURL != "" {
			urls[e.ToURL] = true
		}
	}

	return map[string]any{
		"total":       len(entries),
		"by_type":     byType,
		"unique_urls": len(urls),
		"metadata":    meta,
	}
}
