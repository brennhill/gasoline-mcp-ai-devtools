// Purpose: Defines shared workflow aliases/helpers for interact compound actions.
// Why: Keeps workflow glue code centralized across navigate/form/a11y workflow handlers.

package toolinteract

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"time"

	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// WorkflowStep — type alias delegated to internal/tools/interact package.
type WorkflowStep = act.WorkflowStep

// FormField — type alias delegated to internal/tools/interact package.
type FormField = act.FormField

// isErrorResponse — delegated to internal/tools/interact package.
func isErrorResponse(resp mcp.JSONRPCResponse) bool {
	return act.IsErrorResponse(resp)
}

// isNonFinalResponse — delegated to internal/tools/interact package.
func isNonFinalResponse(resp mcp.JSONRPCResponse) bool {
	return act.IsNonFinalResponse(resp)
}

// responseStatus — delegated to internal/tools/interact package.
func responseStatus(resp mcp.JSONRPCResponse) string {
	return act.ResponseStatus(resp)
}

// workflowResult — delegated to internal/tools/interact package.
func workflowResult(req mcp.JSONRPCRequest, workflow string, trace []WorkflowStep, lastResp mcp.JSONRPCResponse, start time.Time) mcp.JSONRPCResponse {
	return act.WorkflowResult(req, workflow, trace, lastResp, start)
}
