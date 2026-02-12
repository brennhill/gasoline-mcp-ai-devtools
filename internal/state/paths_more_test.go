package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRootDirFallsBackToUserConfigDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(StateDirEnv, "")
	t.Setenv(xdgStateHomeEnv, "")

	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("os.UserConfigDir() error = %v", err)
	}

	got, err := RootDir()
	if err != nil {
		t.Fatalf("RootDir() error = %v", err)
	}

	want := filepath.Join(configDir, appName)
	if got != want {
		t.Fatalf("RootDir() = %q, want %q", got, want)
	}
}

func TestPathHelpersAndLegacyPaths(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	t.Setenv(StateDirEnv, root)
	t.Setenv(xdgStateHomeEnv, "")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	logsDir, err := LogsDir()
	if err != nil {
		t.Fatalf("LogsDir() error = %v", err)
	}
	if want := filepath.Join(root, "logs"); logsDir != want {
		t.Fatalf("LogsDir() = %q, want %q", logsDir, want)
	}

	recordingsDir, err := RecordingsDir()
	if err != nil {
		t.Fatalf("RecordingsDir() error = %v", err)
	}
	if want := filepath.Join(root, "recordings"); recordingsDir != want {
		t.Fatalf("RecordingsDir() = %q, want %q", recordingsDir, want)
	}

	screenshotsDir, err := ScreenshotsDir()
	if err != nil {
		t.Fatalf("ScreenshotsDir() error = %v", err)
	}
	if want := filepath.Join(root, "screenshots"); screenshotsDir != want {
		t.Fatalf("ScreenshotsDir() = %q, want %q", screenshotsDir, want)
	}

	securityConfig, err := SecurityConfigFile()
	if err != nil {
		t.Fatalf("SecurityConfigFile() error = %v", err)
	}
	if want := filepath.Join(root, "security", "security.json"); securityConfig != want {
		t.Fatalf("SecurityConfigFile() = %q, want %q", securityConfig, want)
	}

	legacyCrash, err := LegacyCrashLogFile()
	if err != nil {
		t.Fatalf("LegacyCrashLogFile() error = %v", err)
	}
	if want := filepath.Join(home, "gasoline-crash.log"); legacyCrash != want {
		t.Fatalf("LegacyCrashLogFile() = %q, want %q", legacyCrash, want)
	}

	legacyPID, err := LegacyPIDFile(7890)
	if err != nil {
		t.Fatalf("LegacyPIDFile() error = %v", err)
	}
	if want := filepath.Join(home, ".gasoline-7890.pid"); legacyPID != want {
		t.Fatalf("LegacyPIDFile() = %q, want %q", legacyPID, want)
	}

	legacySettings, err := LegacySettingsFile()
	if err != nil {
		t.Fatalf("LegacySettingsFile() error = %v", err)
	}
	if want := filepath.Join(home, ".gasoline-settings.json"); legacySettings != want {
		t.Fatalf("LegacySettingsFile() = %q, want %q", legacySettings, want)
	}

	legacyRecordings, err := LegacyRecordingsDir()
	if err != nil {
		t.Fatalf("LegacyRecordingsDir() error = %v", err)
	}
	if want := filepath.Join(home, ".gasoline", "recordings"); legacyRecordings != want {
		t.Fatalf("LegacyRecordingsDir() = %q, want %q", legacyRecordings, want)
	}

	legacySecurity, err := LegacySecurityConfigFile()
	if err != nil {
		t.Fatalf("LegacySecurityConfigFile() error = %v", err)
	}
	if want := filepath.Join(home, ".gasoline", "security.json"); legacySecurity != want {
		t.Fatalf("LegacySecurityConfigFile() = %q, want %q", legacySecurity, want)
	}
}

func TestNormalizePath(t *testing.T) {
	t.Parallel()

	if _, err := normalizePath(""); err == nil {
		t.Fatal("normalizePath(\"\") should return error")
	}

	absInput := filepath.Join(string(os.PathSeparator), "tmp", "a", "..", "b")
	absGot, err := normalizePath(absInput)
	if err != nil {
		t.Fatalf("normalizePath(abs) error = %v", err)
	}
	if absGot != filepath.Clean(absInput) {
		t.Fatalf("normalizePath(abs) = %q, want %q", absGot, filepath.Clean(absInput))
	}

	relGot, err := normalizePath(filepath.Join(".", "x", "..", "y"))
	if err != nil {
		t.Fatalf("normalizePath(rel) error = %v", err)
	}
	if !filepath.IsAbs(relGot) {
		t.Fatalf("normalizePath(rel) = %q, want absolute path", relGot)
	}
}
