// install_id_test.go — Tests for random install ID generation and persistence.
// Tests in this package must NOT use t.Parallel() due to shared package-level state.

package telemetry

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

var hexPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

func TestGetInstallID_GeneratesOnFirstCall(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id)
	}
}

func TestGetInstallID_PersistsAcrossCalls(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id1 := GetInstallID()
	id2 := GetInstallID()
	if id1 != id2 {
		t.Fatalf("GetInstallID() returned different values: %q vs %q", id1, id2)
	}
}

func TestGetInstallID_StableAcrossParallelRuntimeStateDirsForSameHome(t *testing.T) {
	home := t.TempDir()
	firstRuntimeStateDir := filepath.Join(t.TempDir(), "parallel", "run-1001")
	secondRuntimeStateDir := filepath.Join(t.TempDir(), "parallel", "run-2002")

	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(state.StateDirEnv, firstRuntimeStateDir)
	resetInstallIDState()
	resetKaboomDir()
	defer func() {
		resetInstallIDState()
		resetKaboomDir()
	}()

	root1, err := state.RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}
	if root1 != firstRuntimeStateDir {
		t.Fatalf("RootDir() = %q, want %q", root1, firstRuntimeStateDir)
	}

	id1 := GetInstallID()
	if !hexPattern.MatchString(id1) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id1)
	}

	t.Setenv(state.StateDirEnv, secondRuntimeStateDir)
	resetInstallIDState()
	resetKaboomDir()

	root2, err := state.RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}
	if root2 != secondRuntimeStateDir {
		t.Fatalf("RootDir() = %q, want %q", root2, secondRuntimeStateDir)
	}
	if root1 == root2 {
		t.Fatalf("runtime state dirs should differ across parallel startups, both were %q", root1)
	}

	id2 := GetInstallID()
	if id1 != id2 {
		t.Fatalf("GetInstallID() changed across parallel runtime state dirs: %q vs %q", id1, id2)
	}

	data, err := os.ReadFile(filepath.Join(home, ".kaboom", "install_id"))
	if err != nil {
		t.Fatalf("failed to read persisted install_id: %v", err)
	}
	if got := string(data); got != id1 {
		t.Fatalf("persisted install_id = %q, want %q", got, id1)
	}
}

func TestGetInstallID_ReadsFromFile(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Pre-write a known ID file.
	knownID := "aabbccddeeff"
	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte(knownID), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != knownID {
		t.Fatalf("GetInstallID() = %q, want %q (from file)", id, knownID)
	}
}

func TestGetInstallID_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", ".kaboom")
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id)
	}

	// Verify directory was created.
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("strum dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("strum dir path is not a directory")
	}

	// Verify file was written.
	data, err := os.ReadFile(filepath.Join(dir, "install_id"))
	if err != nil {
		t.Fatalf("install_id file not written: %v", err)
	}
	if string(data) != id {
		t.Fatalf("file content = %q, want %q", string(data), id)
	}
}

// #7: Install ID file with trailing newline should be trimmed.
func TestGetInstallID_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Write ID with trailing newline (common from echo "id" > file).
	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte("aabbccddeeff\n"), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != "aabbccddeeff" {
		t.Fatalf("GetInstallID() = %q, want %q (should trim whitespace)", id, "aabbccddeeff")
	}
}

// #7: Install ID with spaces and carriage return should be trimmed.
func TestGetInstallID_TrimsCarriageReturn(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte("  aabbccddeeff\r\n"), 0600); err != nil {
		t.Fatalf("failed to write test install_id: %v", err)
	}

	id := GetInstallID()
	if id != "aabbccddeeff" {
		t.Fatalf("GetInstallID() = %q, want %q", id, "aabbccddeeff")
	}
}

func TestGetInstallID_ReadFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 not effective on Windows")
	}

	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Create a directory where the install_id file would be, making ReadFile fail.
	idPath := filepath.Join(dir, "install_id")
	if err := os.Mkdir(idPath, 0000); err != nil {
		t.Fatalf("failed to create blocking dir: %v", err)
	}
	defer os.Chmod(idPath, 0700) // cleanup

	id := GetInstallID()
	if id != "" {
		t.Fatalf("GetInstallID() = %q, want empty string on read failure", id)
	}
}

func TestGetInstallID_WriteFailureReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	blockedPath := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	resetInstallIDState()
	overrideKaboomDir(blockedPath)
	defer resetKaboomDir()

	id := GetInstallID()
	if id != "" {
		t.Fatalf("GetInstallID() = %q, want empty string when install ID cannot be persisted", id)
	}
}

func TestMarkFirstToolCallEmittedForInstall_NoStableInstallID(t *testing.T) {
	dir := t.TempDir()
	blockedPath := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blockedPath, []byte("not a directory"), 0600); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	resetInstallIDState()
	resetFirstToolCallState()
	overrideKaboomDir(blockedPath)
	defer resetKaboomDir()

	if markFirstToolCallEmittedForInstall() {
		t.Fatal("markFirstToolCallEmittedForInstall() = true, want false without a stable install ID")
	}
}

func TestLoadOrGenerateInstallID_ConcurrentFreshWritersShareOneInstallID(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	resetFirstToolCallState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	originalReadMachineID := readMachineID
	readMachineID = func() (string, bool) {
		return "", false
	}
	defer func() {
		readMachineID = originalReadMachineID
	}()

	entered, release := firstWriterGate(t, &installIDBeforePersistHook)

	firstResult := make(chan string, 1)
	secondResult := make(chan string, 1)

	go func() {
		firstResult <- loadOrGenerateInstallID()
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first install_id writer did not reach persist hook")
	}

	go func() {
		secondResult <- loadOrGenerateInstallID()
	}()

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

// TestInstallID_RejectsCorruptedFile pins format validation: a non-hex
// content in the primary file is treated as missing and the loader
// re-derives instead of returning the garbage as a usable ID.
func TestInstallID_RejectsCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	if err := os.WriteFile(filepath.Join(dir, "install_id"), []byte("not a valid id\n"), 0o600); err != nil {
		t.Fatalf("seed corrupt file: %v", err)
	}

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex (corrupt file should be ignored)", id)
	}
}

// TestInstallID_ReadsFromBackupWhenPrimaryGone pins the .bak fallback:
// if the primary file is deleted but the backup is intact, the loader
// returns the backup value and re-heals the primary.
func TestInstallID_ReadsFromBackupWhenPrimaryGone(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	const known = "abcdef012345"
	if err := os.WriteFile(filepath.Join(dir, "install_id.bak"), []byte(known), 0o600); err != nil {
		t.Fatalf("seed bak: %v", err)
	}

	id := GetInstallID()
	if id != known {
		t.Fatalf("GetInstallID() = %q, want %q (from .bak)", id, known)
	}
	primary, err := os.ReadFile(filepath.Join(dir, "install_id"))
	if err != nil {
		t.Fatalf("primary should be self-healed: %v", err)
	}
	if string(primary) != known {
		t.Fatalf("primary = %q, want %q after heal", string(primary), known)
	}
}

// TestInstallID_ReadsFromSecondaryWhenPrimaryDirGone pins the cross-location
// mirror: if the primary directory has nothing usable but the secondary
// holds a valid ID, the loader recovers it and heals the primary.
func TestInstallID_ReadsFromSecondaryWhenPrimaryDirGone(t *testing.T) {
	primary := t.TempDir()
	secondary := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(primary)
	overrideSecondaryDir(secondary)
	defer resetKaboomDir()

	const known = "0123456789ab"
	if err := os.WriteFile(filepath.Join(secondary, "install_id"), []byte(known), 0o600); err != nil {
		t.Fatalf("seed secondary: %v", err)
	}

	id := GetInstallID()
	if id != known {
		t.Fatalf("GetInstallID() = %q, want %q (from secondary)", id, known)
	}
	healed, err := os.ReadFile(filepath.Join(primary, "install_id"))
	if err != nil || string(healed) != known {
		t.Fatalf("primary should be healed from secondary; got data=%q err=%v", string(healed), err)
	}
}

// TestInstallIDLocationsPriorityOrder pins the read-priority contract
// [primary, primary.bak, secondary, secondary.bak]. Reordering this list
// silently changes which file wins; a test that seeds distinct valid IDs
// at each location and asserts primary wins guards against that.
func TestInstallIDLocationsPriorityOrder(t *testing.T) {
	primary := t.TempDir()
	secondary := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(primary)
	overrideSecondaryDir(secondary)
	defer resetKaboomDir()

	const primaryID = "111111111111"
	const primaryBakID = "222222222222"
	const secondaryID = "333333333333"
	const secondaryBakID = "444444444444"

	if err := os.WriteFile(filepath.Join(primary, "install_id"), []byte(primaryID), 0o600); err != nil {
		t.Fatalf("seed primary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(primary, "install_id.bak"), []byte(primaryBakID), 0o600); err != nil {
		t.Fatalf("seed primary.bak: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondary, "install_id"), []byte(secondaryID), 0o600); err != nil {
		t.Fatalf("seed secondary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secondary, "install_id.bak"), []byte(secondaryBakID), 0o600); err != nil {
		t.Fatalf("seed secondary.bak: %v", err)
	}

	got := GetInstallID()
	if got != primaryID {
		t.Fatalf("priority order broken: got %q, want primary %q", got, primaryID)
	}
}

// TestValidInstallID covers the format-rejection branches table-driven so
// new corruption modes only need a row, not a new test.
func TestValidInstallID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"happy 12 lowercase hex", "abcdef012345", true},
		{"empty string", "", false},
		{"11 chars", "abcdef01234", false},
		{"13 chars", "abcdef0123456", false},
		{"uppercase", "AABBCCDDEEFF", false},
		{"mixed case", "AaBbCcDdEeFf", false},
		{"non-hex shaped", "ggggggggggg1", false},
		{"with whitespace", " bcdef0123456", false},
		{"trailing newline (untrimmed)", "abcdef012345\n", false},
		{"unicode", "abcdef01234é", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validInstallID(tt.in); got != tt.want {
				t.Errorf("validInstallID(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestCheckInstallIDDrift_NoOpWhenStoredEqualsDerived confirms that when
// the persisted ID matches what we'd derive right now, CheckInstallIDDrift
// is silent — no log fires, no beacon emits.
func TestCheckInstallIDDrift_NoOpWhenStoredEqualsDerived(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Force derivation to return the same value the stored file has.
	const knownID = "abcdef012345"
	prevReadMachine := readMachineID
	defer func() { readMachineID = prevReadMachine }()
	// We need machine_id such that derive == knownID. Easier: seed the
	// file with what derive will produce, then check no drift logged.
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

	// Force derive to yield a different value.
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

	// Cadence guard: second call with same derivation should NOT re-fire.
	CheckInstallIDDrift()
	if fired != 1 {
		t.Errorf("drift logger fired %d times after dedupe, want 1", fired)
	}

	// Verify lineage file was persisted.
	lineage, err := os.ReadFile(filepath.Join(dir, "install_id_lineage"))
	if err != nil {
		t.Fatalf("lineage file missing: %v", err)
	}
	if string(lineage) != loggedDerived {
		t.Errorf("lineage = %q, want %q", string(lineage), loggedDerived)
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

	// All four locations must hold the same valid ID after heal.
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
