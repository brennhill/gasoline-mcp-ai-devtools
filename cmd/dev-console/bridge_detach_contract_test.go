package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// TestBridgeSpawnPathsSetDetachedProcess enforces that bridge daemon spawn paths
// always detach child processes from the caller session.
func TestBridgeSpawnPathsSetDetachedProcess(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bridge.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse bridge.go: %v", err)
	}

	required := map[string]bool{
		"spawnDaemonAsync": false,
		"respawnIfNeeded":  false,
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if _, tracked := required[fn.Name.Name]; !tracked {
			continue
		}

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
				required[fn.Name.Name] = true
				return false
			}
			return true
		})
	}

	for fnName, found := range required {
		if !found {
			t.Fatalf("%s must call util.SetDetachedProcess(cmd) before cmd.Start()", fnName)
		}
	}
}
