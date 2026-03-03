// Purpose: Defines the dedicated generate/testgen sub-handler.
// Why: Reduces ToolHandler responsibility by isolating test-generation orchestration.
// Docs: docs/features/feature/test-generation/index.md

package main

type testGenHandler struct {
	parent *ToolHandler
}

func newTestGenHandler(parent *ToolHandler) *testGenHandler {
	return &testGenHandler{parent: parent}
}

func (h *ToolHandler) testGen() *testGenHandler {
	if h.testGenHandler == nil {
		h.testGenHandler = newTestGenHandler(h)
	}
	return h.testGenHandler
}
