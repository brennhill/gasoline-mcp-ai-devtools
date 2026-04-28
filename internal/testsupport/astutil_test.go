// astutil_test.go — Direct unit tests for ImportQualifiers. Each table
// case feeds a synthetic Go source string through go/parser and asserts
// the ImportFacts result. Transitive coverage via the browser-agent
// contract test exists, but direct cases lock in every import-shape
// branch (named, plain, blank, dot, multiple dots) without depending on
// incidental imports of unrelated production files.
//
// The nil-vs-empty contract for DotImports is pinned in exactly one test
// (TestImportQualifiers_NilWhenNoDotImports). Other tests use
// `len(...) != 0` or value equality so a future micro-optimization that
// pre-allocates DotImports cascades through one test, not five.

package testsupport

import (
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"testing"
)

// parseImports compiles src as a Go file and returns its ImportFacts.
// Source must be a syntactically valid Go file (package decl + imports);
// the body is irrelevant to ImportQualifiers.
func parseImports(t *testing.T, src string) ImportFacts {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return ImportQualifiers(file)
}

func TestImportQualifiers_NoImports(t *testing.T) {
	got := parseImports(t, "package p\n")
	if len(got.Qualifiers) != 0 {
		t.Errorf("Qualifiers = %v, want empty map", got.Qualifiers)
	}
	if len(got.DotImports) != 0 {
		t.Errorf("DotImports = %v, want empty", got.DotImports)
	}
}

// TestImportQualifiers_NilWhenNoDotImports is the SINGLE test that pins
// the documented contract that DotImports is nil (not []string{}) when
// the file has no dot-imports. Other tests use len() to avoid coupling
// to this distinction; if the contract ever flips, only this test needs
// to change.
func TestImportQualifiers_NilWhenNoDotImports(t *testing.T) {
	got := parseImports(t, `package p
import "fmt"
`)
	if got.DotImports != nil {
		t.Errorf("DotImports = %v (len %d), want nil per documented contract", got.DotImports, len(got.DotImports))
	}
}

func TestImportQualifiers_PlainImport(t *testing.T) {
	got := parseImports(t, `package p
import "fmt"
`)
	if !got.Qualifiers["fmt"] {
		t.Errorf("Qualifiers = %v, want fmt present", got.Qualifiers)
	}
}

func TestImportQualifiers_PlainImport_TrailingSegment(t *testing.T) {
	got := parseImports(t, `package p
import "go/ast"
`)
	if !got.Qualifiers["ast"] {
		t.Errorf("Qualifiers = %v, want trailing segment 'ast' present", got.Qualifiers)
	}
	if got.Qualifiers["go"] {
		t.Errorf("Qualifiers = %v, must NOT contain non-trailing path segments", got.Qualifiers)
	}
}

func TestImportQualifiers_NamedImport(t *testing.T) {
	got := parseImports(t, `package p
import alias "go/ast"
`)
	if !got.Qualifiers["alias"] {
		t.Errorf("Qualifiers = %v, want named alias present", got.Qualifiers)
	}
	if got.Qualifiers["ast"] {
		t.Errorf("Qualifiers = %v, must NOT contain trailing segment when alias was provided", got.Qualifiers)
	}
}

func TestImportQualifiers_BlankImport_Skipped(t *testing.T) {
	got := parseImports(t, `package p
import _ "embed"
`)
	if len(got.Qualifiers) != 0 {
		t.Errorf("Qualifiers = %v, want empty (blank import skipped)", got.Qualifiers)
	}
	if len(got.DotImports) != 0 {
		t.Errorf("DotImports = %v, want empty (blank import is not a dot import)", got.DotImports)
	}
}

func TestImportQualifiers_DotImport(t *testing.T) {
	got := parseImports(t, `package p
import . "fmt"
`)
	if got.Qualifiers["fmt"] {
		t.Errorf("Qualifiers = %v, must NOT contain a qualifier for a dot-imported package", got.Qualifiers)
	}
	want := []string{"fmt"}
	if !reflect.DeepEqual(got.DotImports, want) {
		t.Errorf("DotImports = %v, want %v", got.DotImports, want)
	}
}

func TestImportQualifiers_MultipleDotImports_AllReturned(t *testing.T) {
	got := parseImports(t, `package p
import (
	. "fmt"
	. "errors"
)
`)
	if len(got.Qualifiers) != 0 {
		t.Errorf("Qualifiers = %v, want empty (both dot imports skipped)", got.Qualifiers)
	}
	// Order matches source order — preserve so callers' error messages
	// list offenders deterministically.
	want := []string{"fmt", "errors"}
	if !reflect.DeepEqual(got.DotImports, want) {
		t.Errorf("DotImports = %v, want %v (in source order)", got.DotImports, want)
	}
}

func TestImportQualifiers_MixedShapes(t *testing.T) {
	got := parseImports(t, `package p
import (
	"fmt"
	alias "go/ast"
	_ "embed"
	. "errors"
)
`)
	gotKeys := make([]string, 0, len(got.Qualifiers))
	for k := range got.Qualifiers {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	wantKeys := []string{"alias", "fmt"}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("qualifier keys = %v, want %v", gotKeys, wantKeys)
	}
	if !reflect.DeepEqual(got.DotImports, []string{"errors"}) {
		t.Errorf("DotImports = %v, want [errors]", got.DotImports)
	}
}
