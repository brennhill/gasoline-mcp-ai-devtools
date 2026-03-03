// Purpose: Encodes annotation session payloads for async command completion responses.
// Why: Keeps result-shape logic centralized so waiter completion stays consistent across call sites.
// Docs: docs/features/feature/annotated-screenshots/index.md

package annotation

import (
	"encoding/json"
	"net/url"
	"strings"
)

// BuildSessionResult serializes an annotation session for the CommandTracker.
func BuildSessionResult(session *Session, urlFilter string) json.RawMessage {
	annotations := session.Annotations
	if !annotationURLMatches(urlFilter, session.PageURL) {
		annotations = []Annotation{}
	}
	result := map[string]any{
		"status":          "complete",
		"annotations":     annotations,
		"count":           len(annotations),
		"page_url":        session.PageURL,
		"terminal_reason": "completed",
		"filter_applied":  annotationFilterAppliedValue(urlFilter),
	}
	if session.ScreenshotPath != "" && len(annotations) > 0 {
		result["screenshot"] = session.ScreenshotPath
	}
	// Error impossible: map of primitive types.
	data, _ := json.Marshal(result)
	return data
}

// BuildNamedSessionResult serializes a named session for the CommandTracker.
func BuildNamedSessionResult(ns *NamedSession, urlFilter string) json.RawMessage {
	totalCount := 0
	filteredPages := filterPagesByURL(ns.Pages, urlFilter)
	pages := make([]map[string]any, 0, len(filteredPages))
	for _, page := range filteredPages {
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
		"page_count":         len(filteredPages),
		"total_count":        totalCount,
		"terminal_reason":    "completed",
		"filter_applied":     annotationFilterAppliedValue(urlFilter),
	}
	// Error impossible: map of primitive types.
	data, _ := json.Marshal(result)
	return data
}

func annotationFilterAppliedValue(urlFilter string) string {
	if strings.TrimSpace(urlFilter) == "" {
		return "none"
	}
	return urlFilter
}

func filterPagesByURL(pages []*Session, urlFilter string) []*Session {
	if strings.TrimSpace(urlFilter) == "" {
		return pages
	}
	filtered := make([]*Session, 0, len(pages))
	for _, page := range pages {
		if annotationURLMatches(urlFilter, page.PageURL) {
			filtered = append(filtered, page)
		}
	}
	return filtered
}

func annotationURLMatches(urlFilter, pageURL string) bool {
	urlFilter = strings.TrimSpace(urlFilter)
	if urlFilter == "" {
		return true
	}
	pageURL = strings.TrimSpace(pageURL)
	if pageURL == "" {
		return false
	}

	if strings.Contains(urlFilter, "*") {
		prefix := strings.ReplaceAll(urlFilter, "*", "")
		return strings.HasPrefix(pageURL, prefix)
	}

	if strings.HasSuffix(urlFilter, "/") {
		prefix := strings.TrimSuffix(urlFilter, "/")
		return strings.HasPrefix(pageURL, prefix)
	}

	parsed, err := url.Parse(urlFilter)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" && (parsed.Path == "" || parsed.Path == "/") {
		base := strings.TrimSuffix(urlFilter, "/")
		return strings.HasPrefix(pageURL, base)
	}

	return pageURL == urlFilter
}
