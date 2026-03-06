// Purpose: Wires reproduction script generation into the MCP generate tool, delegating to internal/reproduction.
// Why: Keeps the cmd layer as a thin adapter while reproduction logic lives in a testable internal package.
// Docs: docs/features/feature/reproduction-scripts/index.md

package main

import (
	"encoding/json"
	"fmt"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/reproduction"
)

// Type aliases — keep existing code compiling without changes.
type ReproductionParams = reproduction.Params
type ReproductionResult = reproduction.Result
type ReproductionMeta = reproduction.Meta

// Function aliases for callers in tools_generate.go and other cmd/dev-console code.
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
		return fail(req, ErrInvalidParam, err, "Use 'gasoline' or 'playwright'", withParam("output_format"))
	}

	allActions := h.capture.GetAllEnhancedActions()
	actions := reproduction.FilterLastN(allActions, params.LastN)

	script := reproduction.GenerateScript(actions, params)
	result := reproduction.BuildResult(script, params, actions, allActions)

	summary := fmt.Sprintf("Reproduction script (%s, %d actions)", params.OutputFormat, len(actions))
	return succeed(req, summary, result)
}
