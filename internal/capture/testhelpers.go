// Purpose: Owns testhelpers.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// testhelpers.go — Test helpers for capture package tests.
// Provides factory functions for creating test instances.
package capture

import (
	"path/filepath"
	"testing"

	"github.com/dev-console/dev-console/internal/server"
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
