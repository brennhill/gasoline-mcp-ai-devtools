// paths.go — Filesystem path helpers shared across test files. Kept
// separate from repo.go so the file's concerns stay single-purpose
// (per CLAUDE.md best-practices §5: "Keep modules single-purpose").

package testsupport

import "path/filepath"

// AssertPathResolvesTo fails t (via Fatalf) unless `actual` and
// `expected` resolve to the same canonical filesystem path after
// symlink resolution. Used by tests that compare paths returned from
// APIs that may resolve `/var` ↔ `/private/var` (macOS) or other
// platform-symlink quirks.
//
// The verb-shaped name (`AssertPathResolvesTo`) makes argument order
// unambiguous at the call site: read as "assert {actual} resolves to
// {expected}". The earlier name `AssertPathsEqual` had three same-
// typed string arguments in a row, inviting silent swaps that would
// produce a misleading "got X, want Y" on inequality.
//
// `msg` is variadic so call sites that have no extra context can omit
// it entirely (vs. typing `""` to advertise an uninformative empty).
// When supplied, the strings are joined with spaces and appended to
// every failure mode (inequality OR symlink-lookup error).
//
// Both inputs MUST exist on disk; missing paths fail the test via
// Fatalf with the same msg appended.
func AssertPathResolvesTo(t HelperFatalfTB, actual, expected string, msg ...string) {
	t.Helper()
	tail := joinMsg(msg)
	actualResolved, err := filepath.EvalSymlinks(actual)
	if err != nil {
		t.Fatalf("AssertPathResolvesTo: EvalSymlinks(%q): %v%s", actual, err, tail)
		return
	}
	expectedResolved, err := filepath.EvalSymlinks(expected)
	if err != nil {
		t.Fatalf("AssertPathResolvesTo: EvalSymlinks(%q): %v%s", expected, err, tail)
		return
	}
	if actualResolved != expectedResolved {
		t.Fatalf("paths differ: actual %q (resolved %q), expected %q (resolved %q)%s",
			actual, actualResolved, expected, expectedResolved, tail)
	}
}

// joinMsg formats the variadic msg slice into a `: <msg>` suffix for
// Fatalf templates, or "" if the slice is empty. Hoisted so every
// failure branch in AssertPathResolvesTo formats consistently.
func joinMsg(msg []string) string {
	if len(msg) == 0 {
		return ""
	}
	out := ":"
	for _, m := range msg {
		if m == "" {
			continue
		}
		out += " " + m
	}
	if out == ":" {
		// All msg entries were empty strings — produce no suffix.
		return ""
	}
	return out
}
