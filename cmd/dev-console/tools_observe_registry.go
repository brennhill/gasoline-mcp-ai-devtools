// Purpose: Central registry for observe mode handlers and mode metadata.
// Why: Keeps mode definitions discoverable in one place and decouples registry updates from dispatch logic.
// Docs: docs/features/feature/observe/index.md

package main

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/observe"
)

// ObserveHandler is the function signature for observe tool handlers.
type ObserveHandler func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse

// observeHandlers maps observe mode names to their handler functions.
// Modes handled by internal/tools/observe delegate to the extracted package.
// Modes that depend on async/recording subsystems remain local.
var observeHandlers = map[string]ObserveHandler{
	// Delegated to internal/tools/observe
	"errors": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetBrowserErrors(h, req, args)
	},
	"logs": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetBrowserLogs(h, req, args)
	},
	"extension_logs": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetExtensionLogs(h, req, args)
	},
	"network_waterfall": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetNetworkWaterfall(h, req, args)
	},
	"network_bodies": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetNetworkBodies(h, req, args)
	},
	"websocket_events": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetWSEvents(h, req, args)
	},
	"websocket_status": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetWSStatus(h, req, args)
	},
	"actions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetEnhancedActions(h, req, args)
	},
	"vitals": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetWebVitals(h, req, args)
	},
	"page": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetPageInfo(h, req, args)
	},
	"tabs": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetTabs(h, req, args)
	},
	"history": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.AnalyzeHistory(h, req, args)
	},
	"pilot": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.ObservePilot(h, req, args)
	},
	"timeline": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetSessionTimeline(h, req, args)
	},
	"error_bundles": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetErrorBundles(h, req, args)
	},
	"screenshot": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetScreenshot(h, req, args)
	},
	"storage": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetStorage(h, req, args)
	},
	"indexeddb": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetIndexedDB(h, req, args)
	},
	"summarized_logs": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetSummarizedLogs(h, req, args)
	},
	"transients": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return observe.GetTransients(h, req, args)
	},
	// Composite: page inventory (#318)
	"page_inventory": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObservePageInventory(req, args)
	},
	// Push inbox handler
	"inbox": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObserveInbox(req, args)
	},
	// Local handlers — depend on async/recording subsystems
	"command_result": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObserveCommandResult(req, args)
	},
	"pending_commands": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObservePendingCommands(req, args)
	},
	"failed_commands": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObserveFailedCommands(req, args)
	},
	"saved_videos": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolObserveSavedVideos(req, args)
	},
	"recordings": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetRecordings(req, args)
	},
	"recording_actions": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetRecordingActions(req, args)
	},
	"playback_results": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetPlaybackResults(req, args)
	},
	"log_diff_report": func(h *ToolHandler, req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
		return h.toolGetLogDiffReport(req, args)
	},
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
func getValidObserveModes() string {
	modes := make([]string, 0, len(observeHandlers))
	for mode := range observeHandlers {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}
