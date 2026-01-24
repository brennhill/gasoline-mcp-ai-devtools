package main

import (
	"testing"
)

func setupTestCapture(t *testing.T) *Capture {
	t.Helper()
	return NewCapture()
}

// setupToolHandler creates a NewToolHandler and registers cleanup to prevent goroutine leaks.
// The SessionStore's background goroutine is shut down when the test completes.
func setupToolHandler(t *testing.T, server *Server, capture *Capture) *MCPHandler {
	t.Helper()
	mcp := NewToolHandler(server, capture)
	t.Cleanup(func() {
		if mcp.toolHandler != nil && mcp.toolHandler.sessionStore != nil {
			mcp.toolHandler.sessionStore.Shutdown()
		}
	})
	return mcp
}
