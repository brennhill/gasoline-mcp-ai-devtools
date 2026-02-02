// helpers.go — Shared test helpers for internal packages.
// Provides factory functions for creating test instances of core types.
package testing

import (
	"path/filepath"
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/server"
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
func SetupTestCapture(t *testing.T) *capture.Capture {
	t.Helper()
	return capture.NewCapture()
}

// SetupToolHandler creates a ToolHandler for testing.
// This is a placeholder that may need adjustment based on actual ToolHandler implementation.
// For now, it returns nil since ToolHandler doesn't exist in internal packages yet.
func SetupToolHandler(t *testing.T, srv *server.Server, cap *capture.Capture) interface{} {
	t.Helper()
	// TODO: Implement when ToolHandler is moved to internal packages
	return nil
}
