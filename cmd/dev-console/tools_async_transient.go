// tools_async_transient.go — Attaches transient UI elements to async command results.
package main

import "time"

// maxTransientsPerResult caps how many transient elements are attached to a single
// command result. Keeps response size bounded while still surfacing important UI events.
const maxTransientsPerResult = 10

// attachTransientElements appends any transient UI elements (toasts, alerts, snackbars)
// that appeared after `since` to the response data. Uses cmd.CreatedAt as the time window
// start — no arming state machine needed, unlike evidence capture which requires screenshots.
//
// The loop iterates backwards because enhanced actions are stored in chronological order
// (append-only buffer). The `break` on old timestamps relies on this sorted invariant.
func (h *ToolHandler) attachTransientElements(responseData map[string]any, since time.Time) {
	if h == nil || responseData == nil {
		return
	}
	// Subtract 500ms to tolerate browser/server clock skew. Timestamps on transient
	// elements come from Date.now() in the browser, while `since` is server-side time.
	sinceMs := since.UnixMilli() - 500
	allActions := h.capture.GetAllEnhancedActions()
	transients := make([]map[string]any, 0, 4)
	for i := len(allActions) - 1; i >= 0 && len(transients) < maxTransientsPerResult; i-- {
		a := allActions[i]
		// Enhanced actions buffer is append-only and chronologically sorted.
		// Once we hit an action older than the command start, no more transients to find.
		if a.Timestamp < sinceMs {
			break
		}
		if a.Type != "transient" {
			continue
		}
		transients = append(transients, map[string]any{
			"classification": a.Classification,
			"value":          a.Value,
			"role":           a.Role,
			"url":            a.URL,
			"timestamp":      a.Timestamp,
		})
	}
	if len(transients) > 0 {
		responseData["transient_elements"] = transients
	}
}
