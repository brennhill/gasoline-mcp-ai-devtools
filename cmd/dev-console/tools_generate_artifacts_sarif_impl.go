// Purpose: Implements generate(sarif) artifact assembly.
// Why: Keeps accessibility audit conversion flow scoped to SARIF concerns.

package main

import (
	"encoding/json"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/export"
)

func (h *ToolHandler) exportSARIFImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var arguments struct {
		Scope         string `json:"scope"`
		IncludePasses bool   `json:"include_passes"`
		SaveTo        string `json:"save_to"`
		// Internal-use path for workflows that already executed accessibility.
		A11yResult json.RawMessage `json:"a11y_result"`
	}
	if len(args) > 0 {
		if resp, stop := parseArgs(req, args, &arguments); stop {
			return resp
		}
	}

	// Use precomputed a11y results when available; otherwise run a11y audit.
	a11yResult := arguments.A11yResult
	if len(a11yResult) == 0 {
		if h.capture.IsExtensionConnected() {
			var err error
			a11yResult, err = h.ExecuteA11yQuery(arguments.Scope, nil, nil, false)
			if err != nil {
				a11yResult = json.RawMessage("{}")
			}
		} else {
			a11yResult = json.RawMessage("{}")
		}
	}

	// Convert to SARIF.
	sarifLog, err := export.ExportSARIF(a11yResult, export.SARIFExportOptions{
		Scope:         arguments.Scope,
		IncludePasses: arguments.IncludePasses,
		SaveTo:        arguments.SaveTo,
	})
	if err != nil {
		return fail(req, ErrNoData, "SARIF export failed: "+err.Error(), "Check a11y audit results and try again.")
	}

	// Marshal SARIFLog to a generic map for the MCP response.
	sarifJSON, err := json.Marshal(sarifLog)
	if err != nil {
		return fail(req, ErrNoData, "SARIF marshal failed: "+err.Error(), "Report this bug.")
	}
	var sarifMap map[string]any
	if err := json.Unmarshal(sarifJSON, &sarifMap); err != nil {
		return fail(req, ErrNoData, "SARIF unmarshal failed: "+err.Error(), "Report this bug.")
	}

	return succeed(req, "SARIF export complete", sarifMap)
}
