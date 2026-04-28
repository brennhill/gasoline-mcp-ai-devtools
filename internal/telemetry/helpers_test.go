// helpers_test.go — Test-only helpers that manipulate package-level state
// for installation ID, beacon endpoint, and emission notifications.
// Compiled only under `go test`, so the production binary never ships these
// symbols.
//
// State-rotation pattern: each `withXxxState(t, ...)` helper acquires a
// dedicated mutex (lockBudgetMu, homeDirFnMu, etc.) via TryLock + t.Fatalf
// so nested-misuse fails loudly at the call site, then restores via
// t.Cleanup so a t.Fatal cannot leak overrides across tests. The mutexes
// are independent (no cross-rotation deadlock today). If a 4th rotation
// pattern joins, consolidate the per-mutex pairs into a single
// withTestState(t, mutations...) helper to keep the family discoverable
// and prevent cross-mutex acquisition-order hazards.
//
// The persist hook vars themselves (installIDBeforePersistHook,
// firstToolCallBeforePersistHook, onFireBeacon) remain in their respective
// production files because production code references them as the
// no-op-by-default extension point. Tests assign through these helpers or
// directly to the var to drive deterministic interleavings.

package telemetry

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// overrideKaboomDir sets a custom directory for testing. Also disables the
// platform-stable secondary mirror so tests retain single-location semantics
// unless they explicitly opt in via overrideSecondaryDir.
func overrideKaboomDir(dir string) {
	kaboomDir = dir
	secondaryDirOverride = ""
	secondaryDirDisabled = true
}

// overrideSecondaryDir enables the secondary mirror at the given path for
// tests that exercise the cross-location feature. Must be called AFTER
// overrideKaboomDir (which disables the secondary by default).
func overrideSecondaryDir(dir string) {
	secondaryDirOverride = dir
	secondaryDirDisabled = false
}

// resetKaboomDir restores the default Kaboom directory after testing.
func resetKaboomDir() {
	kaboomDir = defaultKaboomDir()
	secondaryDirOverride = ""
	secondaryDirDisabled = false
}

// resetInstallIDState clears the cached install ID for testing. Race-safe
// with parallel GetInstallID callers via installIDLoadMu, AND with stale
// leader cleanup via the leader's `installIDLoadInFlight == op` guard at
// install_id.go (the leader will not clobber a successor op installed
// after its own slow path returned). The reset also zeroes
// installIDLoadInFlight so the next caller becomes a fresh leader.
func resetInstallIDState() {
	installIDLoadMu.Lock()
	defer installIDLoadMu.Unlock()
	cachedInstallIDPtr.Store(nil)
	installIDLoadInFlight = nil
}

// resetFirstToolCallState clears the cached first-tool-call state for testing.
// Race-safe via firstToolCallMu (same lock markFirstToolCallEmittedForInstall holds).
func resetFirstToolCallState() {
	firstToolCallMu.Lock()
	defer firstToolCallMu.Unlock()
	cachedFirstToolCallLoaded = false
	cachedFirstToolCallInstallID = ""
}

// overrideEndpoint sets a custom endpoint for testing.
func overrideEndpoint(url string) {
	beaconMu.Lock()
	endpoint = url
	beaconMu.Unlock()
}

// resetEndpoint restores the default endpoint after testing.
func resetEndpoint() {
	beaconMu.Lock()
	endpoint = defaultEndpoint
	beaconMu.Unlock()
}

// setOnFireBeacon sets the test hook (use nil to clear).
func setOnFireBeacon(fn func(sent bool)) {
	onFireBeaconMu.Lock()
	onFireBeacon = fn
	onFireBeaconMu.Unlock()
}

// rotationT is the minimal subset of *testing.T behavior withRotation
// requires. Defining it as an interface lets the helpers self-test
// (helpers_internal_test.go) drive a fake T and observe the misuse-guard
// branch without aborting the surrounding test. *testing.T satisfies the
// interface implicitly.
type rotationT interface {
	Helper()
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// withRotation is the canonical state-rotation primitive. It TryLocks the
// given mutex, runs save() to snapshot existing state, registers a t.Cleanup
// that restores via restore(snapshot) and unlocks the mutex. The label
// flows into the misuse-guard t.Fatalf so all rotation helpers produce
// uniform diagnostic messages. Use this directly when adding a new rotation
// pattern; don't replicate the TryLock/Cleanup boilerplate inline.
//
// Generic over T: the snapshot type is inferred from save(). Callers no
// longer perform `s.(SnapshotType)` type assertions, so a misspelled type
// becomes a compile error rather than a runtime panic.
//
// The label remains a hand-passed string (not derived via runtime.Caller):
// it appears verbatim in the misuse t.Fatalf message and serves as a
// human-meaningful tag, not a stack-frame name. A Caller-derived helper
// name (e.g., "withLockBudget.func1") would be noisier without adding
// clarity.
//
// CONSTRAINT: do NOT call any with*State helper from a subtest of a test
// that already called the same helper. The misuse guard fires t.Fatalf at
// the offending call site rather than blocking on `go test -timeout`.
func withRotation[T any](t rotationT, mu *sync.Mutex, label string, save func() T, restore func(T)) {
	t.Helper()
	if !mu.TryLock() {
		t.Fatalf("%s: mutex already held — likely a nested call from a subtest of a test that already called %s (see CONSTRAINT in helpers_test.go)", label, label)
	}
	snapshot := save()
	t.Cleanup(func() {
		restore(snapshot)
		mu.Unlock()
	})
}

// lockBudgetMu serializes concurrent mutation of installStateLockTimeout/
// Poll/Stale. Tests in this package don't use t.Parallel(), but a future
// `go test -count=N` run could overlap cleanups if -p>1 splits packages.
var lockBudgetMu sync.Mutex

// withLockBudget shrinks installStateLockTimeout/Poll/Stale for the duration
// of t and restores via t.Cleanup. Subject to withRotation's CONSTRAINT.
func withLockBudget(t *testing.T, timeout, poll, stale time.Duration) {
	t.Helper()
	withRotation(t, &lockBudgetMu, "withLockBudget",
		func() [3]time.Duration {
			return [3]time.Duration{installStateLockTimeout, installStateLockPoll, installStateLockStale}
		},
		func(v [3]time.Duration) {
			installStateLockTimeout, installStateLockPoll, installStateLockStale = v[0], v[1], v[2]
		})
	installStateLockTimeout = timeout
	installStateLockPoll = poll
	installStateLockStale = stale
}

// homeDirFnMu serializes concurrent mutation of userHomeDirFn.
var homeDirFnMu sync.Mutex

// withHomeDirFn rotates userHomeDirFn for the duration of t. Subject to
// withRotation's CONSTRAINT.
func withHomeDirFn(t *testing.T, fn func() (string, error)) {
	t.Helper()
	withRotation(t, &homeDirFnMu, "withHomeDirFn",
		func() func() (string, error) { return userHomeDirFn },
		func(prev func() (string, error)) { userHomeDirFn = prev })
	userHomeDirFn = fn
}

// secondaryDirStateMu serializes mutation of secondaryDirDisabled and
// secondaryDirOverride.
var secondaryDirStateMu sync.Mutex

// withSecondaryDirState rotates secondaryDirDisabled and secondaryDirOverride
// for the duration of t. Subject to withRotation's CONSTRAINT.
func withSecondaryDirState(t *testing.T, disabled bool, override string) {
	t.Helper()
	type secondaryDirSnapshot struct {
		disabled bool
		override string
	}
	withRotation(t, &secondaryDirStateMu, "withSecondaryDirState",
		func() secondaryDirSnapshot {
			return secondaryDirSnapshot{disabled: secondaryDirDisabled, override: secondaryDirOverride}
		},
		func(v secondaryDirSnapshot) {
			secondaryDirDisabled = v.disabled
			secondaryDirOverride = v.override
		})
	secondaryDirDisabled = disabled
	secondaryDirOverride = override
}

// firstWriterGate installs a single-shot gate on a *func() persist hook
// (e.g., installIDBeforePersistHook, firstToolCallBeforePersistHook). The
// first goroutine to reach the hook signals `entered`; subsequent goroutines
// pass through immediately. Call `release()` to let the first goroutine
// proceed past the hook.
//
// Leak-safety: t.Cleanup always closes `gate` (via sync.Once so an explicit
// release() does not double-close) and nils the hook var. So a t.Fatal
// before release() does NOT leak the parked writer — cleanup unblocks it
// and lets it complete its persist + return its result, freeing the file
// lock for downstream tests.
//
// Misuse guard: panics if hookVar already points at a gate (re-installing
// would orphan the prior gate). Each test should install at most one.
//
// Usage:
//
//	entered, release := firstWriterGate(t, &installIDBeforePersistHook)
//	go func() { result <- loadOrGenerateInstallID() }()
//	<-entered                  // wait for goroutine 1 to reach the hook
//	go func() { ... }()        // start goroutine 2 contending the lock
//	release()                  // let goroutine 1 proceed
func firstWriterGate(t *testing.T, hookVar *func()) (<-chan struct{}, func()) {
	t.Helper()
	if *hookVar != nil {
		t.Fatal("firstWriterGate: hookVar already set — install at most one gate per test")
	}
	entered := make(chan struct{}, 1)
	gate := make(chan struct{})
	var releaseOnce sync.Once
	releaseGate := func() { releaseOnce.Do(func() { close(gate) }) }

	var calls atomic.Int32
	*hookVar = func() {
		if calls.Add(1) != 1 {
			return
		}
		entered <- struct{}{}
		<-gate
	}
	t.Cleanup(func() {
		releaseGate() // unblock parked writer if test failed before release()
		*hookVar = nil
	})
	return entered, releaseGate
}
