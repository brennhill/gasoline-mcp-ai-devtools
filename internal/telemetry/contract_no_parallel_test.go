// contract_no_parallel_test.go — Mechanical enforcement of the package
// doc.go constraint that no test in package telemetry calls
// t.Parallel(). Several files own package-level globals that the
// production pipeline mutates singly (kaboomDir, installIDLoadInFlight,
// cachedInstallIDPtr, secondaryDirOverride, userHomeDirFn, session,
// endpoint). The withXxxState helpers in helpers_test.go gate these
// with mutexes so a sequential test suite is safe; t.Parallel() would
// run cleanups concurrently and corrupt the global state.
//
// This contract test AST-walks every *_test.go file in this package
// and fails if any of them contain a call expression of the form
// `t.Parallel()` (or any receiver name with a `.Parallel()` selector).
// Subtests inside a single test (t.Run) are fine — the constraint is
// solely about parallel execution between sibling top-level tests.

package telemetry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// TestPackage_NoTParallelCalls is the load-bearing CI guard for the
// package-level no-t.Parallel constraint. AST-walks every *_test.go
// in this package's directory and Errorfs each `*.Parallel()` call
// with file:line so contributors can see exactly which test trips
// the constraint.
//
// We use AST (not regex) so a comment containing `t.Parallel()` does
// not false-positive, and a method call like `something.Parallel()`
// on an unrelated receiver still trips — the receiver name is not
// load-bearing, only the method name `Parallel` is. (No legitimate
// non-testing.T `Parallel()` method exists in this package today;
// if one is introduced later, this test is the natural place to add
// a receiver-type whitelist.)
func TestPackage_NoTParallelCalls(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate package directory")
	}
	pkgDir := filepath.Dir(thisFile)

	matches, err := filepath.Glob(filepath.Join(pkgDir, "*_test.go"))
	if err != nil {
		t.Fatalf("glob *_test.go: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("found ZERO *_test.go files under %s — the glob has regressed; test would silently pass without coverage", pkgDir)
	}

	type offender struct {
		path string
		line int
	}
	var offenders []offender

	for _, path := range matches {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if sel.Sel.Name != "Parallel" {
				return true
			}
			if len(call.Args) != 0 {
				return true
			}
			offenders = append(offenders, offender{
				path: path,
				line: fset.Position(call.Lparen).Line,
			})
			return true
		})
	}

	if len(offenders) > 0 {
		var lines []string
		for _, o := range offenders {
			lines = append(lines, "  "+filepath.Base(o.path)+":"+strconv.Itoa(o.line))
		}
		t.Errorf("package telemetry forbids t.Parallel() (see doc.go for rationale) — found %d call site(s):\n%s",
			len(offenders), strings.Join(lines, "\n"))
	}
}
