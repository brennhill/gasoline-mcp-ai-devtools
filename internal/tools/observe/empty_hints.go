// empty_hints.go — Diagnostic hint builders for empty observe results.
// Purpose: Generates contextual hints when observe modes return 0 entries.
// Why: When an observe mode returns 0 entries, the hint field explains why
// the buffer may be empty and suggests remediation. Fixes #278 and #287.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"fmt"
	"strings"
)

// NetworkBodiesHintFilters captures active network_bodies filters for hint generation.
type NetworkBodiesHintFilters struct {
	URL       string
	Method    string
	StatusMin int
	StatusMax int
	BodyKey   string
	BodyPath  string
}

// networkBodiesEmptyHint returns a diagnostic hint when GetNetworkBodies returns
// 0 filtered entries. It cross-references the waterfall count to explain
// whether the issue is prospective-only capture, a URL filter mismatch, or no data at all.
// waterfallCount is the number of entries in the network waterfall buffer.
func networkBodiesEmptyHint(waterfallCount int, unfilteredCount int, filters NetworkBodiesHintFilters) string {
	activeFilterSummary := formatNetworkBodiesFilterSummary(filters)

	// Case 1: Filter reduced non-empty results to zero
	if unfilteredCount > 0 && activeFilterSummary != "" {
		return fmt.Sprintf(
			"No bodies matched filters (%s), but %d bodies exist in the buffer. "+
				"Try broadening the filters or calling observe({what: \"network_bodies\"}) without filter params.",
			activeFilterSummary, unfilteredCount,
		)
	}

	// Case 2: No bodies at all — check waterfall for context
	if waterfallCount > 0 {
		return fmt.Sprintf(
			"No network bodies captured, but the network waterfall shows %d requests. "+
				"Bodies are only captured for requests made after the extension started tracking. "+
				"Requests that completed before tracking began appear in the waterfall (via PerformanceResourceTiming) "+
				"but their bodies were not intercepted. Navigate to a new page or trigger new requests to capture bodies.",
			waterfallCount,
		)
	}

	// Case 3: No waterfall, no bodies — nothing captured yet
	return "No network bodies captured. Ensure the Gasoline extension is connected and tracking a tab. " +
		"Bodies are captured for requests made after tracking starts. Check observe({what: \"pilot\"}) for extension status."
}

func formatNetworkBodiesFilterSummary(filters NetworkBodiesHintFilters) string {
	parts := make([]string, 0, 5)
	if filters.URL != "" {
		parts = append(parts, fmt.Sprintf("url~%q", filters.URL))
	}
	if filters.Method != "" {
		parts = append(parts, fmt.Sprintf("method=%s", strings.ToUpper(filters.Method)))
	}
	if filters.StatusMin > 0 && filters.StatusMax > 0 {
		parts = append(parts, fmt.Sprintf("status=%d..%d", filters.StatusMin, filters.StatusMax))
	} else if filters.StatusMin > 0 {
		parts = append(parts, fmt.Sprintf("status>=%d", filters.StatusMin))
	} else if filters.StatusMax > 0 {
		parts = append(parts, fmt.Sprintf("status<=%d", filters.StatusMax))
	}
	if filters.BodyKey != "" {
		parts = append(parts, fmt.Sprintf("body_key=%s", filters.BodyKey))
	}
	if filters.BodyPath != "" {
		parts = append(parts, fmt.Sprintf("body_path=%s", filters.BodyPath))
	}
	return strings.Join(parts, ", ")
}

// wsEventsEmptyHint returns a diagnostic hint when GetWSEvents returns 0 filtered entries.
func wsEventsEmptyHint(unfilteredCount int, urlFilter string) string {
	// Case 1: Filter reduced non-empty results to zero
	if unfilteredCount > 0 && urlFilter != "" {
		return fmt.Sprintf(
			"No WebSocket events matched the url filter %q, but %d events exist unfiltered. "+
				"Try broadening the filter or calling observe({what: \"websocket_events\"}) without a url param.",
			urlFilter, unfilteredCount,
		)
	}

	// Case 2: No events at all
	return "No WebSocket events captured. WebSocket interception must be active before connections open. " +
		"If the page was loaded before tracking started, existing WebSocket connections are not captured. " +
		"Navigate to the page again (or refresh) while tracking is active to capture WebSocket traffic."
}

// wsStatusEmptyHint returns a diagnostic hint when GetWSStatus returns 0 connections.
func wsStatusEmptyHint() string {
	return "No WebSocket connections found. WebSocket interception must be active before connections open. " +
		"If the page was loaded before tracking started, existing connections are not visible. " +
		"Navigate to the page again (or refresh) while tracking is active to detect WebSocket connections."
}
