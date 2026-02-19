// reproduction.go — Type aliases bridging internal/reproduction into package main.
// The reproduction logic lives in internal/reproduction.
package main

import (
	"encoding/json"
	"fmt"

	"github.com/dev-console/dev-console/internal/reproduction"
)

// Type aliases — keep existing code compiling without changes.
type ReproductionParams = reproduction.Params
type ReproductionResult = reproduction.Result
type ReproductionMeta = reproduction.Meta

// Function aliases for callers in tools_generate.go and other cmd/dev-console code.
var (
	filterLastN     = reproduction.FilterLastN
	escapeJS        = reproduction.EscapeJS
	chopString      = reproduction.ChopString
	writePauseComment = reproduction.WritePauseComment
	playwrightStep  = reproduction.PlaywrightStep
	playwrightLocator = reproduction.PlaywrightLocator
	describeElement = reproduction.DescribeElement
)

// toolGetReproductionScriptImpl generates a reproduction script from captured actions.
func (h *ToolHandler) toolGetReproductionScriptImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	params := reproduction.ParseParams(args)

	if err := reproduction.ValidateOutputFormat(params.OutputFormat); err != "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam, err, "Use 'gasoline' or 'playwright'", withParam("output_format"),
		)}
	}

	allActions := h.capture.GetAllEnhancedActions()
	actions := reproduction.FilterLastN(allActions, params.LastN)

	script := reproduction.GenerateScript(actions, params)
	result := reproduction.BuildResult(script, params, actions, allActions)

	summary := fmt.Sprintf("Reproduction script (%s, %d actions)", params.OutputFormat, len(actions))
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse(summary, result)}
}
