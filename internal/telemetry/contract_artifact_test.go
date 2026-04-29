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
//      on-disk call site is tagged with an ARTIFACT comment within
//      5 lines above. Catches "added a new on-disk file without tagging
//      it" before the symmetry check can drift. The scanner is
//      table-driven via onDiskRules, so adding a new write primitive
//      (e.g., os.Symlink, os.MkdirAll) is a one-line addition.
//
// Opt-out: a literal `// ARTIFACT: -` marker satisfies the within-window
// requirement WITHOUT contributing a name to the table-symmetry test. Use
// it for legitimately untagged literals (e.g., a debug-only file that
// is intentionally NOT a daemon-owned artifact). The opt-out is explicit
// — silent untagged calls still fail.
//
// Coverage guard: TestContract_AllOnDiskCallSitesAreTagged refuses to
// pass with zero call sites seen across all scanned files. Without this,
// a regression in classifyOnDiskCall that returned (label, false) for
// every call would silently turn the test into a no-op.

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

// onDiskUntaggedDiagFmt is the printf-style format string used by the
// untagged-call-site failure path. Centralized so both
// TestContract_AllOnDiskCallSitesAreTagged (which fires the diagnostic
// against real production source) and
// TestContract_OptOutGuidanceInFailureMessage (which asserts the
// diagnostic mentions the `-` opt-out marker) reference one source —
// a refactor that updated the prose in only one place would silently
// leave the other behind.
//
// (Both call sites are in this *_test.go file; there is no production
// scanner. The format string is shared between two tests, not between
// test and production.)
//
// Format args (in order): file, line, label, window.
const onDiskUntaggedDiagFmt = "%s:%d: %s names a kaboomDir-relative on-disk artifact but has no `// ARTIFACT: <name>` comment within %d lines above; tag the call site so TestContract_ArtifactTableMatchesCallSites cannot drift undetected (use `// ARTIFACT: -` to opt out if the literal is intentionally NOT a daemon-owned artifact)"

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

	prodFiles := artifactScannedFiles(t, repoRoot)
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
// Detected patterns: see onDiskRules below.
//
// Coverage guard: the test fails if classifyOnDiskCall recognized ZERO
// call sites across all scanned files. A regression that broke pattern
// matching would otherwise silently degrade this test to a no-op.
func TestContract_AllOnDiskCallSitesAreTagged(t *testing.T) {
	repoRoot := testsupport.RepoRoot(t)
	prodFiles := artifactScannedFiles(t, repoRoot)

	const tagWindow = 5 // max lines a tag may appear above the call site
	callSitesSeen := 0

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
			callSitesSeen++
			_, found := findArtifactTagWithin(artifactCommentLines, callLine, tagWindow)
			if !found {
				t.Errorf(onDiskUntaggedDiagFmt, filepath.Base(path), callLine, label, tagWindow)
			}
			// Whether the tag is a real name or `// ARTIFACT: -`,
			// the contract is satisfied for this call site.
			return true
		})
	}

	if callSitesSeen == 0 {
		t.Fatalf("classifyOnDiskCall recognized ZERO call sites across %d scanned files (%v) — the pattern matcher has likely regressed; this test would silently pass without coverage", len(prodFiles), prodFiles)
	}
}

// artifactScannedFiles is the canonical list of production files the
// ARTIFACT contract scans. The list is derived by globbing
// `internal/telemetry/install_id*.go` (and excluding `*_test.go`) so a
// future split that creates `install_id_secondary.go`, `install_id_io.go`,
// etc. is automatically covered.
func artifactScannedFiles(t *testing.T, repoRoot string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(repoRoot, "internal", "telemetry", "install_id*.go"))
	if err != nil {
		t.Fatalf("glob install_id*.go: %v", err)
	}
	out := make([]string, 0, len(matches))
	for _, p := range matches {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		t.Fatalf("artifactScannedFiles: glob returned no install_id*.go files under %s — repo layout has changed; update the glob",
			filepath.Join(repoRoot, "internal", "telemetry"))
	}
	return out
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

// onDiskRule describes a CallExpr shape that names a daemon-owned
// artifact at construction time. Each rule is matched against the
// callable's package + function name; litArg says which positional arg
// MUST be a string-literal artifact name; optional argIdents requires
// arg[idx] to be a bare *ast.Ident with the named value (e.g.,
// `0:"kaboomDir"` for the `filepath.Join(kaboomDir, "...")` shape).
//
// pkg == "" means "in-package bare ident" (e.g., withKaboomStateLock).
// pkg != "" means "selector qualified by this package name" (e.g.,
// "filepath", "os").
//
// All extension shapes today fit into this declarative struct — no
// rule needs an arbitrary closure. If a future shape genuinely
// requires custom predicate logic, add a typed field for that
// predicate (don't reach for func(call *CallExpr) bool generically;
// closures are escape-hatch flexibility this table has resisted).
//
// The argIdents map (rather than a `firstArgIdent string`) anticipates
// future rules that need ident constraints on more than the first arg
// — e.g., a hypothetical `wrapper(ctx, kaboomDir, "...")` rule would
// constrain arg[1].
type onDiskRule struct {
	pkg       string
	fn        string
	litArg    int
	label     string
	argIdents map[int]string // optional; arg[idx] must be a bare ident with the named value
}

// onDiskRules enumerates every call shape the producer-side scanner
// recognizes. Adding a new write primitive (e.g., os.Symlink, os.Link)
// is a one-line entry — no nested type-switch additions to
// classifyOnDiskCall.
//
// The set of os.* writers is "any standard library function that
// materializes a name on disk in its argument list." os.CreateTemp is
// intentionally NOT included: its second arg is a name PATTERN
// ("install_id.*"), not a final artifact name; the resulting file is
// renamed before becoming an artifact, and we tag the rename call site.
var onDiskRules = []onDiskRule{
	// filepath.Join(kaboomDir, "<lit>") — first arg must be the
	// kaboomDir package ident; second arg is the literal artifact name.
	{
		pkg: "filepath", fn: "Join", litArg: 1,
		label:     `filepath.Join(kaboomDir, "...")`,
		argIdents: map[int]string{0: "kaboomDir"},
	},
	// withKaboomStateLock("<lit>", ...) — bare in-package ident.
	{pkg: "", fn: "withKaboomStateLock", litArg: 0, label: `withKaboomStateLock("...", ...)`},
	// os.* writers — first arg is the artifact path.
	{pkg: "os", fn: "WriteFile", litArg: 0, label: `os.WriteFile("...", ...)`},
	{pkg: "os", fn: "Create", litArg: 0, label: `os.Create("...")`},
	{pkg: "os", fn: "OpenFile", litArg: 0, label: `os.OpenFile("...", ...)`},
	{pkg: "os", fn: "Mkdir", litArg: 0, label: `os.Mkdir("...", ...)`},
	{pkg: "os", fn: "MkdirAll", litArg: 0, label: `os.MkdirAll("...", ...)`},
	// os.Rename / os.Symlink / os.Link: destination (the artifact being
	// materialized) is the SECOND arg, not the first.
	{pkg: "os", fn: "Rename", litArg: 1, label: `os.Rename(..., "...")`},
	{pkg: "os", fn: "Symlink", litArg: 1, label: `os.Symlink(..., "...")`},
	{pkg: "os", fn: "Link", litArg: 1, label: `os.Link(..., "...")`},
}

// callableInfo extracts the package qualifier (or "" for a bare ident)
// and function name from a CallExpr's callee. Returns (name, pkg, true)
// on a recognized shape; (_, _, false) for shapes we don't classify
// (anonymous funcs, method calls on values, etc.).
func callableInfo(call *ast.CallExpr) (name, pkg string, ok bool) {
	switch fn := call.Fun.(type) {
	case *ast.SelectorExpr:
		if id, idOK := fn.X.(*ast.Ident); idOK {
			return fn.Sel.Name, id.Name, true
		}
	case *ast.Ident:
		return fn.Name, "", true
	}
	return "", "", false
}

// classifyOnDiskCall walks onDiskRules looking for a match. Returns a
// short human-readable label for the diagnostic and true if matched.
//
// Every shape requires a STRING-LITERAL argument at the rule's litArg
// index: that's the artifact name and the load-bearing condition for
// "this site materializes a specific on-disk file." A generic helper
// that takes a name parameter (e.g., withKaboomStateLock receiving its
// lockName var) is not a match — it's the wrapper layer, and its
// callers carry the literal.
func classifyOnDiskCall(call *ast.CallExpr) (string, bool) {
	name, pkg, ok := callableInfo(call)
	if !ok {
		return "", false
	}
	for _, r := range onDiskRules {
		if r.fn != name || r.pkg != pkg {
			continue
		}
		if r.litArg >= len(call.Args) {
			continue
		}
		if !isStringLit(call.Args[r.litArg]) {
			continue
		}
		// argIdents: every constrained position must be a bare ident
		// with the expected name. Out-of-range or non-ident args fail
		// the rule.
		identsOK := true
		for idx, want := range r.argIdents {
			if idx >= len(call.Args) {
				identsOK = false
				break
			}
			id, ok := call.Args[idx].(*ast.Ident)
			if !ok || id.Name != want {
				identsOK = false
				break
			}
		}
		if !identsOK {
			continue
		}
		return r.label, true
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

// TestContract_OptOutGuidanceInFailureMessage pins the producer-side
// scanner's failure-message contract: the diagnostic MUST mention both
// the `ARTIFACT:` tag scheme AND the `-` opt-out marker, so an author
// who trips the scanner sees the escape hatch in the same line as the
// failure (no need to read the test source).
//
// Implementation: we run a synthesized fake-source call site through the
// scanner via t.Run subtests with a captured *testing.T (a *testCapture
// fake) so we can assert against the recorded Errorf message without
// failing the surrounding *testing.T.
func TestContract_OptOutGuidanceInFailureMessage(t *testing.T) {
	// Synthetic source: an untagged kaboomDir-relative call that will
	// trip the scanner. The package name (`fake`), the kaboomDir
	// declaration, and the function body exist solely to satisfy
	// go/parser's syntactic requirements — they are NOT meant to
	// mirror real production shape (no Telemetry semantics, no
	// state-lock wrapper). If the scanner ever validates that
	// kaboomDir is declared by the same package as the production
	// telemetry pipeline, this fixture must add a matching package
	// declaration.
	const src = `package fake

import "path/filepath"

var kaboomDir = "/tmp"

func _() string {
	return filepath.Join(kaboomDir, "new_artifact")
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "fake.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write synthetic source: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse synthetic source: %v", err)
	}

	// Build the same diagnostic the real scanner builds, then assert
	// it mentions both the tag scheme and the opt-out marker.
	const tagWindow = 5
	artifactCommentLines := collectArtifactCommentLines(fset, file)
	var diagnostic string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		label, matched := classifyOnDiskCall(call)
		if !matched {
			return true
		}
		callLine := fset.Position(call.Lparen).Line
		if _, found := findArtifactTagWithin(artifactCommentLines, callLine, tagWindow); !found {
			diagnostic = fmt.Sprintf(onDiskUntaggedDiagFmt,
				filepath.Base(path), callLine, label, tagWindow)
		}
		return true
	})

	if diagnostic == "" {
		t.Fatal("synthetic scanner did not produce a diagnostic for the untagged kaboomDir call site — test fixture is broken or scanner regressed")
	}
	for _, want := range []string{"ARTIFACT:", "ARTIFACT: -", "opt out"} {
		if !strings.Contains(diagnostic, want) {
			t.Errorf("diagnostic missing %q guidance:\n%s", want, diagnostic)
		}
	}
}
