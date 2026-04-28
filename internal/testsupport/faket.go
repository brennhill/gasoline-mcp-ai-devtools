// faket.go — Shared *testing.T fake for self-tests of helpers that take
// minimal testing.TB-shaped interfaces. Captures Fatalf without aborting
// the surrounding *testing.T so the failure path is observable as a
// value to assert against, not a fatal signal that aborts the run.
//
// Why this lives in the production package (not a _test.go file): every
// caller is in a different test binary (internal/telemetry,
// internal/testsupport, future packages), and Go does not let a `_test.go`
// in package P be imported by a `_test.go` in package Q. The bar in the
// package doc — "used by 2+ test packages" — is met. A contract test
// (`TestPackageNotImportedByProductionCode` in package_isolation_test.go)
// guards the "production code MUST NOT import this package" rule.
//
// Canonical TB-shaped interfaces live in this file (helperFatalfTB,
// helperFatalTB) so every helper in the package — RepoRoot,
// AssertPathsEqual, ExpectFakeFatal — uses the same canonical names
// rather than each one inventing its own minimal subset.

package testsupport

import (
	"fmt"
	"sync"
)

// helperFatalfTB is the canonical minimal subset of *testing.T behavior
// for helpers that fail the test via Fatalf (formatted message). Used
// by RepoRoot, AssertPathsEqual, and any future helper that needs the
// "Helper() + Fatalf()" shape.
//
// *testing.T and *testing.B satisfy this implicitly. *FakeT also
// satisfies it, which is why these helpers are testable without a real
// *testing.T.
type helperFatalfTB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// helperFatalTB is the canonical minimal subset for helpers that fail
// the test via Fatal (un-formatted, args spread). Used by
// ExpectFakeFatal to fail the surrounding *testing.T when the body
// returns normally without invoking FakeT.Fatalf.
//
// Separate from helperFatalfTB because Fatal and Fatalf are distinct
// methods on testing.TB; merging them would require callers to choose
// at every site.
type helperFatalTB interface {
	Helper()
	Fatal(args ...any)
}

// FakeFatalSentinel is the panic value FakeT.Fatalf uses to abort the
// calling goroutine. Tests recover() this sentinel explicitly so any other
// panic still propagates.
type FakeFatalSentinel struct{}

// FakeT is a minimal *testing.T fake. It satisfies any interface composed
// of Helper + Fatalf (+ optional Cleanup), which covers both the
// helperFatalfTB and the rotationT (in package telemetry) shapes.
//
// Fatalf appends the formatted message to the internal fatals slice and
// panics with FakeFatalSentinel{}; the calling test recovers and
// inspects via Fatal() (last message) or Fatals() (full history). Cleanup
// is collected in registration order and exposed via RunCleanups, which
// fires LIFO to mirror testing.T.Cleanup semantics.
//
// Concurrent use: writes to fatals/cleanups are mutex-guarded so a fake
// shared between goroutines (e.g., a future test that drives a helper
// from multiple workers) does not race the recorder. Reads through
// Fatal() / Fatals() are also guarded; direct field access is forbidden.
type FakeT struct {
	mu       sync.Mutex
	fatals   []string
	cleanups []func()
}

// Helper is a no-op required by the testing.TB-shaped interfaces.
func (f *FakeT) Helper() {}

// Fatalf records the formatted message and panics with
// FakeFatalSentinel{}. The caller is expected to recover the sentinel
// (via the recoverFakeFatal helper or, more commonly, ExpectFakeFatal)
// and inspect via Fatal() / Fatals().
//
// Multiple Fatalf invocations in a single body are allowed and recorded
// in the fatals slice. Real testing.T.Fatalf aborts the goroutine, so
// in practice a body produces exactly one fatal — but real code can
// invoke a fake's Fatalf from a deferred cleanup, in which case both
// messages must be visible to the test asserting the order.
func (f *FakeT) Fatalf(format string, args ...any) {
	f.mu.Lock()
	f.fatals = append(f.fatals, fmt.Sprintf(format, args...))
	f.mu.Unlock()
	panic(FakeFatalSentinel{})
}

// Fatal returns the most recently recorded Fatalf message, or "" if no
// Fatalf has fired. For history, use Fatals().
func (f *FakeT) Fatal() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.fatals) == 0 {
		return ""
	}
	return f.fatals[len(f.fatals)-1]
}

// Fatals returns a snapshot of every Fatalf message recorded so far, in
// the order they fired. The returned slice is independent of the FakeT's
// internal state; the caller may mutate it freely.
func (f *FakeT) Fatals() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.fatals))
	copy(out, f.fatals)
	return out
}

// clearFatals resets the recorded messages. Used internally by
// ExpectFakeFatal so a stale prior message cannot mask a body that
// returns normally without invoking Fatalf.
func (f *FakeT) clearFatals() {
	f.mu.Lock()
	f.fatals = nil
	f.mu.Unlock()
}

// Cleanup registers fn to be invoked by a later RunCleanups call.
func (f *FakeT) Cleanup(fn func()) {
	f.mu.Lock()
	f.cleanups = append(f.cleanups, fn)
	f.mu.Unlock()
}

// RunCleanups fires all registered cleanups in LIFO order, mirroring
// testing.T.Cleanup semantics, and clears the list so a second call is a
// no-op (also matching real testing.T, which does not re-fire cleanups).
//
// Cleanups run WITHOUT the internal mutex held — a cleanup is allowed
// to call back into FakeT methods (e.g., register another cleanup, or
// invoke Fatalf) without deadlocking.
func (f *FakeT) RunCleanups() {
	f.mu.Lock()
	cleanups := f.cleanups
	f.cleanups = nil
	f.mu.Unlock()
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

// recoverFakeFatal is a deferred-call helper that recovers a
// FakeFatalSentinel panic and re-panics any other value so unexpected
// panics still propagate.
//
// Unexported because every external caller uses ExpectFakeFatal, which
// wraps recoverFakeFatal with the surrounding "expected panic; did not
// occur" assertion. Internal tests inside the testsupport package can
// reach this directly if they need bare recover semantics.
func recoverFakeFatal() {
	r := recover()
	if r == nil {
		return
	}
	if _, ok := r.(FakeFatalSentinel); ok {
		return
	}
	panic(r)
}

// ExpectFakeFatal runs body() expecting it to invoke fake.Fatalf (which
// panics with FakeFatalSentinel{}). Recovers the sentinel and returns
// normally. If body returns without panicking, fails t with a stable
// "did not occur" message.
//
// This is the recommended call shape for any test that asserts a helper
// fires Fatalf:
//
//	testsupport.ExpectFakeFatal(t, fake, func() { helperUnderTest(fake) })
//
// Pre-existing fake.Fatal content is cleared on entry so a stale value
// from a prior call cannot mask a body that silently returns without
// invoking Fatalf.
func ExpectFakeFatal(t helperFatalTB, fake *FakeT, body func()) {
	t.Helper()
	fake.clearFatals()
	defer recoverFakeFatal()
	body()
	t.Fatal("expected FakeT.Fatalf to fire; did not occur — body returned normally")
}
