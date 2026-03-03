// Purpose: Implements generate(har) artifact assembly.
// Why: Keeps HAR export/serialization behavior isolated from other generate formats.

package main

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/export"
)

func (h *ToolHandler) exportHARImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		URL       string `json:"url"`
		Method    string `json:"method"`
		StatusMin int    `json:"status_min"`
		StatusMax int    `json:"status_max"`
		SaveTo    string `json:"save_to"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	bodies := h.capture.GetNetworkBodies()
	waterfall := h.capture.GetNetworkWaterfallEntries()
	filter := capture.NetworkBodyFilter{
		URLFilter: params.URL,
		Method:    params.Method,
		StatusMin: params.StatusMin,
		StatusMax: params.StatusMax,
	}

	if params.SaveTo != "" {
		result, err := export.ExportHARMergedToFile(bodies, waterfall, filter, version, params.SaveTo)
		if err != nil {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrExportFailed, "HAR file export failed: "+err.Error(), "Check the save_to path and try again",
			)}
		}
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(
			fmt.Sprintf("HAR exported to %s (%d entries)", result.SavedTo, result.EntriesCount), result,
		)}
	}

	harLog := export.ExportHARMerged(bodies, waterfall, filter, version)
	summary := fmt.Sprintf("HAR export (%d entries)", len(harLog.Log.Entries))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, harLog)}
}
