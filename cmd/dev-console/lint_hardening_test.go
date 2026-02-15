// lint_hardening_test.go — Go test wrapper for custom lint rules.
// Runs scripts/lint-hardening.sh as a Go test so violations are caught
// by `go test` (including `go test -short`). Fast: only grep-based scans.
package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// projectRoot returns the repository root by navigating from this source file.
func projectRoot() string {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile = .../cmd/dev-console/lint_hardening_test.go
	return filepath.Join(filepath.Dir(thisFile), "..", "..")
}

// TestLintHardening runs the custom hardening lint script and fails the test
// if any violations are found. The script checks for bare goroutines, unchecked
// JSON encodes, missing headers, route sync, middleware, SafeGo closures, and
// queue overflow logging.
func TestLintHardening(t *testing.T) {
	t.Parallel()

	root := projectRoot()
	scriptPath := filepath.Join(root, "scripts", "lint-hardening.sh")

	cmd := exec.Command("bash", scriptPath) // #nosec G204 -- fixed script path, test-only
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("lint-hardening.sh failed (exit %v):\n%s", err, output)
	}
	t.Logf("lint-hardening.sh passed:\n%s", output)
}
