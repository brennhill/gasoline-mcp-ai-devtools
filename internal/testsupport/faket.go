// faket.go — Shared *testing.T fake for self-tests of helpers that take
// minimal testing.TB-shaped interfaces (e.g., repoRootTB, rotationT).
// Captures Fatalf without aborting the surrounding *testing.T so the
// failure path is observable as a value to assert against, not a fatal
// signal that aborts the run.
//
// Why this lives in the production package (not a _test.go file): every
// caller is in a different test binary (internal/telemetry,
// internal/testsupport, future packages), and Go does not let a `_test.go`
// in package P be imported by a `_test.go` in package Q. The bar in the
// package doc — "used by 2+ test packages" — is met. A contract test
// (`TestPackageNotImportedByProductionCode` in package_isolation_test.go)
// guards the "production code MUST NOT import this package" rule that
// the package doc states.

package testsupport

import "fmt"

// FakeFatalSentinel is the panic value FakeT.Fatalf uses to abort the
// calling goroutine. Tests recover() this sentinel explicitly so any other
// panic still propagates.
type FakeFatalSentinel struct{}

// FakeT is a minimal *testing.T fake. It satisfies any interface composed
// of Helper + Fatalf (+ optional Cleanup), which covers both the
// repoRootTB (Helper, Fatalf) and rotationT (Helper, Fatalf, Cleanup)
// interfaces in this repo.
//
// Fatalf records the formatted message into Fatal and panics with
// FakeFatalSentinel{}; the calling test recovers and inspects Fatal to
// assert the failure path was taken. Cleanup is collected in registration
// order and exposed via RunCleanups, which fires them LIFO to mirror
// testing.T.Cleanup semantics.
//
// FakeT is NOT safe for concurrent use — each test goroutine must own
// its own instance.
type FakeT struct {
	Fatal    string
	cleanups []func()
}

// Helper is a no-op required by the testing.TB-shaped interfaces.
func (f *FakeT) Helper() {}

// Fatalf records the message and panics with FakeFatalSentinel{}. The
// caller is expected to recover the sentinel (via RecoverFakeFatal in a
// deferred wrapper, or via the higher-level ExpectFakeFatal helper) and
// inspect FakeT.Fatal.
func (f *FakeT) Fatalf(format string, args ...any) {
	f.Fatal = fmt.Sprintf(format, args...)
	panic(FakeFatalSentinel{})
}

// Cleanup registers fn to be invoked by a later RunCleanups call.
func (f *FakeT) Cleanup(fn func()) {
	f.cleanups = append(f.cleanups, fn)
}

// RunCleanups fires all registered cleanups in LIFO order, mirroring
// testing.T.Cleanup semantics, and clears the list so a second call is a
// no-op (also matching real testing.T, which does not re-fire cleanups).
// Tests that need to observe restore behavior invoke this manually after
// the body returns.
func (f *FakeT) RunCleanups() {
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
	f.cleanups = nil
}

// RecoverFakeFatal is a deferred-call helper that recovers a
// FakeFatalSentinel panic and re-panics any other value so unexpected
// panics still propagate. Use as `defer RecoverFakeFatal()` in func
// literals that wrap a call expected to invoke FakeT.Fatalf.
//
// Most call sites should prefer ExpectFakeFatal, which adds the
// "expected panic; did not occur" assertion the IIFE wrapper otherwise
// has to repeat by hand.
func RecoverFakeFatal() {
	r := recover()
	if r == nil {
		return
	}
	if _, ok := r.(FakeFatalSentinel); ok {
		return
	}
	panic(r)
}

// fatalTB is the minimal *testing.T behavior ExpectFakeFatal needs to
// fail the surrounding test when the body did NOT panic with a
// FakeFatalSentinel. *testing.T satisfies this implicitly.
type fatalTB interface {
	Helper()
	Fatal(args ...any)
}

// ExpectFakeFatal runs body() expecting it to invoke fake.Fatalf (which
// panics with FakeFatalSentinel{}). Recovers the sentinel and returns
// normally. If body returns without panicking, fails t with a stable
// "did not occur" message.
//
// This is the recommended call shape for any test that asserts a helper
// fires Fatalf. The hand-rolled
//
//	func() {
//	    defer testsupport.RecoverFakeFatal()
//	    helperUnderTest(fake)
//	    t.Fatal("expected panic; did not occur")
//	}()
//
// idiom collapses to:
//
//	testsupport.ExpectFakeFatal(t, fake, func() { helperUnderTest(fake) })
//
// — fewer lines per call site, and the "did not occur" assertion can't
// drift between sites.
//
// Pre-existing fake.Fatal content is cleared on entry so a stale value
// from a prior call cannot mask a body that silently returns without
// invoking Fatalf.
func ExpectFakeFatal(t fatalTB, fake *FakeT, body func()) {
	t.Helper()
	fake.Fatal = ""
	defer RecoverFakeFatal()
	body()
	t.Fatal("expected FakeT.Fatalf to fire; did not occur — body returned normally")
}
