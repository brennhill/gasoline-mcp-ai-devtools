// repo_test.go — Self-tests for testsupport.RepoRoot. Verify the happy-path
// walk-up returns the repo root containing go.mod, and the failure mode
// fires testing.TB.Fatalf with a remediation hint when no go.mod is reachable.

package testsupport

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeTB satisfies the repoRootTB interface (Helper + Fatalf) — the
// minimal subset RepoRoot uses. We can't implement the full testing.TB
// (unexported methods) so RepoRoot's parameter type was tightened to
// repoRootTB precisely so this self-test could exist.
type fakeTB struct {
	fatal string
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Fatalf(format string, args ...any) {
	f.fatal = fmt.Sprintf(format, args...)
	panic(fakeFatalSentinel{})
}

type fakeFatalSentinel struct{}

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
	const wantModule = "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP"
	if !strings.Contains(string(body), wantModule) {
		t.Errorf("go.mod at %s does not declare module %q (full body: %q)", root, wantModule, body)
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

	fake := &fakeTB{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(fakeFatalSentinel); !ok {
					panic(r)
				}
			}
		}()
		RepoRoot(fake)
		t.Fatal("expected fakeTB.Fatalf panic; did not occur")
	}()
	if fake.fatal == "" {
		t.Fatal("Fatalf was not called from a directory with no go.mod ancestor")
	}
	// The remediation hint mentions t.Chdir — load-bearing for users
	// who tripped this by chdir'ing to a non-module path.
	if !strings.Contains(fake.fatal, "t.Chdir") {
		t.Errorf("Fatalf message = %q, want substring %q (remediation hint missing)", fake.fatal, "t.Chdir")
	}
}
