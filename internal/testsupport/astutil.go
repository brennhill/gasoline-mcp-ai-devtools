// astutil.go — AST helpers shared across contract tests that walk Go
// source. Currently scoped to import-qualifier extraction; expand only
// when a second test package needs the same primitive (the `2+ callers`
// bar described in the package doc).

package testsupport

import (
	"go/ast"
	"strings"
)

// ImportQualifiers returns the set of identifier qualifiers usable as the
// left-hand side of a SelectorExpr in `file`, plus the import path of any
// dot-import encountered. The boolean second return is true iff the file
// contains at least one dot-import.
//
// Built-in semantics:
//   - Named import (`foo "x/y"`) → qualifier "foo".
//   - Plain import (`"x/y/baz"`) → qualifier "baz" (trailing path segment).
//   - Blank import (`_ "x/y"`)   → skipped (no qualifier).
//   - Dot import (`. "x/y"`)     → SKIPPED in the map AND the dot-import
//     flag is set; the caller MUST handle this (typically t.Fatalf) because
//     dot-imports defeat selector-based whitelisting: identifiers from the
//     dot-imported package appear bare (as *ast.Ident), bypassing any
//     SelectorExpr-based check.
//
// The returned map never contains "" or "_". The caller is responsible for
// adding "main" or any other in-package qualifier — this helper deals
// solely with the file's imports.
func ImportQualifiers(file *ast.File) (qualifiers map[string]bool, dotImport string) {
	qualifiers = make(map[string]bool, len(file.Imports))
	for _, imp := range file.Imports {
		// Strip the surrounding quotes from the import path literal.
		path := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			switch imp.Name.Name {
			case ".":
				// Caller-visible signal: at least one dot-import is
				// present. We return the first one found so the
				// caller's error message can name it; this matches
				// the "fail fast at first offender" UX of the
				// existing telemetry contract tests.
				if dotImport == "" {
					dotImport = path
				}
				continue
			case "_":
				continue
			default:
				qualifiers[imp.Name.Name] = true
				continue
			}
		}
		// Default qualifier: trailing path segment.
		var qual string
		if i := strings.LastIndex(path, "/"); i >= 0 {
			qual = path[i+1:]
		} else {
			qual = path
		}
		if qual != "" {
			qualifiers[qual] = true
		}
	}
	return qualifiers, dotImport
}
