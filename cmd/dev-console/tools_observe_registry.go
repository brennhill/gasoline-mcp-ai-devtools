// Purpose: Central registry for observe mode handlers and mode metadata.
// Why: Keeps mode definitions discoverable in one place and decouples registry updates from dispatch logic.
// Docs: docs/features/feature/mcp-persistent-server/index.md

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/observe"
)

// obs wraps an observe.Deps-accepting function as a ModeHandler.
// *ToolHandler satisfies observe.Deps, but Go requires explicit adaptation.
func obs(fn func(observe.Deps, JSONRPCRequest, json.RawMessage) JSONRPCResponse) ModeHandler {
	return func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return fn(h, req, args)
	}
}

// observeHandlers maps observe mode names to their handler functions.
var observeHandlers = map[string]ModeHandler{
	// Delegated to internal/tools/observe
	"errors":            obs(observe.GetBrowserErrors),
	"logs":              obs(observe.GetBrowserLogs),
	"extension_logs":    obs(observe.GetExtensionLogs),
	"network_waterfall": obs(observe.GetNetworkWaterfall),
	"network_bodies":    obs(observe.GetNetworkBodies),
	"websocket_events":  obs(observe.GetWSEvents),
	"websocket_status":  obs(observe.GetWSStatus),
	"actions":           obs(observe.GetEnhancedActions),
	"vitals":            obs(observe.GetWebVitals),
	"page":              obs(observe.GetPageInfo),
	"tabs":              obs(observe.GetTabs),
	"history":           obs(observe.AnalyzeHistory),
	"pilot":             obs(observe.ObservePilot),
	"timeline":          obs(observe.GetSessionTimeline),
	"error_bundles":     obs(observe.GetErrorBundles),
	"screenshot":        obs(observe.GetScreenshot),
	"storage":           obs(observe.GetStorage),
	"indexeddb":         obs(observe.GetIndexedDB),
	"summarized_logs":   obs(observe.GetSummarizedLogs),
	"transients":        obs(observe.GetTransients),
	// Local handlers
	"page_inventory":    method((*ToolHandler).toolObservePageInventory),
	"inbox":             method((*ToolHandler).toolObserveInbox),
	"command_result":    method((*ToolHandler).toolObserveCommandResult),
	"pending_commands":  method((*ToolHandler).toolObservePendingCommands),
	"failed_commands":   method((*ToolHandler).toolObserveFailedCommands),
	"saved_videos":      method((*ToolHandler).toolObserveSavedVideos),
	"recordings":        method((*ToolHandler).toolGetRecordings),
	"recording_actions": method((*ToolHandler).toolGetRecordingActions),
	"playback_results":  method((*ToolHandler).toolGetPlaybackResults),
	"log_diff_report":   method((*ToolHandler).toolGetLogDiffReport),
}

// observeAliases maps shorthand names to their canonical observe mode names.
var observeAliases = map[string]string{
	"network": "network_waterfall",
	"ws":      "websocket_events",
}

// serverSideObserveModes lists modes that don't depend on live extension data.
// Kept near observeHandlers so additions to one are visible near the other.
var serverSideObserveModes = map[string]bool{
	"command_result":    true,
	"pending_commands":  true,
	"failed_commands":   true,
	"saved_videos":      true,
	"recordings":        true,
	"recording_actions": true,
	"playback_results":  true,
	"log_diff_report":   true,
	"pilot":             true,
	"history":           true,
	"inbox":             true,
}

// getValidObserveModes returns a sorted, comma-separated list of valid observe modes.
func getValidObserveModes() string { return sortedMapKeys(observeHandlers) }
