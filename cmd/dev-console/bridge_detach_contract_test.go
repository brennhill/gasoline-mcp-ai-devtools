package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestBridgeSpawnPathsSetDetachedProcess enforces that the daemon command builder
// always detaches child processes from the caller session.
// Both spawnDaemonAsync and respawnIfNeeded delegate to buildDaemonCmd,
// so we verify that buildDaemonCmd calls util.SetDetachedProcess.
func TestBridgeSpawnPathsSetDetachedProcess(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bridge.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse bridge.go: %v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Name.Name == "buildDaemonCmd" {
			fn = d
			break
		}
	}
	if fn == nil {
		t.Fatal("buildDaemonCmd not found in bridge.go")
	}

	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkgIdent, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if pkgIdent.Name == "util" && sel.Sel.Name == "SetDetachedProcess" {
			found = true
			return false
		}
		return true
	})

	if !found {
		t.Fatal("buildDaemonCmd must call util.SetDetachedProcess(cmd) before returning")
	}
}
