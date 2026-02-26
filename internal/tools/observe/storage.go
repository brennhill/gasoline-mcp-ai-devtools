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

// parseSummaryParam extracts the summary boolean from request args. Defaults to true.
func parseSummaryParam(args json.RawMessage) bool {
	if len(args) == 0 {
		return true
	}
	var p struct {
		Summary *bool `json:"summary"`
	}
	if json.Unmarshal(args, &p) != nil || p.Summary == nil {
		return true
	}
	return *p.Summary
}

// summarizeStorageMap returns a summary of a key-value storage map.
func summarizeStorageMap(data map[string]any) map[string]any {
	keys := make([]string, 0, len(data))
	totalBytes := 0
	for k, v := range data {
		keys = append(keys, k)
		totalBytes += len(k)
		if s, ok := v.(string); ok {
			totalBytes += len(s)
		} else {
			b, _ := json.Marshal(v)
			totalBytes += len(b)
		}
	}
	sampleKeys := keys
	if len(sampleKeys) > 5 {
		sampleKeys = sampleKeys[:5]
	}
	return map[string]any{
		"key_count":   len(data),
		"total_bytes": totalBytes,
		"sample_keys": sampleKeys,
	}
}

// summarizeCookies returns a summary of a cookie array.
func summarizeCookies(cookies []any) map[string]any {
	names := make([]string, 0, len(cookies))
	totalBytes := 0
	for _, c := range cookies {
		if m, ok := c.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				names = append(names, name)
			}
			b, _ := json.Marshal(m)
			totalBytes += len(b)
		}
	}
	sampleKeys := names
	if len(sampleKeys) > 5 {
		sampleKeys = sampleKeys[:5]
	}
	return map[string]any{
		"key_count":   len(cookies),
		"total_bytes": totalBytes,
		"sample_keys": sampleKeys,
	}
}

// GetStorage returns localStorage, sessionStorage, and cookies from the tracked tab.
func GetStorage(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	summary := parseSummaryParam(args)
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

	if summary {
		// Summary mode: return key counts and byte estimates instead of full data
		if v, ok := stateResult["localStorage"].(map[string]any); ok {
			response["local_storage"] = summarizeStorageMap(v)
		}
		if v, ok := stateResult["sessionStorage"].(map[string]any); ok {
			response["session_storage"] = summarizeStorageMap(v)
		}
		if v, ok := stateResult["cookies"].([]any); ok {
			response["cookies"] = summarizeCookies(v)
		}
	} else {
		// Full mode: return raw key-value pairs
		if v, ok := stateResult["localStorage"]; ok {
			response["local_storage"] = v
		}
		if v, ok := stateResult["sessionStorage"]; ok {
			response["session_storage"] = v
		}
		if v, ok := stateResult["cookies"]; ok {
			response["cookies"] = v
		}
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
