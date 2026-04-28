// repo_test.go — Self-tests for testsupport.RepoRoot. Verify the happy-path
// walk-up returns the repo root containing go.mod, the failure mode fires
// testing.TB.Fatalf with a remediation hint when no go.mod is reachable,
// and the foreign-module skip branch (the whole reason ExpectedModulePath
// exists) actually walks past a non-matching go.mod.

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

// TestRepoRoot_FatalfWhenNoGoMod simulates an environment where no
// ancestor directory contains a go.mod. We can't actually chdir to "/"
// (the test framework would lose the test binary), so we spawn a fake T
// that captures Fatalf, and chdir to a tempdir that we KNOW has no go.mod
// in any ancestor — except: the test's tempdir is normally inside a
// /var/folders or /tmp path that has no go.mod ancestors, so this works.
//
// The test asserts the captured Fatalf message contains the remediation
// hint about t.Chdir, which is what makes the failure actionable.
func TestRepoRoot_FatalfWhenNoGoMod(t *testing.T) {
	// Make sure we're chdir'd to a path with no go.mod in any ancestor.
	// /var/folders/... (macOS) and /tmp/... typically lack go.mod, but
	// guard with a runtime check: if our tempdir DOES have a go.mod
	// ancestor (e.g., on a contributor's machine where $TMPDIR sits
	// inside a Go module), skip rather than produce a misleading pass.
	tmp := t.TempDir()
	for d := tmp; d != filepath.Dir(d); d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			t.Skipf("ancestor %q contains go.mod; cannot exercise the not-found branch on this filesystem", d)
		}
	}

	t.Chdir(tmp) // restored automatically when test ends

	fake := &FakeT{}
	func() {
		defer RecoverFakeFatal()
		RepoRoot(fake)
		t.Fatal("expected FakeT.Fatalf panic; did not occur")
	}()
	if fake.Fatal == "" {
		t.Fatal("Fatalf was not called from a directory with no go.mod ancestor")
	}
	// The remediation hint mentions t.Chdir — load-bearing for users
	// who tripped this by chdir'ing to a non-module path.
	if !strings.Contains(fake.Fatal, "t.Chdir") {
		t.Errorf("Fatalf message = %q, want substring %q (remediation hint missing)", fake.Fatal, "t.Chdir")
	}
}

// TestRepoRoot_SkipsForeignGoMod is the regression guard for the whole
// reason ExpectedModulePath exists. We construct a directory tree like:
//
//	tmp/
//	  go.mod                 ← canonical (declares ExpectedModulePath)
//	  inner/
//	    go.mod               ← foreign (declares some other module)
//	    pkg/                 ← cwd
//
// and assert RepoRoot returns tmp/, NOT tmp/inner/. Without the
// module-path filter in repo.go, RepoRoot would return tmp/inner/ —
// silently rerouting contract tests that read repo-rooted paths.
//
// A regression in readModulePath that always returns ("", false) would
// trip the not-found branch tested above, but a regression that returned
// the WRONG module name would silently route to whichever go.mod was
// nearest. This test pins both correctness layers.
func TestRepoRoot_SkipsForeignGoMod(t *testing.T) {
	tmp := t.TempDir()

	// Outer go.mod: the canonical one we expect RepoRoot to pick.
	outerMod := "module " + ExpectedModulePath + "\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(outerMod), 0o644); err != nil {
		t.Fatalf("write outer go.mod: %v", err)
	}

	// Inner go.mod: a fixture sub-module masquerading as a root.
	innerDir := filepath.Join(tmp, "inner")
	if err := os.MkdirAll(innerDir, 0o755); err != nil {
		t.Fatalf("mkdir inner: %v", err)
	}
	innerMod := "module example.com/fixture/sub\n\ngo 1.24\n"
	if err := os.WriteFile(filepath.Join(innerDir, "go.mod"), []byte(innerMod), 0o644); err != nil {
		t.Fatalf("write inner go.mod: %v", err)
	}

	pkgDir := filepath.Join(innerDir, "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}

	t.Chdir(pkgDir)

	got := RepoRoot(t)
	want := tmp
	// Resolve symlinks both sides so /private/var/folders/... vs
	// /var/folders/... (macOS tempdir aliasing) does not flake the
	// equality check.
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks(got): %v", err)
	}
	wantResolved, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("EvalSymlinks(want): %v", err)
	}
	if gotResolved != wantResolved {
		t.Errorf("RepoRoot returned %q (resolved %q); want %q (resolved %q) — foreign go.mod was not skipped",
			got, gotResolved, want, wantResolved)
	}
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
