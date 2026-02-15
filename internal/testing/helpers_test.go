package testing_test

import (
	"os"
	"path/filepath"
	"testing"

	testhelpers "github.com/dev-console/dev-console/internal/testing"
)

func TestSetupTestServer(t *testing.T) {
	t.Parallel()

	srv, logFile := testhelpers.SetupTestServer(t)
	if srv == nil {
		t.Fatal("SetupTestServer returned nil server")
	}
	if got := srv.GetLogCount(); got != 0 {
		t.Fatalf("new test server log count = %d, want 0", got)
	}
	if filepath.Base(logFile) != "test-logs.jsonl" {
		t.Fatalf("unexpected log file name: %q", logFile)
	}
	if _, err := os.Stat(filepath.Dir(logFile)); err != nil {
		t.Fatalf("expected log directory to exist: %v", err)
	}
}

func TestSetupTestCapture(t *testing.T) {
	t.Parallel()

	cap := testhelpers.SetupTestCapture(t)
	if cap == nil {
		t.Fatal("SetupTestCapture returned nil capture")
	}
}
