// tools_observe.go — MCP observe tool dispatcher and handlers.
// Docs: docs/features/feature/observe/index.md
// Handles all observe modes: errors, logs, network, websocket, actions, etc.
package main

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/dev-console/dev-console/internal/tools/observe"
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

// getValidObserveModes returns a sorted, comma-separated list of valid observe modes.
func getValidObserveModes() string {
	modes := make([]string, 0, len(observeHandlers))
	for mode := range observeHandlers {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	return strings.Join(modes, ", ")
}

// toolObserve dispatches observe requests based on the 'what' parameter.
func (h *ToolHandler) toolObserve(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		What string `json:"what"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
		}
	}

	if params.What == "" {
		validModes := getValidObserveModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'what' is missing", "Add the 'what' parameter and call again", withParam("what"), withHint("Valid values: "+validModes))}
	}

	if alias, ok := observeAliases[params.What]; ok {
		params.What = alias
	}

	handler, ok := observeHandlers[params.What]
	if !ok {
		validModes := getValidObserveModes()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrUnknownMode, "Unknown observe mode: "+params.What, "Use a valid mode from the 'what' enum", withParam("what"), withHint("Valid values: "+validModes))}
	}

	resp := handler(h, req, args)

	// Warn when extension is disconnected (except for server-side modes that don't need it)
	if !h.capture.IsExtensionConnected() && !serverSideObserveModes[params.What] {
		resp = h.prependDisconnectWarning(resp)
	}

	// Piggyback alerts: append as second content block if any pending
	alerts := h.drainAlerts()
	if len(alerts) > 0 {
		resp = h.appendAlertsToResponse(resp, alerts)
	}

	return resp
}

// serverSideObserveModes lists modes that don't depend on live extension data.
// Kept next to observeHandlers so additions to one are visible near the other.
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
}

// prependDisconnectWarning adds a warning to the first content block when the extension is disconnected.
func (h *ToolHandler) prependDisconnectWarning(resp JSONRPCResponse) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		return resp
	}

	warning := "⚠ Extension is not connected — results may be stale or empty. Ensure the Gasoline extension shows 'Connected' and a tab is tracked.\n\n"
	result.Content[0].Text = warning + result.Content[0].Text

	// Error impossible: simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}

// appendAlertsToResponse adds an alerts content block to an existing MCP response.
func (h *ToolHandler) appendAlertsToResponse(resp JSONRPCResponse, alerts []Alert) JSONRPCResponse {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return resp
	}

	alertText := formatAlertsBlock(alerts)
	result.Content = append(result.Content, MCPContentBlock{
		Type: "text",
		Text: alertText,
	})

	// Error impossible: MCPToolResult is a simple struct with no circular refs or unsupported types
	resultJSON, _ := json.Marshal(result)
	resp.Result = json.RawMessage(resultJSON)
	return resp
}
