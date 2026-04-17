// Purpose: Declares the Deps interface that observe handlers require from the host server.
// Docs: docs/features/feature/observe/index.md

package observe

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"

// Deps provides all dependencies the observe handlers need.
// *ToolHandler in cmd/browser-agent/ satisfies this interface.
type Deps interface {
	mcp.DiagnosticProvider
	mcp.CaptureProvider
	mcp.LogBufferReader
	mcp.A11yQueryExecutor
	mcp.NoiseFilterer
}
