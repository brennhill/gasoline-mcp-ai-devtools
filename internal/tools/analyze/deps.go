// Purpose: Provides analyze tool implementation helpers shared by command handlers.
// Why: Centralizes analyze logic to keep handler behavior consistent across command paths.
// Docs: docs/features/feature/analyze-tool/index.md

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
