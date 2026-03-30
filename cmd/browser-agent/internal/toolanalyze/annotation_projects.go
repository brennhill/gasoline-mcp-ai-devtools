// annotation_projects.go — Builds project-level summaries and scope warnings from annotation sessions.
// Why: Isolates project detection logic from annotation handler flow control.
// Docs: docs/features/feature/annotated-screenshots/index.md

package toolanalyze

import (
	"net/url"
	"sort"
	"strings"
)

// AnnotationPage is a minimal interface for annotation session pages.
type AnnotationPage interface {
	GetPageURL() string
	GetAnnotationCount() int
}

// BuildProjectSummariesFromURLs builds project-level summaries from page URL + annotation count pairs.
func BuildProjectSummariesFromURLs(pageURLs []string, annotCounts []int) []map[string]any {
	type projectAggregate struct {
		pageSet         map[string]struct{}
		annotationCount int
	}
	projects := make(map[string]*projectAggregate)
	for i, pageURL := range pageURLs {
		baseURL := AnnotationProjectBaseURL(pageURL)
		if strings.TrimSpace(baseURL) == "" {
			continue
		}
		agg, ok := projects[baseURL]
		if !ok {
			agg = &projectAggregate{
				pageSet: make(map[string]struct{}),
			}
			projects[baseURL] = agg
		}
		if i < len(annotCounts) {
			agg.annotationCount += annotCounts[i]
		}
		if strings.TrimSpace(pageURL) != "" {
			agg.pageSet[pageURL] = struct{}{}
		}
	}

	if len(projects) == 0 {
		return nil
	}
	baseURLs := make([]string, 0, len(projects))
	for baseURL := range projects {
		baseURLs = append(baseURLs, baseURL)
	}
	sort.Strings(baseURLs)

	summaries := make([]map[string]any, 0, len(baseURLs))
	for _, baseURL := range baseURLs {
		agg := projects[baseURL]
		pageURLsList := make([]string, 0, len(agg.pageSet))
		for pageURL := range agg.pageSet {
			pageURLsList = append(pageURLsList, pageURL)
		}
		sort.Strings(pageURLsList)
		summary := map[string]any{
			"base_url":           baseURL,
			"annotation_count":   agg.annotationCount,
			"page_count":         len(pageURLsList),
			"recommended_filter": baseURL + "/*",
		}
		if len(pageURLsList) > 0 {
			summary["page_urls"] = pageURLsList
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

// BuildScopeWarning builds a warning message for multi-project annotation sessions.
func BuildScopeWarning(projects []map[string]any) map[string]any {
	suggestedFilters := make([]string, 0, len(projects))
	projectBaseURLs := make([]string, 0, len(projects))
	for _, project := range projects {
		if filter, ok := project["recommended_filter"].(string); ok && filter != "" {
			suggestedFilters = append(suggestedFilters, filter)
		}
		if baseURL, ok := project["base_url"].(string); ok && baseURL != "" {
			projectBaseURLs = append(projectBaseURLs, baseURL)
		}
	}
	return map[string]any{
		"warning":           "MULTI-PROJECT ANNOTATION SESSION DETECTED: annotations span multiple projects.",
		"recommendation":    "Re-run analyze({what:'annotations'}) with 'url' or 'url_pattern' scoped to the active project before implementing changes.",
		"suggested_filters": suggestedFilters,
		"projects_detected": projectBaseURLs,
	}
}

// AnnotationProjectBaseURL extracts the scheme+host from a URL for project grouping.
func AnnotationProjectBaseURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return rawURL
	}
	return parsed.Scheme + "://" + parsed.Host
}
