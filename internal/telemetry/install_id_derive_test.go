// install_id_derive_test.go — Tests for HMAC-derived install ID fallback,
// atomic persistence, and self-heal on corrupted/wiped id files.
// Tests in this package must NOT use t.Parallel() due to shared package-level state.

package telemetry

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDeriveInstallID_Stable(t *testing.T) {
	a := deriveInstallIDFromParts("machine-abc", "1000", "host1")
	b := deriveInstallIDFromParts("machine-abc", "1000", "host1")
	if a != b {
		t.Fatalf("derive not stable: %q vs %q", a, b)
	}
}

func TestDeriveInstallID_HexFormat(t *testing.T) {
	id := deriveInstallIDFromParts("machine-abc", "1000", "host1")
	if !hexPattern.MatchString(id) {
		t.Fatalf("derived id %q does not match ^[0-9a-f]{12}$", id)
	}
}

func TestDeriveInstallID_DiffersByUser(t *testing.T) {
	a := deriveInstallIDFromParts("machine-abc", "1000", "host1")
	b := deriveInstallIDFromParts("machine-abc", "1001", "host1")
	if a == b {
		t.Fatalf("derived id did not change with uid: %q == %q", a, b)
	}
}

func TestDeriveInstallID_DiffersByMachine(t *testing.T) {
	a := deriveInstallIDFromParts("machine-abc", "1000", "host1")
	b := deriveInstallIDFromParts("machine-xyz", "1000", "host1")
	if a == b {
		t.Fatalf("derived id did not change with machineID: %q == %q", a, b)
	}
}

func TestDeriveInstallID_DiffersByHostname(t *testing.T) {
	a := deriveInstallIDFromParts("machine-abc", "1000", "host1")
	b := deriveInstallIDFromParts("machine-abc", "1000", "host2")
	if a == b {
		t.Fatalf("derived id did not change with hostname: %q == %q", a, b)
	}
}

// stubMachineID replaces readMachineID for the duration of the test.
func stubMachineID(t *testing.T, id string, ok bool) {
	t.Helper()
	prev := readMachineID
	readMachineID = func() (string, bool) { return id, ok }
	t.Cleanup(func() { readMachineID = prev })
}

func TestLoadOrGenerateInstallID_WipedFileRecoversViaDerive(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Force a deterministic, non-OS machineID so we can compare.
	stubMachineID(t, "deterministic-machine-id", true)

	// First call: nothing on disk → derive path populates the file.
	id1 := GetInstallID()
	if !hexPattern.MatchString(id1) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id1)
	}

	idPath := filepath.Join(dir, "install_id")
	data, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatalf("install_id file not persisted: %v", err)
	}
	if string(data) != id1 {
		t.Fatalf("persisted id = %q, want %q", string(data), id1)
	}

	// Wipe the file (simulates ~/.kaboom/ being deleted or VM snapshot rollback).
	if err := os.Remove(idPath); err != nil {
		t.Fatalf("failed to remove install_id: %v", err)
	}

	// Second call: file gone, but derive returns the same id.
	resetInstallIDState()
	id2 := GetInstallID()
	if id2 != id1 {
		t.Fatalf("post-wipe derive recovery failed: got %q, want %q", id2, id1)
	}

	// And it was re-persisted.
	data2, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatalf("install_id file not re-persisted: %v", err)
	}
	if string(data2) != id1 {
		t.Fatalf("re-persisted id = %q, want %q", string(data2), id1)
	}
}

func TestLoadOrGenerateInstallID_WipedFileFallsBackToRandomWhenDeriveFails(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	// Machine id lookup fails → fall through to random.
	stubMachineID(t, "", false)

	id := GetInstallID()
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id)
	}

	data, err := os.ReadFile(filepath.Join(dir, "install_id"))
	if err != nil {
		t.Fatalf("install_id file not persisted: %v", err)
	}
	if string(data) != id {
		t.Fatalf("persisted id = %q, want %q", string(data), id)
	}
}

func TestWriteInstallIDAtomic_NoPartialWrite(t *testing.T) {
	dir := t.TempDir()
	idPath := filepath.Join(dir, "install_id")
	if err := writeInstallIDAtomic(idPath, "aabbccddeeff"); err != nil {
		t.Fatalf("writeInstallIDAtomic: %v", err)
	}

	info, err := os.Stat(idPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// Content is exact.
	data, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "aabbccddeeff" {
		t.Fatalf("content = %q, want %q", string(data), "aabbccddeeff")
	}

	// Permissions are 0o600 (skip strict perm check on Windows which has different semantics).
	if runtime.GOOS != "windows" {
		perm := info.Mode().Perm()
		if perm != 0o600 {
			t.Fatalf("perm = %o, want %o", perm, 0o600)
		}
	}

	// No stray temp files left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if name == "install_id" {
			continue
		}
		t.Fatalf("unexpected leftover entry %q in dir after atomic write", name)
	}
}

func TestWriteInstallIDAtomic_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	idPath := filepath.Join(dir, "install_id")
	if err := os.WriteFile(idPath, []byte("111111111111"), 0o600); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	if err := writeInstallIDAtomic(idPath, "222222222222"); err != nil {
		t.Fatalf("writeInstallIDAtomic: %v", err)
	}
	data, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "222222222222" {
		t.Fatalf("content = %q, want %q", string(data), "222222222222")
	}
}

func TestLoadOrGenerateInstallID_WhitespaceFileSelfHeals(t *testing.T) {
	dir := t.TempDir()
	resetInstallIDState()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	idPath := filepath.Join(dir, "install_id")
	if err := os.WriteFile(idPath, []byte("   \n"), 0o600); err != nil {
		t.Fatalf("seed whitespace: %v", err)
	}

	id := GetInstallID()
	if id == "" {
		t.Fatal("GetInstallID() = \"\", want non-empty id after self-heal")
	}
	if !hexPattern.MatchString(id) {
		t.Fatalf("GetInstallID() = %q, want 12-char hex string", id)
	}

	// File now contains that id (not whitespace).
	data, err := os.ReadFile(idPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got := string(data); got != id {
		t.Fatalf("file contents = %q, want %q (self-heal must overwrite whitespace)", got, id)
	}
}

func TestParseIORegIOPlatformUUID(t *testing.T) {
	sample := `
+-o Root  <class IORegistryEntry, id 0x100000100, retain 43>
  +-o MacBookPro18,2  <class IOPlatformExpertDevice>
      "IOPlatformUUID" = "D5C17F5C-41A1-546A-A49B-6280B33B1419"
      "IOPlatformSerialNumber" = "XYZ"
`
	id, ok := parseIORegIOPlatformUUID(sample)
	if !ok {
		t.Fatal("parseIORegIOPlatformUUID: ok=false")
	}
	if id != "D5C17F5C-41A1-546A-A49B-6280B33B1419" {
		t.Fatalf("id = %q", id)
	}

	if _, ok := parseIORegIOPlatformUUID("no uuid here"); ok {
		t.Fatal("expected ok=false for unrelated output")
	}
}

func TestParseWindowsMachineGUID(t *testing.T) {
	sample := "\r\nHKEY_LOCAL_MACHINE\\SOFTWARE\\Microsoft\\Cryptography\r\n    MachineGuid    REG_SZ    abc-123-def\r\n"
	id, ok := parseWindowsMachineGUID(sample)
	if !ok {
		t.Fatal("parseWindowsMachineGUID: ok=false")
	}
	if id != "abc-123-def" {
		t.Fatalf("id = %q", id)
	}

	if _, ok := parseWindowsMachineGUID("no guid"); ok {
		t.Fatal("expected ok=false for unrelated output")
	}
}

