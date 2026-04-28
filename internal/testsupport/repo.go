// Package testsupport provides utilities shared across test files in the
// internal/* package tree.
//
// Helpers belong here only when (a) used by 2+ test packages and (b)
// cannot live in `*_test.go` due to Go's per-package test compilation.
// The bar is intentionally high to keep the package small and focused.
//
// Production code MUST NOT import this package — the helpers here are
// test-only and there is no API stability guarantee. Cross-package
// imports from `*_test.go` are expected and supported.
package testsupport

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ExpectedModulePath is the Go module path the repository's root go.mod
// must declare. RepoRoot verifies the discovered go.mod matches this so
// a stray go.mod in a fixture sub-tree (e.g., a future testdata module
// for plugin-loading tests) cannot masquerade as the repo root.
//
// If the project is ever forked or renamed, update this constant in lockstep
// with go.mod's `module` line.
const ExpectedModulePath = "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP"

// repoRootTB is the minimal testing.TB subset RepoRoot needs. Defined as
// an interface (instead of taking *testing.T or testing.TB directly) so
// repo_test.go can drive a fake implementation that captures Fatalf
// without aborting the surrounding *testing.T. *testing.T and *testing.B
// satisfy this interface implicitly.
type repoRootTB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// RepoRoot walks up from the current working directory looking for the
// nearest directory containing a `go.mod` file whose `module` directive
// matches ExpectedModulePath, and returns that directory. Used by contract
// tests that compare source against repo-rooted documentation.
//
// Resolves correctly under:
//   - `go test ./...` from repo root
//   - `cd internal/<pkg> && go test .`
//   - `go test -C <dir> .`
//   - IDE test runners that rebase cwd
//
// Why module-path verification: a future fixture sub-module (e.g., for
// testing plugin loading or vendored snippets) could place a stray go.mod
// inside the repo. Without the module-path check, RepoRoot would stop at
// the fixture and return a misleading "root" — silently rerouting
// contract tests to read fixture-relative paths. The check ensures the
// returned root is the canonical repo, not a nested module.
//
// Fails with t.Fatalf if the walk reaches the filesystem root without
// finding a matching go.mod. The error includes a remediation hint for
// tests that have changed cwd via `t.Chdir(t.TempDir())` to a non-module
// directory.
func RepoRoot(t repoRootTB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("testsupport.RepoRoot: os.Getwd: %v", err)
		return ""
	}
	for d := wd; ; {
		goModPath := filepath.Join(d, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			module, ok := readModulePath(goModPath)
			if ok && module == ExpectedModulePath {
				return d
			}
			// Found a go.mod but it's a different module — keep walking.
			// This handles the "fixture sub-module masquerades as root"
			// case described above.
		}
		parent := filepath.Dir(d)
		if parent == d {
			t.Fatalf("testsupport.RepoRoot: walked past filesystem root from %q without finding go.mod declaring module %q; if your test calls t.Chdir, do not chdir to a non-module directory before invoking this helper", wd, ExpectedModulePath)
			return ""
		}
		d = parent
	}
}

// readModulePath parses the `module <path>` directive from a go.mod file.
// Returns ("", false) on any read/parse failure; only the literal directive
// shape (`module <something>` on its own line) is recognized — block-form
// `module ( ... )` does not exist in current go.mod syntax.
//
// We bufio.Scanner the file rather than slurp the whole thing so a 0-byte
// or huge go.mod is bounded.
func readModulePath(goModPath string) (string, bool) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), true
		}
		if rest, ok := strings.CutPrefix(line, "module\t"); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), true
		}
		// Anything else on the first non-comment line means there's no
		// module directive (or it's malformed); bail rather than scan
		// the whole file.
		return "", false
	}
	return "", false
}
