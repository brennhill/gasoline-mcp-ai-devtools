// install_id_drift_log_test.go — Pins the lifecycle-log field-shape contract
// and the registration wiring for the install_id_drift event. Three tests
// together pin the end-to-end path:
//   1. TestNewInstallIDDriftLogger_LogShape exercises the real wiring fn
//      (no copy-paste) and confirms (stored, derived) → lifecycle map keys.
//   2. TestWireInstallIDDriftLogger_RegistersThroughTelemetry calls the same
//      registration helper runMCPMode invokes, asserting it leaves a non-nil
//      callback at the telemetry public API.
//   3. TestRunMCPMode_CallsWireInstallIDDriftLogger source-greps
//      main_connection_mcp.go to enforce that runMCPMode actually invokes
//      wireInstallIDDriftLogger — catches a refactor that silently drops
//      the registration call.
//
// The complementary contract — that telemetry.CheckInstallIDDrift actually
// invokes the registered callback when stored != derived — lives in
// internal/telemetry/install_id_test.go::TestCheckInstallIDDrift_FiresWhenDerivedChanges.

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/telemetry"
)

func TestNewInstallIDDriftLogger_LogShape(t *testing.T) {
	srv := newTestServerForHandlers(t)
	const port = 7890
	const stored = "111111111111"
	const derived = "222222222222"

	fn := newInstallIDDriftLogger(srv, port)
	fn(stored, derived)

	entries := srv.logs.getEntries()
	var found map[string]any
	for _, e := range entries {
		if e["event"] == "install_id_drift" {
			found = e
			break
		}
	}
	if found == nil {
		t.Fatal("install_id_drift lifecycle event missing from logs")
	}
	if got := found["stored_iid"]; got != stored {
		t.Errorf("stored_iid = %v, want %q", got, stored)
	}
	if got := found["derived_iid"]; got != derived {
		t.Errorf("derived_iid = %v, want %q", got, derived)
	}
	if got := found["type"]; got != "lifecycle" {
		t.Errorf("type = %v, want lifecycle", got)
	}
	if got := found["port"]; got != port {
		t.Errorf("port = %v, want %d", got, port)
	}
}

// TestWireInstallIDDriftLogger_RegistersThroughTelemetry confirms that the
// single helper runMCPMode calls (wireInstallIDDriftLogger) leaves a non-nil
// callback registered with telemetry. Combined with the source-grep test
// below, this pins both the helper's contract AND the call site.
func TestWireInstallIDDriftLogger_RegistersThroughTelemetry(t *testing.T) {
	srv := newTestServerForHandlers(t)
	t.Cleanup(func() { telemetry.SetInstallIDDriftLogFn(nil) })

	wireInstallIDDriftLogger(srv, 7892)

	if !telemetry.HasInstallIDDriftLogFnForTest() {
		t.Fatal("wireInstallIDDriftLogger did not register a callback through the public API")
	}
}

// TestRunMCPMode_CallsWireInstallIDDriftLogger uses go/parser + go/ast to
// confirm runMCPMode's body contains a CallExpr to wireInstallIDDriftLogger,
// without pinning the local variable names of its arguments. A refactor that
// renames `server`→`srv` is benign and must not break this test; one that
// drops the call entirely fails fast.
func TestRunMCPMode_CallsWireInstallIDDriftLogger(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed — cannot locate source for AST walk")
	}
	srcPath := filepath.Join(filepath.Dir(thisFile), "main_connection_mcp.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, srcPath, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", srcPath, err)
	}

	var runMCPMode *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "runMCPMode" && fn.Recv == nil {
			runMCPMode = fn
			break
		}
	}
	if runMCPMode == nil {
		t.Fatal("runMCPMode function not found in main_connection_mcp.go")
	}

	found := false
	ast.Inspect(runMCPMode.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		if ident.Name == "wireInstallIDDriftLogger" {
			found = true
			return false
		}
		return true
	})
	if !found {
		t.Fatal("runMCPMode body must contain a call to wireInstallIDDriftLogger so install_id_drift surfaces in lifecycle logs")
	}
}
