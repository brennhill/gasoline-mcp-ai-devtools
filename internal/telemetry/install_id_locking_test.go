// install_id_locking_test.go — Tests covering withKaboomStateLock branch
// behaviour: timeout-exhausted return when an existing lock file is fresh,
// and stale-lock reclaim when the existing lock's mtime is older than
// installStateLockStale.
//
// Tests in this package must NOT use t.Parallel() — installStateLockTimeout/
// Poll/Stale are package vars, and concurrent mutation would cross-pollute.

package telemetry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestWithKaboomStateLock_TimeoutExhausted pins the timeout branch: when a
// lock file already exists and is not stale, withKaboomStateLock waits up
// to installStateLockTimeout and then returns the original os.IsExist error.
// We shrink the timeout to keep the test fast.
func TestWithKaboomStateLock_TimeoutExhausted(t *testing.T) {
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	withLockBudget(t, 50*time.Millisecond, 5*time.Millisecond, 10*time.Second)

	lockPath := filepath.Join(dir, "test.lock")
	if err := os.WriteFile(lockPath, []byte{}, 0o600); err != nil {
		t.Fatalf("seed lock: %v", err)
	}

	start := time.Now()
	err := withKaboomStateLock("test.lock", func() error {
		t.Fatal("fn should not run when lock is held")
		return nil
	})
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected error when lock is held to timeout, got nil")
	}
	// Bracket: lower bound proves we waited at least one timeout window;
	// upper bound proves we didn't fall through to fn or hang on a
	// regression that bumped the timeout silently.
	if elapsed < 30*time.Millisecond {
		t.Errorf("withKaboomStateLock returned in %s; expected ~50ms timeout", elapsed)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("withKaboomStateLock took %s; expected ~50ms timeout", elapsed)
	}
}

// TestWithKaboomStateLock_StaleLockReclaimed pins the stale-lock branch: a
// lock file with mtime older than installStateLockStale is removed and the
// caller proceeds. Uses os.Chtimes to fabricate a stale mtime — no sleep.
//
// Some filesystems (FAT, certain network mounts) have coarse mtime
// resolution or treat Chtimes as a no-op. The test verifies the chtime
// actually changed mtime; if not, it skips with a clear reason.
func TestWithKaboomStateLock_StaleLockReclaimed(t *testing.T) {
	dir := t.TempDir()
	overrideKaboomDir(dir)
	defer resetKaboomDir()

	withLockBudget(t, 50*time.Millisecond, 5*time.Millisecond, 100*time.Millisecond)

	lockPath := filepath.Join(dir, "test.lock")
	if err := os.WriteFile(lockPath, []byte{}, 0o600); err != nil {
		t.Fatalf("seed lock: %v", err)
	}
	staleTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(lockPath, staleTime, staleTime); err != nil {
		t.Fatalf("chtime: %v", err)
	}
	if info, err := os.Stat(lockPath); err == nil {
		// If the filesystem ignored Chtimes (FAT, certain NFS mounts), skip
		// rather than burn a flaky failure.
		if time.Since(info.ModTime()) < installStateLockStale {
			t.Skipf("filesystem at %s did not honour os.Chtimes (mtime=%v); skipping stale-lock branch test", dir, info.ModTime())
		}
	}

	called := false
	if err := withKaboomStateLock("test.lock", func() error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("withKaboomStateLock: %v", err)
	}
	if !called {
		t.Fatal("fn was not invoked after stale lock was reclaimed")
	}
}
