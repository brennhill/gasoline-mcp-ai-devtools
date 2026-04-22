// screenshot_cleanup_test.go — Tests for disk-bound screenshot retention.
// Screenshots accumulate indefinitely without maintenance and each JPEG/PNG
// is megabytes — without a cleanup job the state dir balloons on active use.
// These tests pin the cleanup sweep's behavior: what it removes, what it
// keeps, and how it behaves on odd filesystem states.

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// touch writes a zero-byte file and stamps mtime to the given time.
func touch(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatalf("chtimes %s: %v", path, err)
	}
}

func TestCleanupOldScreenshots_RemovesFilesOlderThanThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	maxAge := 72 * time.Hour

	oldJPG := filepath.Join(dir, "old.jpg")
	oldPNG := filepath.Join(dir, "old.png")
	touch(t, oldJPG, now.Add(-maxAge).Add(-time.Second))
	touch(t, oldPNG, now.Add(-maxAge).Add(-time.Minute))

	removed, failed, err := cleanupOldScreenshots(dir, maxAge, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 2 || failed != 0 {
		t.Errorf("removed=%d failed=%d, want removed=2 failed=0", removed, failed)
	}
	if _, err := os.Stat(oldJPG); !os.IsNotExist(err) {
		t.Errorf("old.jpg should have been removed")
	}
	if _, err := os.Stat(oldPNG); !os.IsNotExist(err) {
		t.Errorf("old.png should have been removed")
	}
}

func TestCleanupOldScreenshots_KeepsFilesNewerThanThreshold(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	maxAge := 72 * time.Hour

	fresh := filepath.Join(dir, "fresh.jpg")
	atBoundary := filepath.Join(dir, "at-boundary.png")
	touch(t, fresh, now.Add(-1*time.Hour))
	// Exactly at the boundary is kept (retention is strict >).
	touch(t, atBoundary, now.Add(-maxAge))

	removed, failed, err := cleanupOldScreenshots(dir, maxAge, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 || failed != 0 {
		t.Errorf("removed=%d failed=%d, want 0/0", removed, failed)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("fresh.jpg should still exist: %v", err)
	}
	if _, err := os.Stat(atBoundary); err != nil {
		t.Errorf("at-boundary.png should still exist: %v", err)
	}
}

func TestCleanupOldScreenshots_IgnoresSubdirectoriesAndNonImageFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	maxAge := 72 * time.Hour

	// Sub-directory older than threshold — must not be removed.
	subdir := filepath.Join(dir, "archive")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ancient := now.Add(-365 * 24 * time.Hour)
	if err := os.Chtimes(subdir, ancient, ancient); err != nil {
		t.Fatalf("chtimes subdir: %v", err)
	}

	// Non-image file — must not be removed even if old.
	note := filepath.Join(dir, "README.txt")
	touch(t, note, ancient)

	// Sentinel old image — should be removed.
	oldImg := filepath.Join(dir, "remove-me.jpg")
	touch(t, oldImg, ancient)

	removed, failed, err := cleanupOldScreenshots(dir, maxAge, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 1 || failed != 0 {
		t.Errorf("removed=%d failed=%d, want 1/0", removed, failed)
	}
	if _, err := os.Stat(subdir); err != nil {
		t.Errorf("subdir must survive: %v", err)
	}
	if _, err := os.Stat(note); err != nil {
		t.Errorf("non-image file must survive: %v", err)
	}
	if _, err := os.Stat(oldImg); !os.IsNotExist(err) {
		t.Errorf("old image should have been removed")
	}
}

func TestCleanupOldScreenshots_MissingDirIsNoOp(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	removed, failed, err := cleanupOldScreenshots(missing, 72*time.Hour, time.Now())
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if removed != 0 || failed != 0 {
		t.Errorf("removed=%d failed=%d, want 0/0", removed, failed)
	}
}

func TestCleanupOldScreenshots_EmptyDirIsNoOp(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	removed, failed, err := cleanupOldScreenshots(dir, 72*time.Hour, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 0 || failed != 0 {
		t.Errorf("removed=%d failed=%d, want 0/0", removed, failed)
	}
}

func TestCleanupOldScreenshots_MixedExtensions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	maxAge := 72 * time.Hour
	stale := now.Add(-100 * time.Hour)

	// Variants we should remove: both screenshot extensions, upper/lower case.
	for _, name := range []string{"a.jpg", "b.JPG", "c.jpeg", "d.png", "e.PNG"} {
		touch(t, filepath.Join(dir, name), stale)
	}
	// Variants we should keep: non-image extensions, even if old.
	for _, name := range []string{"notes.txt", "capture.webm", "data.json"} {
		touch(t, filepath.Join(dir, name), stale)
	}

	removed, failed, err := cleanupOldScreenshots(dir, maxAge, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != 5 || failed != 0 {
		t.Errorf("removed=%d failed=%d, want 5/0", removed, failed)
	}
	for _, kept := range []string{"notes.txt", "capture.webm", "data.json"} {
		if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
			t.Errorf("%s must survive: %v", kept, err)
		}
	}
}
