package main

import (
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// TestBridgeForwardRequest_NoCancelReassignment enforces a safety contract:
// the cancel func created with context.WithTimeout must not be reassigned.
// Reassignment after defer cancel() can leave a later timeout context uncanceled.
func TestBridgeForwardRequest_NoCancelReassignment(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bridge.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse bridge.go: %v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Name.Name == "bridgeForwardRequest" {
			fn = d
			break
		}
	}
	if fn == nil {
		t.Fatal("bridgeForwardRequest not found in bridge.go")
	}

	reassigned := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || assign.Tok != token.ASSIGN {
			return true
		}
		for _, lhs := range assign.Lhs {
			if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "cancel" {
				reassigned = true
				return false
			}
		}
		return true
	})

	if reassigned {
		t.Fatal("bridgeForwardRequest reassigns cancel; create a new cancel variable for retry context")
	}
}

func TestBridgeForwardRequest_NoRetryWithCtxCancelAssignmentPattern(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "bridge.go", nil, 0)
	if err != nil {
		t.Fatalf("failed to parse bridge.go: %v", err)
	}

	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if ok && d.Name.Name == "bridgeForwardRequest" {
			fn = d
			break
		}
	}
	if fn == nil {
		t.Fatal("bridgeForwardRequest not found in bridge.go")
	}

	for _, stmt := range fn.Body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}

		var b strings.Builder
		if err := format.Node(&b, fset, assign); err != nil {
			t.Fatalf("failed to render assignment: %v", err)
		}
		if strings.Contains(b.String(), "ctx, cancel = context.WithTimeout") {
			t.Fatalf("found fragile retry pattern %q; must use a fresh retry cancel variable", b.String())
		}
	}
}
