// astutil_test.go — Direct unit tests for ImportQualifiers. Each table
// case feeds a synthetic Go source string through go/parser and asserts
// the (qualifiers, dotImports) tuple. Transitive coverage via the
// browser-agent contract test exists, but direct cases lock in every
// import-shape branch (named, plain, blank, dot, multiple dots) without
// depending on incidental imports of unrelated production files.

package testsupport

import (
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"testing"
)

// parseImports compiles src as a Go file and returns its *ast.File.
// Source must be a syntactically valid Go file (package decl + imports);
// the body is irrelevant to ImportQualifiers.
func parseImports(t *testing.T, src string) (qualifiers map[string]bool, dotImports []string) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return ImportQualifiers(file)
}

func TestImportQualifiers_NoImports(t *testing.T) {
	q, d := parseImports(t, "package p\n")
	if len(q) != 0 {
		t.Errorf("qualifiers = %v, want empty map", q)
	}
	if d != nil {
		t.Errorf("dotImports = %v, want nil", d)
	}
}

func TestImportQualifiers_PlainImport(t *testing.T) {
	q, d := parseImports(t, `package p
import "fmt"
`)
	if !q["fmt"] {
		t.Errorf("qualifiers = %v, want fmt present", q)
	}
	if d != nil {
		t.Errorf("dotImports = %v, want nil", d)
	}
}

func TestImportQualifiers_PlainImport_TrailingSegment(t *testing.T) {
	q, _ := parseImports(t, `package p
import "go/ast"
`)
	if !q["ast"] {
		t.Errorf("qualifiers = %v, want trailing segment 'ast' present", q)
	}
	if q["go"] {
		t.Errorf("qualifiers = %v, must NOT contain non-trailing path segments", q)
	}
}

func TestImportQualifiers_NamedImport(t *testing.T) {
	q, _ := parseImports(t, `package p
import alias "go/ast"
`)
	if !q["alias"] {
		t.Errorf("qualifiers = %v, want named alias present", q)
	}
	if q["ast"] {
		t.Errorf("qualifiers = %v, must NOT contain trailing segment when alias was provided", q)
	}
}

func TestImportQualifiers_BlankImport_Skipped(t *testing.T) {
	q, d := parseImports(t, `package p
import _ "embed"
`)
	if len(q) != 0 {
		t.Errorf("qualifiers = %v, want empty (blank import skipped)", q)
	}
	if d != nil {
		t.Errorf("dotImports = %v, want nil (blank import is not a dot import)", d)
	}
}

func TestImportQualifiers_DotImport(t *testing.T) {
	q, d := parseImports(t, `package p
import . "fmt"
`)
	if q["fmt"] {
		t.Errorf("qualifiers = %v, must NOT contain a qualifier for a dot-imported package", q)
	}
	want := []string{"fmt"}
	if !reflect.DeepEqual(d, want) {
		t.Errorf("dotImports = %v, want %v", d, want)
	}
}

func TestImportQualifiers_MultipleDotImports_AllReturned(t *testing.T) {
	q, d := parseImports(t, `package p
import (
	. "fmt"
	. "errors"
)
`)
	if len(q) != 0 {
		t.Errorf("qualifiers = %v, want empty (both dot imports skipped)", q)
	}
	// Order matches source order — preserve so callers' error messages
	// list offenders deterministically.
	want := []string{"fmt", "errors"}
	if !reflect.DeepEqual(d, want) {
		t.Errorf("dotImports = %v, want %v (in source order)", d, want)
	}
}

func TestImportQualifiers_MixedShapes(t *testing.T) {
	q, d := parseImports(t, `package p
import (
	"fmt"
	alias "go/ast"
	_ "embed"
	. "errors"
)
`)
	gotKeys := make([]string, 0, len(q))
	for k := range q {
		gotKeys = append(gotKeys, k)
	}
	sort.Strings(gotKeys)
	wantKeys := []string{"alias", "fmt"}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("qualifier keys = %v, want %v", gotKeys, wantKeys)
	}
	if !reflect.DeepEqual(d, []string{"errors"}) {
		t.Errorf("dotImports = %v, want [errors]", d)
	}
}
