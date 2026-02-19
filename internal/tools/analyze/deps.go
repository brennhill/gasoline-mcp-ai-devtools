// deps.go — Dependency interface for the analyze tool package.
package analyze

import (
	"github.com/dev-console/dev-console/internal/mcp"
)

// Deps provides all dependencies the analyze handlers need.
// *ToolHandler in cmd/dev-console/ satisfies this interface.
type Deps interface {
	mcp.CaptureProvider
	mcp.LogBufferReader
}
