// deps.go — Dependency interface for the configure tool package.
package configure

import "github.com/dev-console/dev-console/internal/mcp"

// Deps provides all dependencies the configure handlers need.
// *ToolHandler in cmd/dev-console/ satisfies this interface.
type Deps interface {
	mcp.CaptureProvider
	mcp.LogBufferReader
}
