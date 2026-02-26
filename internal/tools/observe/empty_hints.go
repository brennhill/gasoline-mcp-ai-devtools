// empty_hints.go — Diagnostic hint builders for empty observe results.
// Why: When an observe mode returns 0 entries, the hint field explains why
// the buffer may be empty and suggests remediation. Fixes #278 and #287.

package observe

import "fmt"

// networkBodiesEmptyHint returns a diagnostic hint when GetNetworkBodies returns
// 0 filtered entries. It cross-references the waterfall count to explain
// whether the issue is prospective-only capture, a URL filter mismatch, or no data at all.
func networkBodiesEmptyHint(deps Deps, unfilteredCount int, urlFilter string) string {
	// Case 1: Filter reduced non-empty results to zero
	if unfilteredCount > 0 && urlFilter != "" {
		return fmt.Sprintf(
			"No bodies matched the url filter %q, but %d bodies exist unfiltered. "+
				"Try broadening the filter or calling observe({what: \"network_bodies\"}) without a url param.",
			urlFilter, unfilteredCount,
		)
	}

	// Case 2: No bodies at all — check waterfall for context
	waterfallCount := len(deps.GetCapture().GetNetworkWaterfallEntries())
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
