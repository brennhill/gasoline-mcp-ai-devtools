// install_id_concurrency_test.go — Tests covering the concurrent install_id
// load path: singleflight invariants, lock contention, four-location heal
// race convergence, and first-tool-call claim races.
//
// Tests in this package must NOT use t.Parallel() due to shared package-level
// state (kaboomDir, cachedInstallIDPtr, persist hooks, lock-budget vars).

package telemetry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetInstallID_ResetDuringInFlightLeaderDoesNotClobberSuccessor(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	originalReadMachineID := readMachineID
	readMachineID = func() (string, bool) { return "", false }
	defer func() { readMachineID = originalReadMachineID }()
	t.Cleanup(func() { installIDBeforePersistHook = nil })

	enteredA := make(chan struct{}, 1)
	releaseA := make(chan struct{})
	installIDBeforePersistHook = func() {
		enteredA <- struct{}{}
		<-releaseA
	}

	leaderAResult := make(chan string, 1)
	go func() { leaderAResult <- GetInstallID() }()

	select {
	case <-enteredA:
	case <-time.After(2 * time.Second):
		t.Fatal("leader A did not reach persist hook")
	}

	resetInstallIDState()

	sentinel := &installIDLoadOp{done: make(chan struct{})}
	installIDLoadMu.Lock()
	installIDLoadInFlight = sentinel
	installIDLoadMu.Unlock()

	close(releaseA)

	idA := <-leaderAResult
	if idA == "" {
		t.Fatal("leader A returned empty install ID")
	}

	installIDLoadMu.Lock()
	stillSentinel := installIDLoadInFlight == sentinel
	installIDLoadMu.Unlock()
	if !stillSentinel {
		t.Fatal("leader A's cleanup clobbered the successor's in-flight op pointer; singleflight invariant broken")
	}

	installIDLoadMu.Lock()
	installIDLoadInFlight = nil
	installIDLoadMu.Unlock()
	close(sentinel.done)
}

func TestLoadOrGenerateInstallID_ConcurrentFreshWritersShareOneInstallID(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	resetFirstToolCallState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	originalReadMachineID := readMachineID
	readMachineID = func() (string, bool) { return "", false }
	defer func() { readMachineID = originalReadMachineID }()

	entered, release := firstWriterGate(t, &installIDBeforePersistHook)

	firstResult := make(chan string, 1)
	secondResult := make(chan string, 1)

	go func() { firstResult <- loadOrGenerateInstallID() }()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first install_id writer did not reach persist hook")
	}

	go func() { secondResult <- loadOrGenerateInstallID() }()

	release()

	id1 := <-firstResult
	id2 := <-secondResult
	if id1 == "" || id2 == "" {
		t.Fatalf("loadOrGenerateInstallID() returned empty ids: %q, %q", id1, id2)
	}
	if id1 != id2 {
		t.Fatalf("concurrent fresh writers returned different install ids: %q vs %q", id1, id2)
	}

	data, err := os.ReadFile(filepath.Join(dir, "install_id"))
	if err != nil {
		t.Fatalf("failed to read persisted install_id: %v", err)
	}
	if got := string(data); got != id1 {
		t.Fatalf("persisted install_id = %q, want %q", got, id1)
	}
}

func TestClaimFirstToolCallInstallID_ConcurrentClaimsEmitOnce(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	resetFirstToolCallState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	installID := "aabbccddeeff"

	entered, release := firstWriterGate(t, &firstToolCallBeforePersistHook)

	type claimResult struct {
		claimed bool
		err     error
	}
	firstResult := make(chan claimResult, 1)
	secondResult := make(chan claimResult, 1)

	go func() {
		claimed, err := claimFirstToolCallInstallID(installID)
		firstResult <- claimResult{claimed: claimed, err: err}
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first first_tool_call claim did not reach persist hook")
	}

	go func() {
		claimed, err := claimFirstToolCallInstallID(installID)
		secondResult <- claimResult{claimed: claimed, err: err}
	}()

	release()

	first := <-firstResult
	second := <-secondResult
	if first.err != nil {
		t.Fatalf("first claimFirstToolCallInstallID() error = %v", first.err)
	}
	if second.err != nil {
		t.Fatalf("second claimFirstToolCallInstallID() error = %v", second.err)
	}
	if !first.claimed && !second.claimed {
		t.Fatal("claimFirstToolCallInstallID() never claimed the first_tool_call marker")
	}
	if first.claimed == second.claimed {
		t.Fatalf("expected exactly one successful first_tool_call claim, got first=%v second=%v", first.claimed, second.claimed)
	}

	data, err := os.ReadFile(filepath.Join(dir, "first_tool_call_install_id"))
	if err != nil {
		t.Fatalf("failed to read persisted first_tool_call marker: %v", err)
	}
	if got := string(data); got != installID {
		t.Fatalf("persisted first_tool_call marker = %q, want %q", got, installID)
	}
}

// TestLoadOrGenerateInstallID_ConcurrentFourLocationWritersConverge
// extends the original concurrent-writers test to the new four-location
// world. The persist hook blocks the first goroutine inside the slow path
// while the second goroutine starts and contends for the install_id.lock,
// guaranteeing the lock + re-check path is exercised (not the fast read
// path). The lock guarantees both share one ID, AND the heal pass
// propagates that ID to ALL four locations (primary + .bak + secondary + .bak).
func TestLoadOrGenerateInstallID_ConcurrentFourLocationWritersConverge(t *testing.T) {
	primary := t.TempDir()
	secondary := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(primary)
	overrideSecondaryDir(secondary)
	defer resetKaboomDir()

	originalReadMachineID := readMachineID
	readMachineID = func() (string, bool) { return "", false }
	defer func() { readMachineID = originalReadMachineID }()

	entered, release := firstWriterGate(t, &installIDBeforePersistHook)

	firstResult := make(chan string, 1)
	secondResult := make(chan string, 1)

	go func() { firstResult <- loadOrGenerateInstallID() }()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first install_id writer did not reach persist hook")
	}

	go func() { secondResult <- loadOrGenerateInstallID() }()

	release()

	id1 := <-firstResult
	id2 := <-secondResult
	if id1 == "" || id2 == "" {
		t.Fatalf("loadOrGenerateInstallID() returned empty: id1=%q id2=%q", id1, id2)
	}
	if id1 != id2 {
		t.Fatalf("concurrent writers diverged: id1=%q id2=%q", id1, id2)
	}

	for _, p := range []string{
		filepath.Join(primary, "install_id"),
		filepath.Join(primary, "install_id.bak"),
		filepath.Join(secondary, "install_id"),
		filepath.Join(secondary, "install_id.bak"),
	} {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("location %s missing after heal: %v", p, err)
			continue
		}
		if string(got) != id1 {
			t.Errorf("location %s = %q, want %q", p, string(got), id1)
		}
	}
}
