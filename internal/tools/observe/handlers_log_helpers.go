// Purpose: Shared helpers for observe browser log normalization.

package observe

func isInternalLogType(entryType string) bool {
	return entryType == "lifecycle" || entryType == "tracking" || entryType == "extension"
}

func logEntryTimestamp(entry map[string]any) string {
	if ts, ok := entry["ts"].(string); ok && ts != "" {
		return ts
	}
	if ts, ok := entry["timestamp"].(string); ok && ts != "" {
		return ts
	}
	return ""
}

func normalizeBrowserLogEntry(entry map[string]any) map[string]any {
	entryType, _ := entry["type"].(string)
	level, _ := entry["level"].(string)
	if level == "" && isInternalLogType(entryType) {
		level = "info"
	}

	message, _ := entry["message"].(string)
	if message == "" {
		if event, ok := entry["event"].(string); ok {
			message = event
		}
	}

	source, _ := entry["source"].(string)
	if source == "" && isInternalLogType(entryType) {
		source = "daemon"
	}

	normalized := map[string]any{
		"level":     level,
		"message":   message,
		"source":    source,
		"url":       entry["url"],
		"line":      entry["line"],
		"column":    entry["column"],
		"timestamp": logEntryTimestamp(entry),
		"tab_id":    entry["tabId"],
	}

	if entryType != "" {
		normalized["type"] = entryType
	}
	if event, ok := entry["event"]; ok {
		normalized["event"] = event
	}
	if pid, ok := entry["pid"]; ok {
		normalized["pid"] = pid
	}
	if port, ok := entry["port"]; ok {
		normalized["port"] = port
	}

	extras := make(map[string]any)
	for k, v := range entry {
		switch k {
		case "type", "level", "message", "source", "url", "line", "column", "ts", "timestamp", "tabId", "event", "pid", "port":
			// handled above
		default:
			extras[k] = v
		}
	}
	if len(extras) > 0 {
		normalized["data"] = extras
	}

	return normalized
}
