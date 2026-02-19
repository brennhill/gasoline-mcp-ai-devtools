// deps.go — Dependency interface for the interact tool package.
package interact

import "github.com/dev-console/dev-console/internal/mcp"

// Deps provides all dependencies the interact handlers need.
// *ToolHandler in cmd/dev-console/ satisfies this interface.
type Deps interface {
	mcp.CaptureProvider
	mcp.AsyncCommandDispatcher
}
