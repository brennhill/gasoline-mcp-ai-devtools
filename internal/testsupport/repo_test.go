// repo_test.go — Self-tests for testsupport.RepoRoot. Verify the
// happy-path walk-up returns the repo root containing go.mod, the
// failure mode fires Fatalf with the RepoRootChdirHint marker when no
// go.mod is reachable, the foreign-module skip branch (the whole
// reason ExpectedModulePath exists) actually walks past a non-matching
// go.mod, and the parser handles each go.mod shape that the wild has
// produced.

package testsupport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepoRoot_FindsRepoRootFromPackageDir runs from this package directory
// (the test runner sets cwd to the package's source dir by default). It
// must walk up to the repo root containing the canonical go.mod.
func TestRepoRoot_FindsRepoRootFromPackageDir(t *testing.T) {
	root := RepoRoot(t)
	// The returned path must contain a go.mod.
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("RepoRoot returned %q but no go.mod is present: %v", root, err)
	}
	// And the go.mod must declare the expected module path. Soft check
	// via substring to avoid coupling to formatting nuances.
	body, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.Contains(string(body), ExpectedModulePath) {
		t.Errorf("go.mod at %s does not declare module %q (full body: %q)", root, ExpectedModulePath, body)
	}
}

// TestRepoRoot_FatalfWhenNoGoMod exercises the not-found branch via the
// wd-injectable internal form, so the test does NOT depend on whether
// $TMPDIR happens to sit inside a Go module on the host machine. We pass
// a synthetic path under TempDir() and verify the walk reaches filesystem
// root with the RepoRootChdirHint marker present.
//
// We still GUARD against a host where TempDir() roots inside a Go module
// (rare but possible on contributor machines) — but the guard now skips
// loudly with t.Logf, so it is visible in `-v` runs whether the branch
// was actually exercised. (Filesystem stat is not currently injectable;
// see the LIMITATION in repoRootFromWd's doc.)
func TestRepoRoot_FatalfWhenNoGoMod(t *testing.T) {
	tmp := t.TempDir()
	for d := tmp; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			t.Skipf("ancestor %q contains go.mod; cannot exercise the not-found branch on this filesystem", d)
		}
	}
	t.Logf("exercising not-found branch from synthetic wd %q", tmp)

	fake := &FakeT{}
	ExpectFakeFatal(t, fake, func() {
		repoRootFromWd(fake, tmp)
	})
	if fake.Fatal() == "" {
		t.Fatal("Fatalf was not called from a directory with no go.mod ancestor")
	}
	// The remediation hint marker is load-bearing — production callers
	// who tripped this by chdir'ing to a non-module path see it in the
	// failure message. The marker is a stable token so prose can evolve
	// without breaking this assertion.
	if !strings.Contains(fake.Fatal(), RepoRootChdirHint) {
		t.Errorf("Fatalf message = %q, want substring %q (remediation marker missing)", fake.Fatal(), RepoRootChdirHint)
	}
}

// TestRepoRoot_SkipsForeignGoMod is the regression guard for the whole
// reason ExpectedModulePath exists. We construct a directory tree:
//
//	tmp/
//	  go.mod                 ← canonical (declares ExpectedModulePath)
//	  inner/
//	    go.mod               ← foreign (declares some other module)
//	    pkg/                 ← test wd
//
// and assert RepoRoot returns tmp/, NOT tmp/inner/. Without the
// module-path filter in repo.go, RepoRoot would return tmp/inner/ —
// silently rerouting contract tests that read repo-rooted paths.
//
// Negative control: we directly call readModulePath on the inner go.mod
// and assert it returns "example.com/fixture/sub". This proves the
// fixture's foreign module IS PARSEABLE — so the skip is filter-driven,
// not parse-failure-driven. A regression in readModulePath that returned
// ("", false) for valid go.mods would silently turn this test into a
// no-op without the negative control.
func TestRepoRoot_SkipsForeignGoMod(t *testing.T) {
	tmp := t.TempDir()

	outerMod := "module " + ExpectedModulePath + "\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(outerMod), 0o644); err != nil {
		t.Fatalf("write outer go.mod: %v", err)
	}

	innerDir := filepath.Join(tmp, "inner")
	if err := os.MkdirAll(innerDir, 0o755); err != nil {
		t.Fatalf("mkdir inner: %v", err)
	}
	const innerModule = "example.com/fixture/sub"
	innerMod := "module " + innerModule + "\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(innerDir, "go.mod"), []byte(innerMod), 0o644); err != nil {
		t.Fatalf("write inner go.mod: %v", err)
	}

	// Negative control: prove the fixture's go.mod is itself parseable
	// as a foreign module. If readModulePath has regressed to always
	// returning ("", false), the SkipsForeignGoMod assertion below
	// would still pass (because the walk treats parse failures the
	// same as foreign modules), and the test would silently lose its
	// teeth.
	parsed, ok := readModulePath(filepath.Join(innerDir, "go.mod"))
	if !ok {
		t.Fatalf("negative control: readModulePath rejected the fixture inner go.mod — readModulePath has regressed")
	}
	if parsed != innerModule {
		t.Fatalf("negative control: readModulePath returned %q, want %q", parsed, innerModule)
	}

	pkgDir := filepath.Join(innerDir, "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}

	// Inject pkgDir as the wd directly — no t.Chdir gymnastics.
	got := repoRootFromWd(t, pkgDir)
	AssertPathsEqual(t, got, tmp, "foreign go.mod was not skipped")
}

// TestExpectedModulePath_MatchesGoMod is the drift guard between the
// `ExpectedModulePath` const and the repository's actual `go.mod`. If a
// future fork or rename updates one without the other, every contract
// test that resolves repo-rooted paths starts skipping or fataling — this
// test fires first with a clear message instead.
//
// We resolve `go.mod` via os.ReadFile + readModulePath so we exercise the
// same parser RepoRoot uses; an unrelated regression that broke
// readModulePath without changing the const would surface here too.
func TestExpectedModulePath_MatchesGoMod(t *testing.T) {
	root := RepoRoot(t)
	goModPath := filepath.Join(root, "go.mod")
	module, ok := readModulePath(goModPath)
	if !ok {
		t.Fatalf("readModulePath(%q) returned !ok — go.mod parse regression", goModPath)
	}
	if module != ExpectedModulePath {
		t.Errorf("ExpectedModulePath = %q, but %s declares module %q — update the const in repo.go and any test fixtures in lockstep with go.mod",
			ExpectedModulePath, goModPath, module)
	}
}

// TestReadModulePath_TableEdges directly covers each go.mod parse shape
// readModulePath claims to handle (and a few intentional non-shapes).
// These are exercised transitively elsewhere, but pinning them as direct
// cases lets a refactor of the parser fail HERE first with a precise
// case label, instead of in some downstream contract test that reads
// the repo's real go.mod.
func TestReadModulePath_TableEdges(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantPath string
		wantOK   bool
	}{
		{
			name:     "plain space-separated",
			body:     "module example.com/foo\n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "tab-separated",
			body:     "module\texample.com/foo\n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "multiple spaces between keyword and path",
			body:     "module    example.com/foo\n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "leading comments and blank lines",
			body:     "// header comment\n\n// another\nmodule example.com/foo\n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "balanced quotes stripped",
			body:     `module "example.com/foo"` + "\n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "unbalanced leading quote NOT stripped",
			body:     `module "example.com/foo` + "\n",
			wantPath: `"example.com/foo`,
			wantOK:   true,
		},
		{
			name:     "unbalanced trailing quote NOT stripped",
			body:     `module example.com/foo"` + "\n",
			wantPath: `example.com/foo"`,
			wantOK:   true,
		},
		{
			name:     "empty file",
			body:     "",
			wantPath: "",
			wantOK:   false,
		},
		{
			name:     "first non-comment line is not a module directive",
			body:     "go 1.24\nmodule example.com/foo\n",
			wantPath: "",
			wantOK:   false,
		},
		{
			name:     "comments only, no directive",
			body:     "// just a comment\n// and another\n",
			wantPath: "",
			wantOK:   false,
		},
		{
			name:     "module directive with trailing whitespace",
			body:     "module example.com/foo   \n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "CRLF line endings",
			body:     "// header\r\nmodule example.com/foo\r\n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "inline comment after path",
			body:     "module example.com/foo // alias note\n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "inline comment with quoted path",
			body:     `module "example.com/foo" // x` + "\n",
			wantPath: "example.com/foo",
			wantOK:   true,
		},
		{
			name:     "modulefoo (no space) is NOT a directive",
			body:     "modulefoo example.com/foo\n",
			wantPath: "",
			wantOK:   false,
		},
		{
			name:     "bare module keyword with no path",
			body:     "module\n",
			wantPath: "",
			wantOK:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "go.mod")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write fixture: %v", err)
			}
			got, ok := readModulePath(path)
			if ok != tc.wantOK {
				t.Errorf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.wantPath {
				t.Errorf("path = %q, want %q", got, tc.wantPath)
			}
		})
	}
}

// TestReadModulePath_NonexistentFile pins the read-error path: a
// missing file returns ("", false) without panicking. The walk-up logic
// in repoRootFromWd relies on this — every parent directory is asked
// for a go.mod and most don't have one.
func TestReadModulePath_NonexistentFile(t *testing.T) {
	got, ok := readModulePath(filepath.Join(t.TempDir(), "does-not-exist"))
	if ok {
		t.Errorf("ok = true, want false for missing file (got path %q)", got)
	}
	if got != "" {
		t.Errorf("path = %q, want empty for missing file", got)
	}
}
