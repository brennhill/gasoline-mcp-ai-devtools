// Purpose: Canonical MCP server identity shared across runtime surfaces.
// Why: Prevents drift between daemon/server identity and emitted notification logger fields.

package identity

const (
	// MCPServerName is the canonical MCP server identity shown to clients and LLMs.
	MCPServerName = "gasoline-browser-devtools"
)

// LegacyMCPServerNames lists historical server names kept for backward compatibility.
var LegacyMCPServerNames = []string{
	"gasoline-agentic-browser",
	"gasoline",
}
