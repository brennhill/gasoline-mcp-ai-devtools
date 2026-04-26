// install_id_drift.go — Drift detection + lineage migration beacon for the
// install ID. Storage and derivation primitives live in install_id.go; this
// file owns the post-load comparison that detects host renames / machine_id
// changes and emits one observability beacon per actual identity change.
//
// Metrics emitted from this file:
//   - SetInstallIDDriftLogFn callback                  — fires once per new
//     derived ID seen, surfacing the (stored, derived) pair to the lifecycle
//     log so operators can stitch a single install across rename events.
//   - telemetry.AppError("install_id_migrated", ...)   — fires alongside the
//     callback. Carries the new derived_iid so analytics can link the
//     pre/post identity at ingest time. Cadence-bounded by
//     ~/.kaboom/install_id_lineage so a permanently-renamed host emits
//     exactly once across all subsequent daemon starts.
//
// Wire contract: docs/core/app-metrics.md.

package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// installIDDriftLogFn is fired by CheckInstallIDDrift when the persisted
// install ID differs from the deterministic derivation. atomic.Pointer
// gives lock-free reads on the hot path and serializes the write-once
// registration without an RWMutex.
var installIDDriftLogFn atomic.Pointer[func(stored, derived string)]

// SetInstallIDDriftLogFn registers a callback the drift checker invokes
// when stored and derived IDs disagree (e.g., hostname change). Stored wins
// regardless — this is observability only.
//
// Call ONCE, BEFORE telemetry.CheckInstallIDDrift. Re-set is allowed (last
// wins) but discouraged: the migration beacon path also runs from
// CheckInstallIDDrift, so a late Set silently misses the first emission.
// Pass nil to clear.
func SetInstallIDDriftLogFn(fn func(stored, derived string)) {
	if fn == nil {
		installIDDriftLogFn.Store(nil)
		return
	}
	installIDDriftLogFn.Store(&fn)
}

func loadInstallIDDriftLogFn() func(stored, derived string) {
	p := installIDDriftLogFn.Load()
	if p == nil {
		return nil
	}
	return *p
}

// HasInstallIDDriftLogFnForTest reports whether SetInstallIDDriftLogFn has
// installed a non-nil callback. The "ForTest" suffix marks it as a test-only
// introspection seam — production code has no business reading registration
// state and should treat this function as if it did not exist. The function
// must remain on the public API only because cross-package tests in
// cmd/browser-agent cannot link against unexported symbols and Go's
// export_test.go pattern only crosses the same-package external test
// boundary, not arbitrary downstream packages.
func HasInstallIDDriftLogFnForTest() bool {
	return loadInstallIDDriftLogFn() != nil
}

// CheckInstallIDDrift fires the registered drift logger AND emits an
// `install_id_migrated` app_error beacon when the stored ID differs from
// the deterministic derivation. Stored always wins as the install identity
// — the beacon exists so analytics can link a hostname/uid/machine change
// (which would produce a new derived ID) back to the original install,
// preserving the single-install lineage.
//
// MUST be called AFTER Warm() (or after at least one GetInstallID call).
// Running inside the GetInstallID load path would re-enter buildEnvelope →
// GetInstallID and deadlock; this synchronous, post-Warm form removes both
// the deadlock and the goroutine-race hazard the prior implementation had.
//
// Cadence: a previously-seen derived ID is persisted at
// `~/.kaboom/install_id_lineage`. The beacon fires only when the current
// derivation differs from the last persisted one — so a permanently-renamed
// host emits exactly one beacon across all subsequent daemon starts, not one
// per process.
//
// Concurrency: the drift log fn is snapshotted exactly once at function
// entry, so a concurrent SetInstallIDDriftLogFn affects only the NEXT call.
// The AppError beacon and lineage persist run unconditionally after the
// snapshot — so if a caller registered a logger but rotated to nil mid-call,
// the original logger still receives this drift event AND the beacon fires.
// The cadence guard above prevents the beacon from re-emitting for the same
// derivation across daemon starts.
func CheckInstallIDDrift() {
	stored := GetInstallID()
	if stored == "" {
		return
	}
	derived, ok := deriveInstallID()
	if !ok || derived == stored {
		return
	}
	if last := readLastDerivedSeen(); last == derived {
		return
	}

	logFnSnapshot := loadInstallIDDriftLogFn() // single load — see comment above.
	if logFnSnapshot != nil {
		logFnSnapshot(stored, derived)
	}
	AppError("install_id_migrated", map[string]string{
		"derived_iid": derived,
	})
	persistLastDerivedSeen(derived)
}

// readLastDerivedSeen returns the last derived ID we beaconed for, or "".
// Used to dedupe the migration beacon across daemon starts.
func readLastDerivedSeen() string {
	if strings.TrimSpace(kaboomDir) == "" {
		return ""
	}
	return tryReadValidID(filepath.Join(kaboomDir, "install_id_lineage"))
}

// persistLastDerivedSeen records derived as the most-recently-emitted
// migration target. Best-effort; failure to persist means the next daemon
// start may re-emit the beacon (acceptable — analytics dedupes by lineage).
func persistLastDerivedSeen(derived string) {
	if strings.TrimSpace(kaboomDir) == "" {
		return
	}
	_ = os.MkdirAll(kaboomDir, 0o700)
	_ = writeTokenAtomic(filepath.Join(kaboomDir, "install_id_lineage"), derived)
}
