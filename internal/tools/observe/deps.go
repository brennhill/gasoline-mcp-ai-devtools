// deps.go — Dependency interface for the observe tool package.
package observe

import "github.com/dev-console/dev-console/internal/mcp"

// Deps provides all dependencies the observe handlers need.
// *ToolHandler in cmd/dev-console/ satisfies this interface.
type Deps interface {
	mcp.DiagnosticProvider
	mcp.CaptureProvider
	mcp.LogBufferReader
	mcp.A11yQueryExecutor
	mcp.NoiseFilterer
}
