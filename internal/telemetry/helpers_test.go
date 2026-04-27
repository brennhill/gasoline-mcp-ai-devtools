// helpers_test.go — Test-only helpers that manipulate package-level state
// for installation ID, beacon endpoint, and emission notifications.
// Compiled only under `go test`, so the production binary never ships these
// symbols.
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

// lockBudgetMu serializes concurrent mutation of installStateLockTimeout/
// Poll/Stale. Tests in this package don't use t.Parallel(), but a future
// `go test -count=N` run could overlap cleanups if -p>1 splits packages
// (cross-package the vars are isolated, but in-package multiple tests using
// the helper benefit from explicit serialization).
var lockBudgetMu sync.Mutex

// withLockBudget shrinks installStateLockTimeout/Poll/Stale for the duration
// of t. The originals are restored via t.Cleanup. Acquisition is gated by
// lockBudgetMu so two tests calling the helper in the same run cannot tear.
//
// CONSTRAINT: do NOT call from a subtest of a test that already called
// withLockBudget — the inner call will block forever on lockBudgetMu since
// the outer cleanup runs only after subtests complete. If you need nested
// budgets, call withLockBudget at the leaf-test level only.
func withLockBudget(t *testing.T, timeout, poll, stale time.Duration) {
	t.Helper()
	lockBudgetMu.Lock()
	origT := installStateLockTimeout
	origP := installStateLockPoll
	origS := installStateLockStale
	installStateLockTimeout = timeout
	installStateLockPoll = poll
	installStateLockStale = stale
	t.Cleanup(func() {
		installStateLockTimeout = origT
		installStateLockPoll = origP
		installStateLockStale = origS
		lockBudgetMu.Unlock()
	})
}

// homeDirFnMu serializes concurrent mutation of userHomeDirFn so two tests
// rotating the override cannot tear in the rare event of `go test -p>1`.
var homeDirFnMu sync.Mutex

// withHomeDirFn rotates userHomeDirFn for the duration of t and restores
// the original via t.Cleanup. Use this instead of `defer userHomeDirFn = orig`
// so a t.Fatal between override and restore cannot leak the override into
// the next test (Cleanup runs even on failure; bare defer doesn't help if
// the test author forgets to write it).
func withHomeDirFn(t *testing.T, fn func() (string, error)) {
	t.Helper()
	homeDirFnMu.Lock()
	orig := userHomeDirFn
	userHomeDirFn = fn
	t.Cleanup(func() {
		userHomeDirFn = orig
		homeDirFnMu.Unlock()
	})
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
