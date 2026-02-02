// setup.go — Shared test setup helpers for internal packages.
// Provides factory functions for creating test instances of core types.
// This file is NOT a test file (no _test.go suffix) so it can be imported by tests in other packages.
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
