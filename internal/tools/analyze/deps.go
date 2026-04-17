// Purpose: Declares the Deps interface that analyze handlers require from the host server.
// Docs: docs/features/feature/analyze-tool/index.md

package analyze

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
)

// Deps provides all dependencies the analyze handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	mcp.CaptureProvider
	mcp.LogBufferReader
}
