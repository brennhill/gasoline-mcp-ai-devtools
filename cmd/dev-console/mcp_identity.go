// mcp_identity.go - Canonical MCP identity constants and compatibility helpers.
// Purpose: Centralize server naming used by install/config/runtime handshake surfaces.

package main

const (
	// mcpServerName is the canonical MCP server identity shown to clients and LLMs.
	mcpServerName = "gasoline-browser-devtools"
)

var legacyMCPServerNames = []string{
	"gasoline-agentic-browser",
	"gasoline",
}

