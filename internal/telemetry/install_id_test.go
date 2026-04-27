// install_id_test.go — Tests for random install ID generation and persistence.
// Tests in this package must NOT use t.Parallel() due to shared package-level state.

package telemetry

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

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

// TestGetInstallID_ResetDuringInFlightLeaderDoesNotClobberSuccessor pins
// the singleflight invariant: when resetInstallIDState fires while a
// leader is mid-I/O, the leader's post-load cleanup must NOT clear the
// successor op installed by the next caller. Otherwise two leaders run
// the slow path concurrently — the very condition singleflight prevents.
//
// Implementation: leader A is parked inside the persist hook. While A is
// parked we (a) reset state, (b) install a sentinel op pointer in
// installIDLoadInFlight directly to simulate the successor B that has
// taken the in-memory mutex but is now blocked on the file lock A still
// holds. Releasing A then exercises A's post-load cleanup. Without the
// `if installIDLoadInFlight == op { … }` guard, A would clobber the
// sentinel; with the guard, the sentinel survives.
// Concurrency tests
// (TestGetInstallID_ResetDuringInFlightLeaderDoesNotClobberSuccessor,
// TestLoadOrGenerateInstallID_ConcurrentFreshWritersShareOneInstallID,
// TestClaimFirstToolCallInstallID_ConcurrentClaimsEmitOnce,
// TestLoadOrGenerateInstallID_ConcurrentFourLocationWritersConverge)
// live in install_id_concurrency_test.go.

// TestSecondaryKaboomDir_HomeFailureBranches covers the wrapper's two
// failure modes that the pure resolver can't reach: UserHomeDir returning
// an error, and UserHomeDir returning empty/whitespace. Both must fall
// through to "" so a daemon on a misconfigured host doesn't crash.
func TestSecondaryKaboomDir_HomeFailureBranches(t *testing.T) {
	prevFn := userHomeDirFn
	prevDisabled := secondaryDirDisabled
	prevOverride := secondaryDirOverride
	defer func() {
		userHomeDirFn = prevFn
		secondaryDirDisabled = prevDisabled
		secondaryDirOverride = prevOverride
	}()
	secondaryDirDisabled = false
	secondaryDirOverride = ""

	userHomeDirFn = func() (string, error) { return "", os.ErrNotExist }
	if got := secondaryKaboomDir(); got != "" {
		t.Errorf("secondaryKaboomDir() with errored home = %q, want \"\"", got)
	}

	userHomeDirFn = func() (string, error) { return "   ", nil }
	if got := secondaryKaboomDir(); got != "" {
		t.Errorf("secondaryKaboomDir() with whitespace home = %q, want \"\"", got)
	}

	userHomeDirFn = func() (string, error) { return "", nil }
	if got := secondaryKaboomDir(); got != "" {
		t.Errorf("secondaryKaboomDir() with empty home = %q, want \"\"", got)
	}
}

// TestSecondaryKaboomDirForOS covers the cross-platform branches of the
// secondary mirror resolver. Without this table-driven test, only the host
// CI's GOOS branch was reachable, leaving linux XDG / windows LOCALAPPDATA
// fallbacks dead-code on darwin runners.
//
// Path separators are produced by filepath.Join (host-OS dependent), so the
// expected paths use filepath.Join too — we are pinning the choice of path
// COMPONENTS, not the rendering. (On Windows hosts the windows branch's
// `filepath.Join` produces backslashes; on darwin/linux hosts it produces
// forward slashes. Either way the same components must appear.)
func TestSecondaryKaboomDirForOS(t *testing.T) {
	noEnv := func(string) string { return "" }
	xdgEnv := func(k string) string {
		if k == "XDG_STATE_HOME" {
			return filepath.Join("/", "custom", "xdg")
		}
		return ""
	}
	// Distinct LOCALAPPDATA root so the LOCALAPPDATA branch and the
	// fallback branch produce DIFFERENT paths — otherwise a regression
	// that swaps the two branches' bodies would be undetectable.
	winLocalAppData := filepath.Join("D:", "CustomAppData")
	winEnv := func(k string) string {
		if k == "LOCALAPPDATA" {
			return winLocalAppData
		}
		return ""
	}

	homeUnix := filepath.Join("/", "home", "test")
	homeMac := filepath.Join("/", "Users", "test")
	homeWin := filepath.Join("C:", "Users", "test")

	cases := []struct {
		name string
		goos string
		home string
		env  func(string) string
		want string
	}{
		{"darwin", "darwin", homeMac, noEnv,
			filepath.Join(homeMac, "Library", "Application Support", "Kaboom")},
		{"linux default", "linux", homeUnix, noEnv,
			filepath.Join(homeUnix, ".local", "state", "kaboom")},
		{"linux XDG_STATE_HOME", "linux", homeUnix, xdgEnv,
			filepath.Join(filepath.Join("/", "custom", "xdg"), "kaboom")},
		{"windows LOCALAPPDATA", "windows", homeWin, winEnv,
			filepath.Join(winLocalAppData, "Kaboom")},
		{"windows fallback", "windows", homeWin, noEnv,
			filepath.Join(homeWin, "AppData", "Local", "Kaboom")},
		{"unsupported GOOS", "plan9", filepath.Join("/", "usr", "test"), noEnv, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := secondaryKaboomDirForOS(tc.goos, tc.home, tc.env)
			if got != tc.want {
				t.Errorf("secondaryKaboomDirForOS(%q, %q, ...) = %q, want %q",
					tc.goos, tc.home, got, tc.want)
			}
		})
	}
}

// Locking tests (TestWithKaboomStateLock_TimeoutExhausted,
// TestWithKaboomStateLock_StaleLockReclaimed) live in install_id_locking_test.go.

// TestMarkFirstToolCallEmittedForInstall_ClaimsAndDedupesInverse pins the
// other half of the cache state machine: when the marker file does NOT
// exist, the first call returns true (claim succeeded), and the second
// call returns false (already emitted, served from cache without disk re-read).
// Together with the ...CacheSticksAcrossFileDeletion sibling, this pins both
// directions of the cachedFirstToolCallLoaded contract.
func TestMarkFirstToolCallEmittedForInstall_ClaimsAndDedupesInverse(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	resetFirstToolCallState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	originalReadMachineID := readMachineID
	readMachineID = func() (string, bool) { return "stub-machine", true }
	defer func() { readMachineID = originalReadMachineID }()

	if got := markFirstToolCallEmittedForInstall(); !got {
		t.Fatal("first call returned false; expected true (no marker, claim should succeed)")
	}
	if got := markFirstToolCallEmittedForInstall(); got {
		t.Fatal("second call returned true; expected false (already emitted)")
	}
}

// TestMarkFirstToolCallEmittedForInstall_CacheSticksAcrossFileDeletion pins
// the once-load semantics of cachedFirstToolCallLoaded: after the in-memory
// cache is hydrated from disk, deleting the marker file does NOT cause a
// re-read on the next call. The test seeds the marker matching the current
// install ID (so the first call sees "already emitted"), deletes the file,
// then verifies the second call still returns false. If the cache flag did
// not stick, the second call would re-read disk → find no marker → attempt
// to claim → return true.
func TestMarkFirstToolCallEmittedForInstall_CacheSticksAcrossFileDeletion(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	resetFirstToolCallState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	originalReadMachineID := readMachineID
	readMachineID = func() (string, bool) { return "stub-machine", true }
	defer func() { readMachineID = originalReadMachineID }()

	installID := GetInstallID()
	if installID == "" {
		t.Fatal("GetInstallID returned empty")
	}

	if err := os.WriteFile(
		filepath.Join(dir, "first_tool_call_install_id"),
		[]byte(installID),
		0o600,
	); err != nil {
		t.Fatalf("seed marker: %v", err)
	}

	if got := markFirstToolCallEmittedForInstall(); got {
		t.Fatal("first call returned true; expected false because the seeded marker matches install ID")
	}

	if err := os.Remove(filepath.Join(dir, "first_tool_call_install_id")); err != nil {
		t.Fatalf("delete marker: %v", err)
	}

	if got := markFirstToolCallEmittedForInstall(); got {
		t.Fatal("second call returned true after file deletion; cachedFirstToolCallLoaded did not gate the disk read")
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

// Drift tests (TestCheckInstallIDDrift_NoOpWhenStoredEqualsDerived,
// TestCheckInstallIDDrift_FiresWhenDerivedChanges,
// TestSetInstallIDDriftLogFn_ConcurrentSetAndLoadIsRaceFree) live in
// install_id_drift_test.go.
