// repo.go — Repo-root walk + go.mod parser. Used by contract tests
// that compare source against repo-rooted documentation. Package
// overview lives in doc.go; canonical TB-shaped interfaces in tb.go.

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

// repoRootChdirHint is a stable marker embedded in the
// "no go.mod ancestor" Fatalf message so tests can assert remediation
// guidance is present without coupling to surrounding prose. Worded as
// human-readable English so a real failure surfaces actionable text
// directly to the developer who tripped it; the test that pins this
// substring-matches the const just as well.
//
// Unexported because the only consumers are in-package: RepoRoot itself
// (which formats the marker into its Fatalf) and repo_test.go (which
// asserts on it). External code has no business depending on the prose
// of an internal helper's failure message.
const repoRootChdirHint = "[hint: avoid t.Chdir to non-module dir]"

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
// finding a matching go.mod. The error includes the repoRootChdirHint
// marker so tests can pin the remediation guidance is present.
func RepoRoot(t HelperFatalfTB) string {
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
//
// LIMITATION: this helper does NOT inject the os.Stat call, so tests
// running on a host where $TMPDIR's ancestors contain a go.mod cannot
// fully exercise the not-found branch. TestRepoRoot_FatalfWhenNoGoMod
// guards against that with a Skipf and a CI-only assertion (the branch
// MUST exercise in CI). A future test requiring full hermeticity (e.g.,
// one that wants to assert specific Fatalf prose without depending on
// host filesystem layout) would need a `statFn func(string)
// (fs.FileInfo, error)` indirection here. We have not added that yet
// because it would clutter the production walk for a single test.
func repoRootFromWd(t HelperFatalfTB, wd string) string {
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
			t.Fatalf("testsupport.RepoRoot: walked past filesystem root from %q without finding go.mod declaring module %q; if your test calls t.Chdir, do not chdir to a non-module directory before invoking this helper %s", wd, ExpectedModulePath, repoRootChdirHint)
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
//
// Quote-handling: trimModuleQuotes accepts a balanced double-quote pair
// and strips it; an unbalanced quote (e.g., `"foo` with no closing quote)
// is returned as-is so the caller can fail the equality check rather
// than silently producing a truncated module path. An empty-quoted
// directive (`module ""`) is rejected outright (returns ("", false))
// — declaring success on a malformed directive that names no module
// path is worse than declaring failure.
//
// Real-world go.mod variants handled:
//   - tab separator (`module\texample.com/foo`)
//   - multiple spaces between keyword and path
//   - leading comments and blank lines
//   - balanced quoted path
//   - inline comments after the path (`module example.com/foo // c`)
//   - trailing whitespace on the directive line
//   - CRLF line endings (Windows-checkout)
//
// NOT handled: BOM-prefixed go.mod files. TestReadModulePath_TableEdges
// pins the current behavior (returns "", false) so a future contributor
// who produces a BOM-prefixed file sees a clear failure.
func readModulePath(goModPath string) (string, bool) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		// Strip trailing CR (CRLF line endings on Windows-checkout
		// repositories). bufio.Scanner splits on \n, leaving \r if
		// present.
		line := strings.TrimRight(scanner.Text(), "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		rest, ok := cutDirective(line, "module")
		if !ok {
			// First non-comment line isn't a module directive — bail
			// rather than scan the whole file.
			return "", false
		}
		// Strip any inline `// comment` tail after the path.
		if i := strings.Index(rest, "//"); i >= 0 {
			rest = rest[:i]
		}
		path := trimModuleQuotes(strings.TrimSpace(rest))
		if path == "" {
			// `module ""` (empty-quoted) or whitespace-only path:
			// the directive is structurally a module declaration but
			// names nothing. Better to reject loudly than to claim
			// success on a malformed file.
			return "", false
		}
		return path, true
	}
	return "", false
}

// cutDirective returns the value portion of a `<keyword> <value>`
// directive, or ("", false) if the line does not start with the
// keyword followed by whitespace. Handles single-space, tab, and
// multi-space separators uniformly.
//
// Generalized from the original `cutModuleDirective` so a future
// parser of `go 1.24` or `toolchain ...` directives can reuse the same
// helper. Today only readModulePath calls it.
func cutDirective(line, keyword string) (string, bool) {
	rest, ok := strings.CutPrefix(line, keyword)
	if !ok {
		return "", false
	}
	if rest == "" {
		// Bare keyword with nothing after — malformed.
		return "", false
	}
	// First char must be whitespace (space or tab); anything else
	// (e.g., "modulefoo") is not a directive.
	if rest[0] != ' ' && rest[0] != '\t' {
		return "", false
	}
	return strings.TrimLeft(rest, " \t"), true
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
