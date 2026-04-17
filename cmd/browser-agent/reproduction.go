// Purpose: Wires reproduction script generation into the MCP generate tool, delegating to internal/reproduction.
// Why: Keeps the cmd layer as a thin adapter while reproduction logic lives in a testable internal package.
// Docs: docs/features/feature/reproduction-scripts/index.md

package main

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/reproduction"
)

// Function aliases for callers in tools_generate.go and other cmd/browser-agent code.
var (
	filterLastN       = reproduction.FilterLastN
	escapeJS          = reproduction.EscapeJS
	chopString        = reproduction.ChopString
	writePauseComment = reproduction.WritePauseComment
	playwrightStep    = reproduction.PlaywrightStep
	playwrightLocator = reproduction.PlaywrightLocator
	describeElement   = reproduction.DescribeElement
)

// toolGetReproductionScript generates a reproduction script from captured actions.
func (h *ToolHandler) toolGetReproductionScript(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	params := reproduction.ParseParams(args)

	if err := reproduction.ValidateOutputFormat(params.OutputFormat); err != "" {
		return fail(req, ErrInvalidParam, err, "Use 'kaboom' or 'playwright'", withParam("output_format"))
	}

	allActions := h.capture.GetAllEnhancedActions()
	actions := reproduction.FilterLastN(allActions, params.LastN)

	script := reproduction.GenerateScript(actions, params)
	result := reproduction.BuildResult(script, params, actions, allActions)

	summary := fmt.Sprintf("Reproduction script (%s, %d actions)", params.OutputFormat, len(actions))
	return succeed(req, summary, result)
}
