// annotations.go — Annotation retrieval and formatting logic extracted from ToolHandler.
// Why: Moves annotation business logic out of the main package god object.
// Docs: docs/features/feature/annotated-screenshots/index.md

package toolanalyze

import (
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/annotation"
)

// AnnotationBlockingWaitDefault is the initial synchronous wait budget for annotations (background:false).
const AnnotationBlockingWaitDefault = 15 * time.Second

// AnnotationBlockingWaitMax caps caller-provided timeout_ms for wait=true annotation calls.
const AnnotationBlockingWaitMax = 10 * time.Minute

// AnnotationWaitCommandTTL is how long pending annotation commands remain active.
const AnnotationWaitCommandTTL = 10 * time.Minute

// AnnotationErrorCorrelationWindow is the time window around an annotation's timestamp
// in which console errors are considered correlated.
const AnnotationErrorCorrelationWindow = 5 * time.Second

// AnnotationBlockingWaitDuration calculates the wait duration from a timeout_ms parameter.
func AnnotationBlockingWaitDuration(timeoutMs int) time.Duration {
	if timeoutMs <= 0 {
		return AnnotationBlockingWaitDefault
	}
	waitDuration := time.Duration(timeoutMs) * time.Millisecond
	if waitDuration > AnnotationBlockingWaitMax {
		return AnnotationBlockingWaitMax
	}
	return waitDuration
}

// ResolveAnnotationURLFilter validates and resolves the url/url_pattern parameters.
// Returns (filter, errMsg, hasErr). If hasErr is true, errMsg describes the conflict.
func ResolveAnnotationURLFilter(urlValue, urlPatternValue string) (string, string, bool) {
	urlValue = strings.TrimSpace(urlValue)
	urlPatternValue = strings.TrimSpace(urlPatternValue)
	if urlValue != "" && urlPatternValue != "" && urlValue != urlPatternValue {
		return "", "Conflicting annotation scope filters: 'url' and 'url_pattern' differ", true
	}
	if urlPatternValue != "" {
		return urlPatternValue, "", false
	}
	return urlValue, "", false
}

// FilterAnnotationPages filters annotation pages by URL filter.
func FilterAnnotationPages(pages []*annotation.Session, urlFilter string) []*annotation.Session {
	if strings.TrimSpace(urlFilter) == "" {
		return pages
	}
	filtered := make([]*annotation.Session, 0, len(pages))
	for _, page := range pages {
		if annotation.URLMatches(urlFilter, page.PageURL) {
			filtered = append(filtered, page)
		}
	}
	return filtered
}

// BuildAnnotationSessionResult builds the response payload for a single annotation session.
func BuildAnnotationSessionResult(session *annotation.Session, urlFilter string) map[string]any {
	matched := annotation.URLMatches(urlFilter, session.PageURL)
	annotations := session.Annotations
	if !matched {
		annotations = []annotation.Annotation{}
	}

	result := map[string]any{
		"annotations":    annotations,
		"count":          len(annotations),
		"page_url":       session.PageURL,
		"filter_applied": annotation.FilterAppliedValue(urlFilter),
	}
	if session.ScreenshotPath != "" && matched {
		result["screenshot"] = session.ScreenshotPath
	}
	projects := BuildProjectSummaries([]*annotation.Session{session})
	if len(projects) > 0 {
		result["projects"] = projects
	}
	if !matched && urlFilter != "" {
		result["message"] = "No annotations match the requested url filter."
	}
	if len(annotations) > 0 {
		result["hints"] = BuildSessionHints(session.ScreenshotPath)
	}
	return result
}

// BuildNamedAnnotationSessionResult builds the response payload for a named annotation session.
func BuildNamedAnnotationSessionResult(ns *annotation.NamedSession, urlFilter string) map[string]any {
	allProjects := BuildProjectSummaries(ns.Pages)
	filteredPages := FilterAnnotationPages(ns.Pages, urlFilter)

	totalCount := 0
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

	// Find first screenshot for hints
	var screenshotPath string
	for _, page := range filteredPages {
		if page.ScreenshotPath != "" {
			screenshotPath = page.ScreenshotPath
			break
		}
	}

	result := map[string]any{
		"annot_session_name": ns.Name,
		"pages":              pages,
		"page_count":         len(filteredPages),
		"total_count":        totalCount,
		"filter_applied":     annotation.FilterAppliedValue(urlFilter),
	}
	if len(allProjects) > 0 {
		result["projects"] = allProjects
	}
	if len(allProjects) > 1 && urlFilter == "" {
		result["scope_ambiguous"] = true
		result["scope_warning"] = BuildScopeWarning(allProjects)
	}
	if len(filteredPages) == 0 && urlFilter != "" {
		result["message"] = "No pages in this annotation session match the requested url filter."
	}
	if totalCount > 0 {
		result["hints"] = BuildSessionHints(screenshotPath)
	}
	return result
}

// BuildAnnotationDetailResult builds the response payload for annotation detail.
func BuildAnnotationDetailResult(detail *annotation.Detail, correlatedErrors []map[string]string) map[string]any {
	result := map[string]any{
		"correlation_id":  detail.CorrelationID,
		"selector":        detail.Selector,
		"tag":             detail.Tag,
		"text_content":    detail.TextContent,
		"classes":         detail.Classes,
		"id":              detail.ID,
		"computed_styles": detail.ComputedStyles,
		"parent_selector": detail.ParentSelector,
		"bounding_rect":   detail.BoundingRect,
	}
	if len(detail.A11yFlags) > 0 {
		result["a11y_flags"] = detail.A11yFlags
	}
	if detail.OuterHTML != "" {
		result["outer_html"] = detail.OuterHTML
	}
	if len(detail.ShadowDOM) > 0 {
		result["shadow_dom"] = detail.ShadowDOM
	}
	if len(detail.AllElements) > 0 {
		result["all_elements"] = detail.AllElements
		result["element_count"] = detail.ElementCount
	}
	if len(detail.SelectorCandidates) > 0 {
		result["selector_candidates"] = detail.SelectorCandidates
	}
	if len(detail.IframeContent) > 0 {
		result["iframe_content"] = detail.IframeContent
	}
	if len(detail.ParentContext) > 0 {
		result["parent_context"] = detail.ParentContext
	}
	if len(detail.Siblings) > 0 {
		result["siblings"] = detail.Siblings
	}
	if detail.CSSFramework != "" {
		result["css_framework"] = detail.CSSFramework
	}
	if detail.JSFramework != "" {
		result["js_framework"] = detail.JSFramework
	}
	if len(detail.Component) > 0 {
		result["component"] = detail.Component
	}

	hasCorrelatedErrors := false
	if len(correlatedErrors) > 0 {
		result["correlated_errors"] = correlatedErrors
		result["error_correlation_window_seconds"] = 5
		hasCorrelatedErrors = true
	}

	if detailHints := BuildDetailHints(detail.CSSFramework, detail.JSFramework, detail.A11yFlags, hasCorrelatedErrors); detailHints != nil {
		result["hints"] = detailHints
	}

	return result
}

// BuildFlushedAnnotationResult builds the payload for a flushed annotation waiter.
func BuildFlushedAnnotationResult(store *annotation.Store, sessionName string, urlFilter string) map[string]any {
	if sessionName != "" {
		if ns := store.GetNamedSession(sessionName); ns != nil {
			data := BuildNamedAnnotationSessionResult(ns, urlFilter)
			data["status"] = "complete"
			data["terminal_reason"] = "flushed"
			return data
		}

		return map[string]any{
			"status":             "complete",
			"annot_session_name": sessionName,
			"pages":              []any{},
			"page_count":         0,
			"total_count":        0,
			"filter_applied":     annotation.FilterAppliedValue(urlFilter),
			"terminal_reason":    "abandoned",
			"message":            "Annotation waiter flushed with no named-session annotations available.",
		}
	}

	if session := store.GetLatestSession(); session != nil {
		data := BuildAnnotationSessionResult(session, urlFilter)
		data["status"] = "complete"
		data["terminal_reason"] = "flushed"
		return data
	}

	return map[string]any{
		"status":          "complete",
		"annotations":     []any{},
		"count":           0,
		"filter_applied":  annotation.FilterAppliedValue(urlFilter),
		"terminal_reason": "abandoned",
		"message":         "Annotation waiter flushed with no captured annotations available.",
	}
}
