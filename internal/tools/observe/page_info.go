// page_info.go — Page information and readiness checks for the observe tool.
// Purpose: Provides GetPageInfo handler for observe(what:"page") mode.
// Why: Extracted from analysis.go to keep files under 800 LOC.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
)

// GetPageInfo returns information about the currently tracked page.
func GetPageInfo(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, tabID, trackedURL := cap.GetTrackingStatus()
	trackedTitle := cap.GetTrackedTabTitle()

	pageURL := resolvePageURL(cap, trackedURL)
	pageTitle := resolvePageTitle(deps, trackedTitle)

	cspRestricted, cspLevel := cap.GetCSPStatus()
	tabStatus := cap.GetTabStatus()

	// Each state getter acquires c.mu.RLock independently. Between calls, state
	// could change (e.g., extension disconnects between GetTabStatus and
	// IsExtensionConnected). This is acceptable for an advisory readiness signal
	// — the next observe(what:"page") call will correct any inconsistency.
	extensionConnected := cap.IsExtensionConnected()

	// Use IsPilotEnabled (defaults false) instead of IsPilotActionAllowed (defaults
	// true during startup). This avoids briefly reporting page_ready_for_commands=true
	// before the first extension sync confirms pilot status.
	pilotEnabled := cap.IsPilotEnabled()

	// page_ready_for_commands is true when all four conditions hold:
	//   1. extensionConnected — WebSocket link to extension is live
	//   2. pilotEnabled       — AI Web Pilot is enabled in extension settings
	//   3. enabled            — a tab is actively being tracked
	//   4. tabStatus=="complete" — the tracked tab has finished loading
	pageReady := extensionConnected && pilotEnabled && enabled && tabStatus == "complete"

	result := map[string]any{
		"url":                     pageURL,
		"title":                   pageTitle,
		"tracked":                 enabled,
		"csp_restricted":          cspRestricted,
		"csp_level":               cspLevel,
		"tab_status":              tabStatus,
		"page_ready_for_commands": pageReady,
		"metadata":                BuildResponseMetadata(cap, time.Now()),
	}
	if tabID > 0 {
		result["tab_id"] = tabID
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Page info", result)}
}

func resolvePageURL(cap *capture.Capture, trackedURL string) string {
	if trackedURL != "" {
		return trackedURL
	}
	waterfallEntries := cap.GetNetworkWaterfallEntries()
	if len(waterfallEntries) > 0 {
		return waterfallEntries[len(waterfallEntries)-1].PageURL
	}
	return ""
}

func resolvePageTitle(deps Deps, trackedTitle string) string {
	if trackedTitle != "" {
		return trackedTitle
	}
	entries, _ := deps.GetLogEntries()
	for i := len(entries) - 1; i >= 0; i-- {
		if title, ok := entries[i]["title"].(string); ok && title != "" {
			return title
		}
	}
	return ""
}
