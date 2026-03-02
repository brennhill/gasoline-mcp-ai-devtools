// Purpose: Provides test fixture constructors shared by capture package tests.
// Why: Reduces duplicated test bootstrapping code and keeps test setup behavior consistent.
// Docs: docs/features/feature/self-testing/index.md

package capture

import (
	"path/filepath"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/server"
)

// setupTestCapture creates a new Capture instance for testing.
func setupTestCapture(t *testing.T) *Capture {
	t.Helper()
	return NewCapture()
}

// setupTestServer creates a test instance of Server with a temporary log file.
func setupTestServer(t *testing.T) (*server.Server, string) {
	t.Helper()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	srv, err := server.NewServer(logFile, 10)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return srv, logFile
}

// setupToolHandler is a placeholder that always returns nil.
// NOTE: ToolHandler and MCPHandler have not been moved to internal packages.
// Tests using this function should be skipped until the refactoring is complete.
func setupToolHandler(t *testing.T, server *server.Server, capture *Capture) any {
	t.Helper()
	return nil
}
