// package_isolation_test.go — Enforces the package doc's "Production
// code MUST NOT import this package" rule. The package ships
// non-`_test.go` files (faket.go, repo.go, astutil.go) because
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
	"strings"
	"testing"
)

// testsupportImportPath is the canonical Go import path of this package.
// Hardcoded here (instead of derived) because deriving from runtime info
// would couple the test to the host go.mod's module path, which is
// exactly what the test must NOT depend on (the test runs BEFORE
// trusting that path is what we think it is).
const testsupportImportPath = "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsupport"

// TestPackageNotImportedByProductionCode walks the repo and fails if a
// non-`_test.go` file imports testsupport. Skips vendor dirs, hidden
// dirs, and node_modules. The walk is a single os.WalkDir; no shell-out.
func TestPackageNotImportedByProductionCode(t *testing.T) {
	root := RepoRoot(t)

	type offender struct {
		path    string
		lineNum int
	}
	var offenders []offender

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Ignore individual entry errors (e.g., transient permission
			// blips); the walk continues. The aggregate result of the
			// scan is what the assertion below cares about.
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			// Skip hidden dirs (.git, .gitnexus, .claude), vendor, and
			// node_modules. Walking these wastes time and could
			// introduce false positives from third-party packages that
			// happen to import testsupport (impossible in practice,
			// but cheap to defend).
			if name != "." && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		// Production = .go file that is not a *_test.go file.
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

	if len(offenders) > 0 {
		var lines []string
		for _, o := range offenders {
			rel, relErr := filepath.Rel(root, o.path)
			if relErr != nil {
				rel = o.path
			}
			lines = append(lines, "  "+rel+":"+itoa(o.lineNum))
		}
		t.Fatalf("production code MUST NOT import %s — found %d offending file(s):\n%s\n\nIf the import is genuinely needed, move the helper out of testsupport into a non-test package (e.g., a new internal/<feature>util/ that does not advertise itself as test-only).",
			testsupportImportPath, len(offenders), strings.Join(lines, "\n"))
	}
}

// itoa is a small int→string helper so the contract test does not need
// to pull in strconv just for one call. Identical to strconv.Itoa for
// non-negative inputs (the only kind we produce — line numbers).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
