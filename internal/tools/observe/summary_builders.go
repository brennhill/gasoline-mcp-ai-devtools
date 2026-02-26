// summary_builders.go — Compact summary builders for observe modes.
package observe

import (
	"sort"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/pagination"
)

// buildErrorsSummary returns {total, by_source, top_messages, metadata}.
func buildErrorsSummary(errors []map[string]any, noiseSuppressed int, meta ResponseMetadata) map[string]any {
	bySource := make(map[string]int)
	msgCounts := make(map[string]int)

	for _, e := range errors {
		src, _ := e["source"].(string)
		if src == "" {
			src = "unknown"
		}
		bySource[src]++

		msg, _ := e["message"].(string)
		if msg != "" {
			msg = truncateRunes(msg, 100)
			msgCounts[msg]++
		}
	}

	// Build top messages sorted by frequency
	type msgCount struct {
		msg   string
		count int
	}
	ranked := make([]msgCount, 0, len(msgCounts))
	for msg, count := range msgCounts {
		ranked = append(ranked, msgCount{msg, count})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].count > ranked[j].count
	})
	topN := 5
	if len(ranked) < topN {
		topN = len(ranked)
	}
	topMessages := make([]map[string]any, topN)
	for i := 0; i < topN; i++ {
		topMessages[i] = map[string]any{"message": ranked[i].msg, "count": ranked[i].count}
	}

	result := map[string]any{
		"total":        len(errors),
		"by_source":    bySource,
		"top_messages": topMessages,
		"metadata":     meta,
	}
	if noiseSuppressed > 0 {
		result["noise_suppressed"] = noiseSuppressed
	}
	return result
}

// buildLogsSummary returns {total, by_level, by_source, metadata}.
func buildLogsSummary(logs []map[string]any, meta map[string]any) map[string]any {
	byLevel := make(map[string]int)
	bySource := make(map[string]int)

	for _, l := range logs {
		level, _ := l["level"].(string)
		if level == "" {
			level = "unknown"
		}
		byLevel[level]++

		src, _ := l["source"].(string)
		if src == "" {
			src = "unknown"
		}
		bySource[src]++
	}

	return map[string]any{
		"total":     len(logs),
		"by_level":  byLevel,
		"by_source": bySource,
		"metadata":  meta,
	}
}

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

// buildErrorBundlesSummary returns {total_bundles, unique_error_messages, newest_entry, metadata}.
func buildErrorBundlesSummary(bundles []map[string]any, newestEntry time.Time, meta ResponseMetadata) map[string]any {
	seen := make(map[string]bool)
	messages := make([]string, 0)

	for _, b := range bundles {
		errMap, ok := b["error"].(map[string]any)
		if !ok {
			continue
		}
		msg, _ := errMap["message"].(string)
		if msg != "" && !seen[msg] {
			seen[msg] = true
			messages = append(messages, msg)
		}
	}

	result := map[string]any{
		"total_bundles":         len(bundles),
		"unique_error_messages": messages,
		"metadata":              meta,
	}
	if !newestEntry.IsZero() {
		result["newest_entry"] = newestEntry.Format(time.RFC3339)
	}
	return result
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

// quickLogsSummary is a lightweight version for pagination header (just by_level + total).
func quickLogsSummary(logs []map[string]any) map[string]any {
	byLevel := make(map[string]int)
	for _, l := range logs {
		level, _ := l["level"].(string)
		if level == "" {
			level = "unknown"
		}
		byLevel[level]++
	}
	return map[string]any{
		"total":    len(logs),
		"by_level": byLevel,
	}
}

// truncateRunes truncates a string to maxRunes runes, avoiding mid-character splits.
func truncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// BuildPaginatedMetadataWithSummary adds a summary block to first-page paginated responses.
func BuildPaginatedMetadataWithSummary(
	cap *capture.Capture, newestEntry time.Time,
	pMeta *pagination.CursorPaginationMetadata,
	isFirstPage bool,
	summaryFn func() map[string]any,
) map[string]any {
	var meta map[string]any
	if cap != nil {
		meta = BuildPaginatedResponseMetadata(cap, newestEntry, pMeta)
	} else {
		// Nil capture — build minimal metadata for testing
		meta = map[string]any{
			"total":   pMeta.Total,
			"has_more": pMeta.HasMore,
		}
		if pMeta.Cursor != "" {
			meta["cursor"] = pMeta.Cursor
		}
	}
	if isFirstPage && summaryFn != nil {
		meta["summary"] = summaryFn()
	}
	return meta
}
