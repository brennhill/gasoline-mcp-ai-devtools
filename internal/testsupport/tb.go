// tb.go — Canonical minimal *testing.T-shaped interfaces consumed by
// helpers in this package (RepoRoot, AssertPathResolvesTo,
// ExpectFakeFatal). Defining them here, separately from faket.go,
// reflects that they are CROSS-CUTTING contract types — every helper
// in the package accepts one of them. A reader looking for
// "what does RepoRoot accept?" lands here, not in the FakeT
// implementation file.
//
// Both interfaces are exported so external test packages can name the
// type when declaring variables (e.g., a helper that wraps multiple
// fakes). *testing.T and *testing.B satisfy both implicitly; *FakeT
// satisfies HelperFatalfTB (via Fatalf) and structurally satisfies
// HelperFatalTB if a future need arises.

package testsupport

// HelperFatalfTB is the canonical minimal subset of *testing.T behavior
// for helpers that fail the test via Fatalf (formatted message). Used
// by RepoRoot, AssertPathResolvesTo, and any future helper that needs
// the "Helper() + Fatalf()" shape.
type HelperFatalfTB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// HelperFatalTB is the canonical minimal subset for helpers that fail
// the test via Fatal (un-formatted, args spread). Used by
// ExpectFakeFatal to fail the surrounding *testing.T when the body
// returns normally without invoking FakeT.Fatalf.
//
// Separate from HelperFatalfTB because Fatal and Fatalf are distinct
// methods on testing.TB; merging them would require callers to choose
// at every site.
type HelperFatalTB interface {
	Helper()
	Fatal(args ...any)
}
