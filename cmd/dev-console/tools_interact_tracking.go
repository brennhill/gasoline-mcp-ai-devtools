// tools_interact_tracking.go — Tracked tab retarget logic for switch_tab.
// Extracts tab_id/url/title from completed switch_tab responses and updates
// server-side tracked tab state. See issue #271.
package main

import "encoding/json"

// applySwitchTabTracking extracts tab_id/url/title from a completed switch_tab
// response and updates the server-side tracked tab state.
// Only updates on success (status=complete, success=true, tab_id present).
//
// NOTE: This only runs in synchronous mode (the default). In async mode
// (background=true), server-side tracking is NOT immediately updated.
// The extension-side persistTrackedTab handles async retarget via the
// next /sync heartbeat. See issue #271.
func (h *ToolHandler) applySwitchTabTracking(correlationID string) {
	cmd, found := h.capture.GetCommandResult(correlationID)
	if !found || cmd == nil || cmd.Status != "complete" {
		return
	}

	var result map[string]any
	if err := json.Unmarshal(cmd.Result, &result); err != nil {
		return
	}

	success, _ := result["success"].(bool)
	if !success {
		return
	}

	tabIDFloat, _ := result["tab_id"].(float64)
	tabID := int(tabIDFloat)
	if tabID <= 0 {
		return
	}

	tabURL, _ := result["url"].(string)
	tabTitle, _ := result["title"].(string)
	h.capture.UpdateTrackedTab(tabID, tabURL, tabTitle)
}
