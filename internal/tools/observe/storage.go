// Purpose: Provides observe tool implementation helpers for filtering and storage queries.
// Why: Centralizes observe query behavior so evidence filtering stays predictable.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/mcp"
	"github.com/dev-console/dev-console/internal/queries"
)

// GetStorage returns localStorage, sessionStorage, and cookies from the tracked tab.
func GetStorage(deps Deps, req mcp.JSONRPCRequest, _ json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrNoData,
			"No tab is being tracked. Open the Gasoline extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='storage'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	queryID, qerr := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "state_capture",
			Params: json.RawMessage(`{"action":"capture"}`),
		},
		10*time.Second,
		"",
	)
	if qerr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrQueueFull,
			"Command queue full: "+qerr.Error(),
			"Wait for in-flight commands to complete, then retry.",
		)}
	}

	result, err := cap.WaitForResult(queryID, 10*time.Second)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtTimeout,
			"Storage capture timeout: "+err.Error(),
			"Ensure the extension is connected and the page has loaded.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	var stateResult map[string]any
	if err := json.Unmarshal(result, &stateResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidJSON,
			"Failed to parse storage result: "+err.Error(),
			"Check extension logs for errors",
		)}
	}

	if errMsg, ok := stateResult["error"].(string); ok {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtError,
			"Storage capture failed: "+errMsg,
			"Check that the tab is accessible.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	response := map[string]any{
		"url":      stateResult["url"],
		"metadata": BuildResponseMetadata(cap, time.Now()),
	}
	if v, ok := stateResult["localStorage"]; ok {
		response["local_storage"] = v
	}
	if v, ok := stateResult["sessionStorage"]; ok {
		response["session_storage"] = v
	}
	if v, ok := stateResult["cookies"]; ok {
		response["cookies"] = v
	}

	// IndexedDB listing is best-effort: return storage data even if this probe fails.
	if indexeddb, err := getIndexedDBListing(cap); err != nil {
		response["indexeddb"] = map[string]any{
			"supported": false,
			"databases": []any{},
		}
		response["indexeddb_error"] = err.Error()
	} else {
		response["indexeddb"] = indexeddb
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("Browser storage", response)}
}
