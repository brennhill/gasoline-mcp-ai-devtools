// Purpose: Provides configure tool implementation helpers for policy and rewrite flows.
// Why: Centralizes configure logic so policy/rewrite behavior remains deterministic and testable.
// Docs: docs/features/feature/config-profiles/index.md

package configure

import "github.com/dev-console/dev-console/internal/mcp"

// Deps provides all dependencies the configure handlers need.
// *ToolHandler in cmd/dev-console/ satisfies this interface.
type Deps interface {
	mcp.CaptureProvider
	mcp.LogBufferReader
}
