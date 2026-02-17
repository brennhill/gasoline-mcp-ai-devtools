// Purpose: Owns helpers.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

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

// SetupToolHandler has been removed - ToolHandler is in cmd/dev-console and not
// available to internal packages. Tests requiring ToolHandler should be integration tests
// in cmd/dev-console with the //go:build integration tag.
