// Purpose: Provides observe tool implementation helpers for filtering and storage queries.
// Why: Centralizes observe query behavior so evidence filtering stays predictable.
// Docs: docs/features/feature/observe/index.md

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
