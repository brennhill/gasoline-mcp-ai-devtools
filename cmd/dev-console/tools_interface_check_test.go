// tools_interface_check_test.go — Compile-time interface satisfaction assertions.
// If *ToolHandler doesn't satisfy a dep interface, compilation fails immediately.
package main

import (
	"github.com/dev-console/dev-console/internal/mcp"
	"github.com/dev-console/dev-console/internal/tools/observe"
)

// Phase 1: Shared dependency interfaces
var _ mcp.DiagnosticProvider = (*ToolHandler)(nil)
var _ mcp.AsyncCommandDispatcher = (*ToolHandler)(nil)

// Phase 2: Tool-specific dependency interfaces
var _ observe.Deps = (*ToolHandler)(nil)
