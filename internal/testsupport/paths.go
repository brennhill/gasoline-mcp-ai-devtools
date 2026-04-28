// paths.go — Filesystem path helpers shared across test files. Kept
// separate from repo.go so the file's concerns stay single-purpose
// (per CLAUDE.md best-practices §5: "Keep modules single-purpose").

package testsupport

import "path/filepath"

// AssertPathsEqual fails t (via Fatalf) unless got and want resolve to
// the same canonical filesystem path after symlink resolution. Used by
// tests that compare paths returned from APIs that may resolve
// `/var` ↔ `/private/var` (macOS) or other platform-symlink quirks.
//
// The msg parameter is appended to the failure message to give the test
// a chance to explain what it was checking (e.g., "foreign go.mod was
// not skipped"). Pass "" if no extra context is needed.
//
// Both inputs MUST exist on disk; missing paths fail the test via
// Fatalf with a clear diagnostic. The function returns nothing — call
// sites that need a continue-on-error semantic should compare resolved
// paths inline rather than reach for this helper.
func AssertPathsEqual(t helperFatalfTB, got, want, msg string) {
	t.Helper()
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("AssertPathsEqual: EvalSymlinks(%q): %v", got, err)
		return
	}
	wantResolved, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("AssertPathsEqual: EvalSymlinks(%q): %v", want, err)
		return
	}
	if gotResolved != wantResolved {
		if msg == "" {
			t.Fatalf("paths differ: got %q (resolved %q), want %q (resolved %q)",
				got, gotResolved, want, wantResolved)
			return
		}
		t.Fatalf("paths differ: got %q (resolved %q), want %q (resolved %q): %s",
			got, gotResolved, want, wantResolved, msg)
	}
}
