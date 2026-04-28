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
// scan must not descend into. Listed by name (no path prefix) so the
// match is fast and order-independent. Entries:
//
//   - hidden dirs (.git, .gitnexus, .claude): infrastructure, never
//     contain Go source we own
//   - vendor: third-party packages — could legitimately contain a
//     testsupport import we don't want to flag against ourselves
//   - node_modules: JS deps, no Go source
//   - testdata: Go's own ignore convention; fixtures live here and may
//     legitimately import testsupport for self-testing
//   - tmp, dist, build, out, coverage: build/output dirs that may
//     contain copied or generated source
//
// Any directory beginning with "_" is also skipped — Go's tooling
// (`go build`, `go test`) ignores `_`-prefixed dirs by convention.
var skippedWalkDirs = map[string]struct{}{
	"vendor":       {},
	"node_modules": {},
	"testdata":     {},
	"tmp":          {},
	"dist":         {},
	"build":        {},
	"out":          {},
	"coverage":     {},
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
			// A parse failure on a production .go file is a separate
			// problem; surface it but do not block the scan, so the
			// final error message lists every offender even when one
			// file is broken.
			t.Logf("%s: parse failure (treated as no-import): %v", path, err)
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
