// package_isolation_test.go — Enforces the package doc's "Production
// code MUST NOT import this package" rule. The package ships
// non-`_test.go` files (faket.go, repo.go, paths.go, astutil.go) because
// `_test.go` files in package P cannot be imported by `_test.go` files
// in package Q — Go's test compilation is per-package. The cost is
// that the testsupport symbols (FakeT, RepoRoot, ImportQualifiers, ...)
// are technically importable from production code.
//
// This test walks every non-`_test.go` Go file under the repo root and
// fails if any of them imports `internal/testsupport`. It runs in CI
// alongside every other test in this package.
//
// Why not a build tag (`//go:build testsupport`)? A build tag would
// require every consumer's `go test` to opt in via `-tags testsupport`,
// which is friction every test-running contributor would have to
// remember (and CI would have to encode). The contract test gives the
// same guarantee — at the cost of running once per `go test` invocation
// against this package — without changing any consumer's invocation.

package testsupport

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// testsupportImportPath is the canonical Go import path of this package,
// derived from ExpectedModulePath so a fork or rename only updates one
// site. Earlier this was a hand-maintained duplicate; the drift-detection
// const itself (ExpectedModulePath) is already validated by
// TestExpectedModulePath_MatchesGoMod, so deriving here adds no risk.
var testsupportImportPath = ExpectedModulePath + "/internal/testsupport"

// skippedWalkDirs enumerates directory names that the production-import
// scan must not descend into. The set is intentionally heterogeneous —
// different sources, different drift risks. New additions should be
// landed alongside a comment naming the convention.
//
// Go-tooling conventions (`go build`/`go test` ignore these by
// convention; never contain compilable Go source the toolchain
// builds):
//   - testdata             — Go's own ignore convention; fixtures may
//     legitimately import testsupport for self-testing
//   - any name beginning with `_` (handled by shouldSkipDir, NOT this
//     map) — Go-tooling-ignored prefix
//
// Hidden infrastructure (handled by shouldSkipDir's `.` prefix check,
// NOT this map): .git, .gitnexus, .claude, .github.
//
// Ecosystem conventions (other languages' conventions; never contain
// Go source we own):
//   - vendor      — Go third-party packages (legitimately may contain
//     a testsupport import; we don't want to flag against ourselves)
//   - node_modules — JS deps
//
// Common build/output directory names. Heuristic: directories with
// these names rarely contain top-level production Go we want scanned.
// A hypothetical `out/cli/main.go` (legal but unusual) would be
// silently skipped — see TestPackageIsolation_FootgunDocumented for
// the documented gap. If a real project ships production Go in a
// directory matching one of these names, drop the entry from this set.
//   - tmp, dist, build, out, coverage
var skippedWalkDirs = map[string]struct{}{
	// Go-tooling conventions
	"testdata": {},
	// Ecosystem conventions
	"vendor":       {},
	"node_modules": {},
	// Common build/output dir names (footgun risk; see comment above)
	"tmp":      {},
	"dist":     {},
	"build":    {},
	"out":      {},
	"coverage": {},
}

// shouldSkipDir reports whether the walk should call SkipDir on the
// given directory name. The root entry ("." or the absolute path's
// basename) is never skipped.
func shouldSkipDir(name string) bool {
	if name == "" || name == "." {
		return false
	}
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	_, ok := skippedWalkDirs[name]
	return ok
}

// TestPackageNotImportedByProductionCode walks the repo and fails if a
// non-`_test.go` file imports testsupport. Tracks filesScanned so a
// regression that breaks the parser entirely (zero files inspected)
// fails LOUDLY instead of silently passing as "no offenders found."
func TestPackageNotImportedByProductionCode(t *testing.T) {
	root := RepoRoot(t)

	type offender struct {
		path    string
		lineNum int
	}
	var offenders []offender
	filesScanned := 0
	parseFailures := 0

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Surface walk errors via Logf so a regression in
			// directory-iteration is visible without aborting the
			// scan over the rest of the tree.
			t.Logf("walk error at %s: %v", path, walkErr)
			return nil
		}
		if d.IsDir() {
			if shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			// Track parse failures separately. We do NOT abort the
			// scan (the offender list should still print even when
			// one file is broken), but we DO mark the test failed
			// at the end — silent parse failures previously masked
			// regressions that broke parsing for "most files" while
			// leaving one parseable, satisfying the filesScanned > 0
			// guard while massively under-scanning.
			parseFailures++
			t.Logf("%s: parse failure: %v", path, err)
			return nil
		}
		filesScanned++
		for _, imp := range file.Imports {
			if strings.Trim(imp.Path.Value, `"`) == testsupportImportPath {
				offenders = append(offenders, offender{
					path:    path,
					lineNum: fset.Position(imp.Pos()).Line,
				})
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	// Coverage guard: a regression that broke parsing of every file
	// (e.g., a corrupt FileSet, or all files mis-ending with .go.bak)
	// would otherwise pass the assertion below with zero offenders.
	// We need to know we actually scanned production code.
	if filesScanned == 0 {
		t.Fatalf("scanned ZERO non-test .go files under %s — the walk has likely regressed; this test would silently pass without coverage", root)
	}

	// Surface ANY parse failures as a non-fatal error. The offender
	// scan still prints below; the test fails at the end so neither
	// signal masks the other.
	if parseFailures > 0 {
		t.Errorf("encountered %d parse failures during the production-import scan; see preceding logs for paths", parseFailures)
	}

	if len(offenders) > 0 {
		var lines []string
		for _, o := range offenders {
			rel, relErr := filepath.Rel(root, o.path)
			if relErr != nil {
				rel = o.path
			}
			lines = append(lines, "  "+rel+":"+strconv.Itoa(o.lineNum))
		}
		t.Fatalf("production code MUST NOT import %s — found %d offending file(s):\n%s\n\nIf the import is genuinely needed, move the helper out of testsupport into a non-test package (e.g., a new internal/<feature>util/ that does not advertise itself as test-only).",
			testsupportImportPath, len(offenders), strings.Join(lines, "\n"))
	}
}
