// helpers.go — Package-specific helpers for interact handlers.
// Purpose: Guard-check utilities that are specific to the interact tool's dependency injection pattern.
// Why: These functions use the GuardCheck type defined in deps.go and have no equivalent in internal/mcp.

package toolinteract

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// checkGuards runs guard checks in sequence. First blocker short-circuits.
func checkGuards(req mcp.JSONRPCRequest, guards ...GuardCheck) (mcp.JSONRPCResponse, bool) {
	for _, g := range guards {
		if resp, blocked := g(req); blocked {
			return resp, true
		}
	}
	return mcp.JSONRPCResponse{}, false
}

// checkGuardsWithOpts runs guard checks with StructuredError options.
func checkGuardsWithOpts(req mcp.JSONRPCRequest, opts []func(*mcp.StructuredError), guards ...GuardCheck) (mcp.JSONRPCResponse, bool) {
	for _, g := range guards {
		if resp, blocked := g(req, opts...); blocked {
			return resp, true
		}
	}
	return mcp.JSONRPCResponse{}, false
}
