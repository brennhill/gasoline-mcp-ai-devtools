// Purpose: Dispatches IndexedDB queries to the extension and formats database/store enumeration responses.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/json"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

const indexedDBQueryTimeout = 10 * time.Second

// GetIndexedDB returns rows from one IndexedDB object store.
func GetIndexedDB(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params struct {
		Database string `json:"database"`
		Store    string `json:"store"`
		Limit    int    `json:"limit"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Database == "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'database' is missing for observe(what='indexeddb')",
			"Add the 'database' parameter and call again.",
			mcp.WithParam("database"),
		)}
	}
	if params.Store == "" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrMissingParam,
			"Required parameter 'store' is missing for observe(what='indexeddb')",
			"Add the 'store' parameter and call again.",
			mcp.WithParam("store"),
		)}
	}
	params.Limit = clampLimit(params.Limit, 100)

	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrNoData,
			"No tab is being tracked. Open the Gasoline extension popup and click 'Track This Tab'.",
			"Track a tab first, then call observe with what='indexeddb'.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	storeData, err := getIndexedDBEntries(cap, params.Database, params.Store, params.Limit)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrExtError,
			"IndexedDB inspection failed: "+err.Error(),
			"Ensure the tab is accessible and the database/store names are correct.",
			mcp.WithHint(deps.DiagnosticHintString()),
		)}
	}

	entries, _ := storeData["entries"].([]any)
	count := len(entries)
	if c, ok := toInt(storeData["count"]); ok {
		count = c
	}

	response := map[string]any{
		"database": params.Database,
		"store":    params.Store,
		"entries":  entries,
		"count":    count,
		"limit":    params.Limit,
		"metadata": BuildResponseMetadata(cap, time.Now()),
	}
	if v, ok := storeData["object_stores"]; ok {
		response["object_stores"] = v
	}

	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.JSONResponse("IndexedDB entries", response)}
}
