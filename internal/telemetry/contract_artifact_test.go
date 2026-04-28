// contract_artifact_test.go — Source-level contract tests for the
// `// ARTIFACT: <name>` magic-comment scheme that pins on-disk
// daemon-owned files to docs.
//
// Two complementary tests guard the scheme:
//   1. TestContract_ArtifactTableMatchesCallSites — every entry in the
//      docs table has a matching ARTIFACT tag in the production source,
//      and every ARTIFACT tag's name appears in the docs table. Catches
//      "added a row but never tagged code" (and vice versa).
//   2. TestContract_AllOnDiskCallSitesAreTagged — every kaboomDir-relative
//      on-disk call site (filepath.Join, withKaboomStateLock, os.WriteFile/
//      Create/OpenFile/Rename with a literal arg) is tagged with an
//      ARTIFACT comment within 5 lines above. Catches "added a new on-disk
//      file without tagging it" before the symmetry check can drift.
//
// Opt-out: a literal `// ARTIFACT: -` marker satisfies the within-window
// requirement WITHOUT contributing a name to the table-symmetry test. Use
// it for legitimately untagged literals (e.g., a debug-only file that
// is intentionally NOT a daemon-owned artifact). The opt-out is explicit
// — silent untagged calls still fail.

package telemetry

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/testsupport"
)

// artifactOptOut is the magic name that satisfies the producer-side
// scanner without contributing to the table-symmetry test. Centralized as
// a constant so future renames can't drift between the two tests below.
const artifactOptOut = "-"

// TestContract_ArtifactTableMatchesCallSites scans production telemetry
// source for `// ARTIFACT: <name>` magic comments next to each on-disk
// daemon-owned file call site and asserts each appears in the
// docs/core/app-metrics.md "Daemon-owned on-disk artifacts" table.
//
// The magic-comment scheme decouples the test from the call-site shape:
// renames of `kaboomDir`, `withKaboomStateLock`, or any helper do not
// silently disable the symmetry check. Adding a new on-disk file just
// requires (1) tagging its call site with `// ARTIFACT: <name>` and
// (2) adding a doc table row.
//
// Producer rules: each artifact must be tagged at least once across the
// production sources scanned below; the same artifact may legitimately
// appear at multiple call sites (read + write) and that's fine.
//
// Opt-out tags (`// ARTIFACT: -`) are intentionally excluded from the
// symmetry check — they mark a literal that the producer-side scanner
// sees but that is NOT a daemon-owned artifact (and therefore has no
// docs row to symmetry-check against).
func TestContract_ArtifactTableMatchesCallSites(t *testing.T) {
	repoRoot := testsupport.RepoRoot(t)
	docPath := filepath.Join(repoRoot, "docs", "core", "app-metrics.md")
	docBody, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("read %s: %v", docPath, err)
	}

	prodFiles := artifactScannedFiles(repoRoot)
	var prodSrc strings.Builder
	for _, p := range prodFiles {
		body, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		prodSrc.Write(body)
		prodSrc.WriteByte('\n')
	}
	src := prodSrc.String()

	// Extract artifact names tagged at production call sites. The opt-out
	// marker is filtered out — it's a producer-side scanner signal, not a
	// docs-table candidate.
	artifactPat := regexp.MustCompile(`(?m)^\s*//\s*ARTIFACT:\s*(\S+)\s*$`)
	codeArtifacts := make(map[string]bool)
	for _, m := range artifactPat.FindAllStringSubmatch(src, -1) {
		if m[1] == artifactOptOut {
			continue
		}
		codeArtifacts[m[1]] = true
	}

	// Set of artifacts expected to be both code-tagged AND doc-rowed.
	// Adding a new on-disk file requires updating ALL THREE: production
	// code (with ARTIFACT tag), this set, and the doc table.
	expected := []string{
		"install_id",
		"install_id.bak",
		"install_id.lock",
		"install_id_lineage",
		"first_tool_call_install_id",
		"first_tool_call_install_id.lock",
	}
	const docSection = "Daemon-owned on-disk artifacts"

	for _, name := range expected {
		if !codeArtifacts[name] {
			t.Errorf("expected artifact %q has no `// ARTIFACT: %s` tag in any of the searched production files: %v — stale expected list, or missing tag at the call site?", name, name, prodFiles)
		}
		needle := "`" + name + "`"
		if !strings.Contains(string(docBody), needle) {
			t.Errorf("expected artifact %q missing doc pin %s — add a row to the %q table at %s", name, needle, docSection, docPath)
		}
	}

	// Direct symmetry: anything tagged with ARTIFACT in code MUST be in
	// expected (catches a new code artifact that landed without doc/test
	// updates).
	expectedSet := make(map[string]bool, len(expected))
	for _, name := range expected {
		expectedSet[name] = true
	}
	for name := range codeArtifacts {
		if !expectedSet[name] {
			t.Errorf("production source tags on-disk artifact %q via // ARTIFACT comment but it is NOT in the test's expected list (and likely not in the %q table either); add doc + expected list update", name, docSection)
		}
	}
}

// TestContract_AllOnDiskCallSitesAreTagged is the producer-side companion
// to TestContract_ArtifactTableMatchesCallSites. The earlier test asserts
// "every name in the expected list has at least one ARTIFACT tag in the
// production source"; this one asserts the inverse, more important
// invariant: "every place that names a kaboomDir-relative on-disk path is
// tagged with an ARTIFACT comment within 5 lines above."
//
// Without this scanner, an author who adds a new on-disk file via
// `filepath.Join(kaboomDir, "new_file")` or `withKaboomStateLock("x.lock",
// ...)` could ship the change tag-less; the symmetry check would still
// pass for ALL EXISTING tagged artifacts and silently miss the new one.
//
// Detected patterns:
//   - filepath.Join(kaboomDir, "<literal>")        — path construction
//   - withKaboomStateLock("<literal>", ...)        — daemon-owned file lock
//   - os.WriteFile/Create/OpenFile/Rename with a literal first arg —
//     a path literal that bypasses the wrapper layer (e.g., someone
//     inlines an absolute path instead of building it via filepath.Join).
//
// All three shapes carry a string literal naming the artifact, which is
// why the ARTIFACT tag MUST appear within 5 lines above (so reviewers can
// match name-to-tag without scrolling).
//
// Why we don't scan os.Stat / os.ReadFile: those are READ-side operations
// and don't materialize a new on-disk file. The contract is about which
// files the daemon CREATES, which is what the doc table enumerates.
func TestContract_AllOnDiskCallSitesAreTagged(t *testing.T) {
	repoRoot := testsupport.RepoRoot(t)
	prodFiles := artifactScannedFiles(repoRoot)

	const tagWindow = 5 // max lines a tag may appear above the call site

	for _, path := range prodFiles {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		artifactCommentLines := collectArtifactCommentLines(fset, file)

		// Walk all call expressions and check the on-disk shapes.
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			callLine := fset.Position(call.Lparen).Line
			label, isOnDiskCall := classifyOnDiskCall(call)
			if !isOnDiskCall {
				return true
			}
			tag, found := findArtifactTagWithin(artifactCommentLines, callLine, tagWindow)
			if !found {
				t.Errorf("%s:%d: %s names a kaboomDir-relative on-disk artifact but has no `// ARTIFACT: <name>` comment within %d lines above; tag the call site so TestContract_ArtifactTableMatchesCallSites cannot drift undetected (use `// ARTIFACT: -` to opt out if the literal is intentionally NOT a daemon-owned artifact)",
					filepath.Base(path), callLine, label, tagWindow)
				return true
			}
			// `tag == artifactOptOut` is the explicit opt-out — pass.
			// Any other non-empty name is a real tag — pass.
			_ = tag
			return true
		})
	}
}

// artifactScannedFiles is the canonical list of production files the
// ARTIFACT contract scans. Centralizing it ensures both tests stay in
// sync; adding a new file with on-disk artifacts just requires updating
// this single function.
func artifactScannedFiles(repoRoot string) []string {
	return []string{
		filepath.Join(repoRoot, "internal", "telemetry", "install_id.go"),
		filepath.Join(repoRoot, "internal", "telemetry", "install_id_drift.go"),
	}
}

// collectArtifactCommentLines returns a map of line-number → artifact name
// for every `// ARTIFACT: <name>` comment in `file`. The map's value
// preserves the exact name (including the `-` opt-out marker) so callers
// can distinguish "real tag" from "opt-out" if needed.
func collectArtifactCommentLines(fset *token.FileSet, file *ast.File) map[int]string {
	out := make(map[int]string)
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if !strings.HasPrefix(c.Text, "//") {
				continue
			}
			body := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			rest, ok := strings.CutPrefix(body, "ARTIFACT:")
			if !ok {
				continue
			}
			name := strings.TrimSpace(rest)
			if name == "" {
				continue // malformed `// ARTIFACT:` with no name — ignore
			}
			line := fset.Position(c.Slash).Line
			out[line] = name
		}
	}
	return out
}

// classifyOnDiskCall recognizes the AST shapes that name a
// kaboomDir-relative artifact at construction time. Returns a short
// human-readable label for the diagnostic and true if matched.
//
// All shapes require a STRING-LITERAL argument: that's the artifact name
// and the load-bearing condition for "this site materializes a specific
// on-disk file." A generic helper that takes a name parameter (e.g.,
// withKaboomStateLock receiving its lockName var) is not a match — it's
// the wrapper layer, and its callers carry the literal.
func classifyOnDiskCall(call *ast.CallExpr) (string, bool) {
	// Pattern A: filepath.Join(kaboomDir, "<literal>")
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "filepath" && sel.Sel.Name == "Join" {
			if len(call.Args) >= 2 {
				if id, ok := call.Args[0].(*ast.Ident); ok && id.Name == "kaboomDir" {
					if isStringLit(call.Args[1]) {
						return "filepath.Join(kaboomDir, \"...\")", true
					}
				}
			}
		}
		// Pattern C: os.WriteFile / os.Create / os.OpenFile / os.Rename
		// with a literal first arg. These calls materialize a file at a
		// path the source spells out directly — bypassing filepath.Join
		// + kaboomDir won't smuggle a new artifact past this scanner.
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" {
			switch sel.Sel.Name {
			case "WriteFile", "Create", "OpenFile", "Rename":
				if isStringLit(call.Args[0]) {
					return fmt.Sprintf("os.%s(\"...\", ...)", sel.Sel.Name), true
				}
			}
		}
	}
	// Pattern B: withKaboomStateLock("<literal>", ...)
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "withKaboomStateLock" {
		if isStringLit(call.Args[0]) {
			return "withKaboomStateLock(\"...\", ...)", true
		}
	}
	return "", false
}

// isStringLit reports whether expr is a string-typed BasicLit. Concatenated
// expressions (e.g., `name + ".bak"`) are intentionally NOT a match — those
// are derivative constructions; the canonical artifact name lives at the
// originating literal site.
func isStringLit(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

// findArtifactTagWithin returns the tag name and true iff any line in
// [callLine-window, callLine-1] carries an `// ARTIFACT: <name>` comment.
// The strict <-window-and-above rule prevents a tag from "below" or
// on-the-same-line from satisfying the contract; the comment must precede
// the call so reviewers see it first.
//
// When multiple tags fall within the window, the closest one (largest
// line number) wins — matches reviewer expectation that the tag
// immediately above the call is the one that names it.
func findArtifactTagWithin(tagLines map[int]string, callLine, window int) (string, bool) {
	for ln := callLine - 1; ln >= callLine-window; ln-- {
		if name, ok := tagLines[ln]; ok {
			return name, true
		}
	}
	return "", false
}

// TestFindArtifactTagWithin_BoundaryEdges pins the half-open
// [callLine-window, callLine-1] range. Boundaries matter: a tag at
// callLine-window must pass; one at callLine-(window+1) must fail; a tag
// on the same line as the call (or below) must NOT count, since the
// scheme requires the comment to precede the call so reviewers see it
// first.
func TestFindArtifactTagWithin_BoundaryEdges(t *testing.T) {
	cases := []struct {
		name      string
		tagLines  map[int]string
		callLine  int
		window    int
		wantName  string
		wantFound bool
	}{
		{
			name:      "tag immediately above (line-1)",
			tagLines:  map[int]string{99: "x"},
			callLine:  100,
			window:    5,
			wantName:  "x",
			wantFound: true,
		},
		{
			name:      "tag at exact window edge (line-window)",
			tagLines:  map[int]string{95: "x"},
			callLine:  100,
			window:    5,
			wantName:  "x",
			wantFound: true,
		},
		{
			name:      "tag just past window edge (line-window-1)",
			tagLines:  map[int]string{94: "x"},
			callLine:  100,
			window:    5,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "tag on same line as call",
			tagLines:  map[int]string{100: "x"},
			callLine:  100,
			window:    5,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "tag below call",
			tagLines:  map[int]string{101: "x"},
			callLine:  100,
			window:    5,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "no tags at all",
			tagLines:  map[int]string{},
			callLine:  100,
			window:    5,
			wantName:  "",
			wantFound: false,
		},
		{
			name:      "opt-out marker recognized as found",
			tagLines:  map[int]string{99: artifactOptOut},
			callLine:  100,
			window:    5,
			wantName:  artifactOptOut,
			wantFound: true,
		},
		{
			name:      "closest tag wins when multiple in window",
			tagLines:  map[int]string{95: "far", 99: "near"},
			callLine:  100,
			window:    5,
			wantName:  "near",
			wantFound: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotFound := findArtifactTagWithin(tc.tagLines, tc.callLine, tc.window)
			if gotFound != tc.wantFound {
				t.Errorf("found = %v, want %v", gotFound, tc.wantFound)
			}
			if gotName != tc.wantName {
				t.Errorf("name = %q, want %q", gotName, tc.wantName)
			}
		})
	}
}
