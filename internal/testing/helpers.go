// Purpose: Provides shared internal test bootstrap helpers for server and capture instances.
// Why: Reduces duplicated test setup logic and keeps fixture initialization behavior consistent.
// Docs: docs/features/feature/self-testing/index.md

package testing

import (
	"path/filepath"
	"testing"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/server"
)

// SetupTestServer creates a test instance of Server with a temporary log file.
func SetupTestServer(t *testing.T) (*server.Server, string) {
	t.Helper()

	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test-logs.jsonl")

	srv, err := server.NewServer(logFile, 10)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	return srv, logFile
}

// SetupTestCapture creates a test instance of Capture.
func SetupTestCapture(t *testing.T) *capture.Store {
	t.Helper()
	return capture.NewCapture()
}

// SetupToolHandler has been removed - ToolHandler is in cmd/dev-console and not
// available to internal packages. Tests requiring ToolHandler should be integration tests
// in cmd/dev-console with the //go:build integration tag.
