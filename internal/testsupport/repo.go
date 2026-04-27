// Package testsupport provides utilities shared across test files in the
// internal/* package tree. The package is import-tagged INTERNAL so it
// cannot be imported by callers outside this module — and the helpers here
// are intended for test contexts only (the package would normally live in
// a `_test.go` file, but Go's per-package test compilation prevents reuse
// across packages — hence the dedicated package).
package testsupport

import (
	"os"
	"path/filepath"
	"testing"
)

// RepoRoot walks up from the current working directory looking for the
// nearest directory containing a `go.mod` file and returns it. Used by
// contract tests that compare source against repo-rooted documentation.
//
// Resolves correctly under:
//   - `go test ./...` from repo root
//   - `cd internal/<pkg> && go test .`
//   - `go test -C <dir> .`
//   - IDE test runners that rebase cwd
//
// Fails with t.Fatalf if the walk reaches the filesystem root without
// finding go.mod. The error includes a remediation hint for tests that
// have changed cwd via `t.Chdir(t.TempDir())` to a non-module directory.
func RepoRoot(t testing.TB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("testsupport.RepoRoot: os.Getwd: %v", err)
	}
	for d := wd; ; {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			t.Fatalf("testsupport.RepoRoot: walked past filesystem root from %q without finding go.mod; if your test calls t.Chdir, do not chdir to a non-module directory before invoking this helper", wd)
		}
		d = parent
	}
}
