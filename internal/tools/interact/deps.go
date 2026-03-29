// Purpose: Declares the Deps interface that interact handlers require from the host server.
// Docs: docs/features/feature/interact-explore/index.md

package interact

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// Deps provides all dependencies the interact handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	mcp.CaptureProvider
	mcp.AsyncCommandDispatcher
}
