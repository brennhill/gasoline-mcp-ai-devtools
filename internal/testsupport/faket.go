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
// package doc — "used by 2+ test packages" — is met.

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
// deferred wrapper) and inspect FakeT.Fatal.
func (f *FakeT) Fatalf(format string, args ...any) {
	f.Fatal = fmt.Sprintf(format, args...)
	panic(FakeFatalSentinel{})
}

// Cleanup registers fn to be invoked by a later RunCleanups call.
func (f *FakeT) Cleanup(fn func()) {
	f.cleanups = append(f.cleanups, fn)
}

// RunCleanups fires all registered cleanups in LIFO order, mirroring
// testing.T.Cleanup semantics. Tests that need to observe restore
// behavior invoke this manually after the body returns (a real *testing.T
// would fire cleanups automatically post-test).
func (f *FakeT) RunCleanups() {
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
}

// RecoverFakeFatal is a deferred-call helper that recovers a
// FakeFatalSentinel panic and re-panics any other value so unexpected
// panics still propagate. Use as:
//
//	func() {
//	    defer testsupport.RecoverFakeFatal()
//	    helperUnderTest(fake)  // expected to invoke fake.Fatalf
//	}()
//
// After the func returns, inspect fake.Fatal to assert the failure path
// fired with the expected message.
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
