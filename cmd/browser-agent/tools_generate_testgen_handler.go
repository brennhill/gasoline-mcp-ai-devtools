// Purpose: Defines the dedicated generate/testgen sub-handler.
// Why: Reduces ToolHandler responsibility by isolating test-generation orchestration.
// Docs: docs/features/feature/test-generation/index.md

package main

import (
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

// testGenHandlerDeps defines the narrow interface that testGenHandler needs from its parent.
type testGenHandlerDeps interface {
	mcp.LogBufferReader
	mcp.CaptureProvider
}

type testGenHandler struct {
	deps testGenHandlerDeps
}

func newTestGenHandler(deps testGenHandlerDeps) *testGenHandler {
	return &testGenHandler{deps: deps}
}

func (h *ToolHandler) testGen() *testGenHandler {
	return h.testGenHandler
}
