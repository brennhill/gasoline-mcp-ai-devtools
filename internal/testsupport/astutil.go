// astutil.go — AST helpers shared across contract tests that walk Go
// source. Currently scoped to import-qualifier extraction; expand only
// when a second test package needs the same primitive (the `2+ callers`
// bar described in the package doc).

package testsupport

import (
	"go/ast"
	"strings"
)

// ImportFacts is the structured return of ImportQualifiers. It groups
// the qualifier lookup set with the dot-import path list so callers can
// access each independently. The earlier tuple return ((map, []string))
// presented "two shapes for the same conceptual data" awkwardly; this
// struct names the relationship.
type ImportFacts struct {
	// Qualifiers is the set of identifiers usable as the left-hand side
	// of a SelectorExpr in the file. Membership is the only operation —
	// callers should use map[k] without iterating.
	Qualifiers map[string]bool

	// DotImports is every dot-import path encountered, in source order.
	// Nil (NOT empty slice) when the file has no dot-imports — the doc
	// commits to nil so a single test (TestImportQualifiers_NilWhenNoDot)
	// pins the contract; other callers should use `len(DotImports) > 0`
	// rather than coupling to nil-vs-empty.
	DotImports []string
}

// ImportQualifiers returns ImportFacts describing the file's imports.
//
// Built-in semantics:
//   - Named import (`foo "x/y"`) → qualifier "foo".
//   - Plain import (`"x/y/baz"`) → qualifier "baz" (trailing path segment).
//   - Blank import (`_ "x/y"`)   → skipped (no qualifier).
//   - Dot import (`. "x/y"`)     → SKIPPED in Qualifiers AND appended to
//     DotImports; the caller MUST handle this (typically t.Fatalf) because
//     dot-imports defeat selector-based whitelisting: identifiers from the
//     dot-imported package appear bare (as *ast.Ident), bypassing any
//     SelectorExpr-based check.
//
// Qualifiers never contains "" or "_". The caller is responsible for
// adding "main" or any other in-package qualifier — this helper deals
// solely with the file's imports.
//
// Future API direction: if a future caller needs all imports including
// blanks, source order, or per-import metadata (path + qualifier + kind),
// migrate to a `[]ImportInfo` return where each entry carries
// (Path, Qualifier, Kind). The current shape is over-fit to "build a
// SelectorExpr whitelist + fail on dot-imports" — the only consumer
// today.
func ImportQualifiers(file *ast.File) ImportFacts {
	out := ImportFacts{Qualifiers: make(map[string]bool, len(file.Imports))}
	for _, imp := range file.Imports {
		// Strip the surrounding quotes from the import path literal.
		path := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			switch imp.Name.Name {
			case ".":
				// Caller-visible signal: every dot-import path is
				// surfaced so the caller's error message can name
				// all offenders. Order matches source order.
				out.DotImports = append(out.DotImports, path)
				continue
			case "_":
				continue
			default:
				out.Qualifiers[imp.Name.Name] = true
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
			out.Qualifiers[qual] = true
		}
	}
	return out
}
