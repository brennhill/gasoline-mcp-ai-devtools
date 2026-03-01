// Purpose: Declares the Deps interface that configure handlers require from the host server.
// Docs: docs/features/feature/config-profiles/index.md

package configure

import "github.com/dev-console/dev-console/internal/mcp"

// Deps provides all dependencies the configure handlers need.
// *ToolHandler in cmd/dev-console/ satisfies this interface.
type Deps interface {
	mcp.CaptureProvider
	mcp.LogBufferReader
}
