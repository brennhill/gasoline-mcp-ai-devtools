package observe

import "time"

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
