// Purpose: Defines shared workflow aliases/helpers for interact compound actions.
// Why: Keeps workflow glue code centralized across navigate/form/a11y workflow handlers.

package main

import (
	act "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/interact"
)

// WorkflowStep — type alias delegated to internal/tools/interact package.
type WorkflowStep = act.WorkflowStep

// FormField — type alias delegated to internal/tools/interact package.
type FormField = act.FormField

// isErrorResponse — delegated to internal/tools/interact package.
func isErrorResponse(resp JSONRPCResponse) bool {
	return act.IsErrorResponse(resp)
}

