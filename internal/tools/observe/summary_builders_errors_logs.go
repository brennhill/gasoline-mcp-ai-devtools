package observe

import "sort"

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
