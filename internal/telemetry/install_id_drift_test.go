// install_id_drift_test.go — Tests covering CheckInstallIDDrift cadence,
// the SetInstallIDDriftLogFn registration race, and lineage persistence.
//
// Tests in this package must NOT use t.Parallel() due to shared package-level
// state.

package telemetry

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

// TestCheckInstallIDDrift_NoOpWhenStoredEqualsDerived confirms that when
// the persisted ID matches what we'd derive right now, CheckInstallIDDrift
// is silent — no log fires, no beacon emits.
func TestCheckInstallIDDrift_NoOpWhenStoredEqualsDerived(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	const knownID = "abcdef012345"
	prevReadMachine := readMachineID
	defer func() { readMachineID = prevReadMachine }()
	readMachineID = func() (string, bool) { return "stub-machine", true }

	id := GetInstallID() // generates via derive (no file yet)
	if id == "" {
		t.Fatal("GetInstallID returned empty")
	}

	var logged bool
	SetInstallIDDriftLogFn(func(stored, derived string) { logged = true })
	defer SetInstallIDDriftLogFn(nil)

	CheckInstallIDDrift()
	if logged {
		t.Errorf("drift logger fired when stored == derived")
	}
	_ = knownID
}

// TestCheckInstallIDDrift_FiresWhenDerivedChanges confirms that a stored ID
// (held over from a previous host) different from the current derivation
// triggers the drift logger AND persists install_id_lineage.
func TestCheckInstallIDDrift_FiresWhenDerivedChanges(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	const stored = "111111111111"
	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte(stored), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	prevReadMachine := readMachineID
	defer func() { readMachineID = prevReadMachine }()
	readMachineID = func() (string, bool) { return "new-machine", true }

	var fired int
	var loggedStored, loggedDerived string
	SetInstallIDDriftLogFn(func(s, d string) {
		fired++
		loggedStored, loggedDerived = s, d
	})
	defer SetInstallIDDriftLogFn(nil)

	CheckInstallIDDrift()
	if fired != 1 {
		t.Errorf("drift logger fired %d times, want 1", fired)
	}
	if loggedStored != stored {
		t.Errorf("logged stored = %q, want %q", loggedStored, stored)
	}
	if loggedDerived == "" || loggedDerived == stored {
		t.Errorf("logged derived = %q, want non-empty and != stored", loggedDerived)
	}

	CheckInstallIDDrift()
	if fired != 1 {
		t.Errorf("drift logger fired %d times after dedupe, want 1", fired)
	}

	lineage, err := os.ReadFile(filepath.Join(dir, "install_id_lineage"))
	if err != nil {
		t.Fatalf("lineage file missing: %v", err)
	}
	if string(lineage) != loggedDerived {
		t.Errorf("lineage = %q, want %q", string(lineage), loggedDerived)
	}
}

// TestSetInstallIDDriftLogFn_ConcurrentSetAndLoadIsRaceFree pins that the
// atomic.Pointer[func] pattern allows arbitrary interleaving of Set + Load
// without data races. 50 setters rotate between two distinct fn values
// while 50 loaders invoke whatever they observe; under -race -count=N
// this fails fast if the pattern regresses (e.g., someone replaces
// atomic.Pointer with a plain var).
//
// Loaders also tally side-effects via observably-distinct sentinel
// closures so the test fails if the loader saw a wrong-shape pointer
// (e.g., a regression that silently coerced the pointer to nil would
// leave both counters at 0). The pre-Set noop call before spawning loaders
// ensures the load path is GUARANTEED (not scheduler-dependent) to observe
// a non-nil fn at least once.
func TestSetInstallIDDriftLogFn_ConcurrentSetAndLoadIsRaceFree(t *testing.T) {
	t.Cleanup(func() { SetInstallIDDriftLogFn(nil) })

	const goroutines = 50
	const iterations = 200
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	var noopCalls atomic.Int64
	var otherCalls atomic.Int64
	noop := func(stored, derived string) { noopCalls.Add(1) }
	other := func(stored, derived string) { otherCalls.Add(1) }

	// Seed the registered fn so loaders observe a non-nil pointer
	// regardless of scheduler interleaving. The happens-before edge that
	// guarantees visibility is the `go` statement establishing the loader
	// goroutines AFTER this Set returns; SetInstallIDDriftLogFn's atomic
	// store ensures the value is durably published. Setters in the loop
	// also never store nil, so the non-nil invariant is preserved across
	// every observable state thereafter.
	SetInstallIDDriftLogFn(noop)

	for i := range goroutines {
		fn := noop
		if i%2 == 0 {
			fn = other
		}
		go func() {
			defer wg.Done()
			for range iterations {
				SetInstallIDDriftLogFn(fn)
			}
		}()
	}
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				if fn := loadInstallIDDriftLogFn(); fn != nil {
					fn("stub-stored", "stub-derived")
				}
			}
		}()
	}

	wg.Wait()

	if noopCalls.Load()+otherCalls.Load() == 0 {
		t.Fatal("no fn invocations recorded — loadInstallIDDriftLogFn always returned nil under contention")
	}
}
