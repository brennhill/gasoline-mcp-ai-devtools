// Purpose: Encodes annotation session payloads for async command completion responses.
// Why: Keeps result-shape logic centralized so waiter completion stays consistent across call sites.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import "encoding/json"

// BuildSessionResult serializes an annotation session for the CommandTracker.
func BuildSessionResult(session *Session) json.RawMessage {
	result := map[string]any{
		"status":      "complete",
		"annotations": session.Annotations,
		"count":       len(session.Annotations),
		"page_url":    session.PageURL,
	}
	if session.ScreenshotPath != "" {
		result["screenshot"] = session.ScreenshotPath
	}
	// Error impossible: map of primitive types.
	data, _ := json.Marshal(result)
	return data
}

// BuildNamedSessionResult serializes a named session for the CommandTracker.
func BuildNamedSessionResult(ns *NamedSession) json.RawMessage {
	totalCount := 0
	pages := make([]map[string]any, 0, len(ns.Pages))
	for _, page := range ns.Pages {
		totalCount += len(page.Annotations)
		p := map[string]any{
			"page_url":    page.PageURL,
			"annotations": page.Annotations,
			"count":       len(page.Annotations),
			"tab_id":      page.TabID,
		}
		if page.ScreenshotPath != "" {
			p["screenshot"] = page.ScreenshotPath
		}
		pages = append(pages, p)
	}
	result := map[string]any{
		"status":             "complete",
		"annot_session_name": ns.Name,
		"pages":              pages,
		"page_count":         len(ns.Pages),
		"total_count":        totalCount,
	}
	// Error impossible: map of primitive types.
	data, _ := json.Marshal(result)
	return data
}
