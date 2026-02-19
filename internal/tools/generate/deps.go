// deps.go — Dependency interface for the generate tool package.
package generate

import (
	"github.com/dev-console/dev-console/internal/annotation"
	"github.com/dev-console/dev-console/internal/mcp"
)

// Deps provides all dependencies the generate handlers need.
// *ToolHandler in cmd/dev-console/ satisfies this interface.
type Deps interface {
	mcp.CaptureProvider
	mcp.LogBufferReader
	mcp.A11yQueryExecutor

	// Annotation access for visual_test, annotation_report, annotation_issues.
	GetAnnotationStore() *annotation.Store
}
