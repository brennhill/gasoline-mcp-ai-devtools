// Purpose: Defines observe handler types and top-level mode dispatch map.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/mcp"
)

// MaxObserveLimit caps the limit parameter to prevent oversized responses.
const MaxObserveLimit = 1000

// Handler is the function signature for observe tool handlers.
type Handler func(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse

// Handlers maps observe mode names to their handler functions.
// Modes not in this map (command_result, pending_commands, etc.) are handled by cmd/dev-console.
var Handlers = map[string]Handler{
	"errors":            GetBrowserErrors,
	"logs":              GetBrowserLogs,
	"extension_logs":    GetExtensionLogs,
	"network_waterfall": GetNetworkWaterfall,
	"network_bodies":    GetNetworkBodies,
	"websocket_events":  GetWSEvents,
	"websocket_status":  GetWSStatus,
	"actions":           GetEnhancedActions,
	"vitals":            GetWebVitals,
	"page":              GetPageInfo,
	"tabs":              GetTabs,
	"history":           AnalyzeHistory,
	"pilot":             ObservePilot,
	"timeline":          GetSessionTimeline,
	"error_bundles":     GetErrorBundles,
	"screenshot":        GetScreenshot,
	"storage":           GetStorage,
	"indexeddb":         GetIndexedDB,
	"summarized_logs":   GetSummarizedLogs,
	"transients":        GetTransients,
}

// clampLimit applies default and max bounds to a limit parameter.
func clampLimit(limit, defaultVal int) int {
	if limit <= 0 {
		return defaultVal
	}
	if limit > MaxObserveLimit {
		return MaxObserveLimit
	}
	return limit
}
