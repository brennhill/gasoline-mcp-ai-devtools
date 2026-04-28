// Package testsupport provides utilities shared across test files in the
// internal/* package tree.
//
// Helpers belong here only when (a) used by 2+ test packages and (b)
// cannot live in `*_test.go` due to Go's per-package test compilation.
// The bar is intentionally high to keep the package small and focused.
//
// Production code MUST NOT import this package — the helpers here are
// test-only and there is no API stability guarantee. Cross-package
// imports from `*_test.go` are expected and supported. The
// "MUST NOT import" rule is enforced by
// TestPackageNotImportedByProductionCode in package_isolation_test.go;
// the build does not have a tag-gate because that would require every
// caller's `go test` invocation to opt in via `-tags`.
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
// This is intentionally a hand-maintained constant rather than a value
// auto-derived from go.mod: deriving from go.mod would defeat the
// drift-detection purpose. TestExpectedModulePath_MatchesGoMod
// (repo_test.go) cross-pins the const against go.mod so a fork or
// rename that updates one without the other fires a clear test failure.
//
// If the project is ever forked or renamed, update this constant in
// lockstep with go.mod's `module` line.
const ExpectedModulePath = "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP"

// RepoRootChdirHint is a stable marker embedded in the
// "no go.mod ancestor" Fatalf message so tests can assert remediation
// guidance is present without coupling to surrounding prose. Production
// callers should not depend on the marker; it exists purely for test
// observability.
const RepoRootChdirHint = "[hint:t-chdir-non-module]"

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
// finding a matching go.mod. The error includes the RepoRootChdirHint
// marker so tests can pin the remediation guidance is present.
func RepoRoot(t repoRootTB) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("testsupport.RepoRoot: os.Getwd: %v", err)
		return ""
	}
	return repoRootFromWd(t, wd)
}

// repoRootFromWd is the wd-injectable form of RepoRoot. Tests use this
// directly to exercise the not-found branch without relying on
// `t.Chdir`-into-a-tempdir tricks (which themselves depend on whether
// `$TMPDIR` happens to sit inside a Go module on the host).
func repoRootFromWd(t repoRootTB, wd string) string {
	t.Helper()
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
			t.Fatalf("testsupport.RepoRoot: walked past filesystem root from %q without finding go.mod declaring module %q; if your test calls t.Chdir, do not chdir to a non-module directory before invoking this helper %s", wd, ExpectedModulePath, RepoRootChdirHint)
			return ""
		}
		d = parent
	}
}

// ResolvePathsEqual reports whether two filesystem paths refer to the
// same canonical location after symlink resolution. Used by tests that
// compare paths returned from APIs that may resolve `/var` ↔
// `/private/var` (macOS) or other platform-symlink quirks.
//
// Both inputs MUST exist on disk; missing paths fail the test via
// t.Fatalf with a clear diagnostic. The boolean result is true iff
// EvalSymlinks(got) == EvalSymlinks(want); use this in `if !equal`
// branches that emit test-specific error messages.
func ResolvePathsEqual(t repoRootTB, got, want string) bool {
	t.Helper()
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("testsupport.ResolvePathsEqual: EvalSymlinks(%q): %v", got, err)
		return false
	}
	wantResolved, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("testsupport.ResolvePathsEqual: EvalSymlinks(%q): %v", want, err)
		return false
	}
	return gotResolved == wantResolved
}

// readModulePath parses the `module <path>` directive from a go.mod file.
// Returns ("", false) on any read/parse failure; only the literal directive
// shape (`module <something>` on its own line) is recognized — block-form
// `module ( ... )` does not exist in current go.mod syntax.
//
// We bufio.Scanner the file rather than slurp the whole thing so a 0-byte
// or huge go.mod is bounded.
//
// Quote-handling: trimModuleQuotes accepts a balanced double-quote pair
// and strips it; an unbalanced quote (e.g., `"foo` with no closing quote)
// is returned as-is so the caller can fail the equality check rather
// than silently producing a truncated module path.
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
			return trimModuleQuotes(strings.TrimSpace(rest)), true
		}
		if rest, ok := strings.CutPrefix(line, "module\t"); ok {
			return trimModuleQuotes(strings.TrimSpace(rest)), true
		}
		// Anything else on the first non-comment line means there's no
		// module directive (or it's malformed); bail rather than scan
		// the whole file.
		return "", false
	}
	return "", false
}

// trimModuleQuotes strips a balanced surrounding double-quote pair from
// the module path. Earlier this used strings.Trim, which would also strip
// a single trailing or leading quote — masking a malformed directive
// like `module "foo` as the valid path `foo`. Requiring both ends keeps
// the parser honest: a malformed quote pair returns the raw string and
// the caller's path-equality check fails loudly.
func trimModuleQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
