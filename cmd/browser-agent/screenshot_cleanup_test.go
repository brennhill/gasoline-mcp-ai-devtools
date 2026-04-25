// screenshot_cleanup_test.go — Tests for disk-bound screenshot retention.
// Screenshots accumulate indefinitely without maintenance and each JPEG/PNG
// is megabytes — without a cleanup job the state dir balloons on active use.
// These tests pin the cleanup sweep's behavior: what it removes, what it
// keeps, and how it behaves on odd filesystem states.

package main

import (
	"context"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
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

// TestCleanupOldScreenshots_CountsRemoveFailures pins the `failed` counter
// against an unwritable parent directory. POSIX-only: chmod 0500 prevents
// directory-entry deletion. On Windows os.Remove ignores the bits so the
// test is meaningless there.
func TestCleanupOldScreenshots_CountsRemoveFailures(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based unwritable-dir test is POSIX-only")
	}
	t.Parallel()
	dir := t.TempDir()
	now := time.Now()
	maxAge := 72 * time.Hour
	stale := now.Add(-100 * time.Hour)

	target := filepath.Join(dir, "stuck.jpg")
	touch(t, target, stale)

	// Mode 0500 = r-x ---- ---- on owner: dir is readable + executable
	// (so ReadDir + entry.Info still work), but not writable, so unlinking
	// any entry fails with EACCES. Restore on cleanup so t.TempDir()'s
	// auto-cleanup can succeed.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod 0500: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	removed, failed, err := cleanupOldScreenshots(dir, maxAge, now)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if removed != 0 || failed != 1 {
		t.Errorf("removed=%d failed=%d, want 0/1", removed, failed)
	}
	// File should still exist — sweep failed to remove it.
	if _, err := os.Stat(target); err != nil {
		t.Errorf("stuck.jpg should still exist: %v", err)
	}
}

// recorderConfig collects log events emitted by the cleanup loop so tests
// can assert on event names + field shapes. Lives entirely in test memory;
// production paths route through s.logLifecycle.
type recorderConfig struct {
	mu     sync.Mutex
	events []recordedEvent
}

type recordedEvent struct {
	Name   string
	Fields map[string]any
}

func (r *recorderConfig) emit(name string, fields map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Copy fields so subsequent map mutations in production code can't race
	// with the recorded slice.
	cp := make(map[string]any, len(fields))
	maps.Copy(cp, fields)
	r.events = append(r.events, recordedEvent{Name: name, Fields: cp})
}

func (r *recorderConfig) snapshot() []recordedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedEvent, len(r.events))
	copy(out, r.events)
	return out
}

// TestRunScreenshotCleanupLoop_LogShapesAreStable pins the structured-log
// contract for the three event names emitted by the loop:
// `screenshot_cleanup_swept`, `..._dir_error`, `..._read_error`. A future
// rename or field drop would silently break dashboard consumers; this test
// catches it.
func TestRunScreenshotCleanupLoop_LogShapesAreStable(t *testing.T) {
	t.Parallel()

	t.Run("swept event has removed/failed/max_age", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		now := time.Now()
		stale := now.Add(-100 * time.Hour)
		touch(t, filepath.Join(dir, "old.jpg"), stale)

		rec := &recorderConfig{}
		startup := make(chan time.Time, 1)
		startup <- now
		ticker := make(chan time.Time)
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			runScreenshotCleanupLoop(ctx, screenshotCleanupConfig{
				dirFn:        func() (string, error) { return dir, nil },
				now:          func() time.Time { return now },
				startupDelay: startup,
				tickerC:      ticker,
				maxAge:       72 * time.Hour,
				logEvent:     rec.emit,
			})
			close(done)
		}()

		// Run the startup sweep, then cancel before any tick fires.
		// Loop pauses awaiting ticker — cancel breaks it out.
		// We need a brief moment to let the sweep land.
		for range 100 {
			if len(rec.snapshot()) > 0 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
		<-done

		events := rec.snapshot()
		if len(events) != 1 || events[0].Name != "screenshot_cleanup_swept" {
			t.Fatalf("events = %v, want 1 swept event", events)
		}
		for _, key := range []string{"removed", "failed", "max_age"} {
			if _, ok := events[0].Fields[key]; !ok {
				t.Errorf("swept event missing field %q (fields=%v)", key, events[0].Fields)
			}
		}
	})

	t.Run("dir_error event has error field", func(t *testing.T) {
		t.Parallel()
		rec := &recorderConfig{}
		startup := make(chan time.Time, 1)
		startup <- time.Now()
		ticker := make(chan time.Time)
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			runScreenshotCleanupLoop(ctx, screenshotCleanupConfig{
				dirFn:        func() (string, error) { return "", errors.New("permission denied") },
				now:          time.Now,
				startupDelay: startup,
				tickerC:      ticker,
				maxAge:       72 * time.Hour,
				logEvent:     rec.emit,
			})
			close(done)
		}()
		for range 100 {
			if len(rec.snapshot()) > 0 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
		<-done

		events := rec.snapshot()
		if len(events) != 1 || events[0].Name != "screenshot_cleanup_dir_error" {
			t.Fatalf("events = %v, want 1 dir_error event", events)
		}
		if events[0].Fields["error"] != "permission denied" {
			t.Errorf("error field = %v, want 'permission denied'", events[0].Fields["error"])
		}
	})

	t.Run("read_error event has dir + error fields", func(t *testing.T) {
		t.Parallel()
		// Point dirFn at a path that exists but isn't a directory — ReadDir
		// returns ENOTDIR, which is a non-IsNotExist error and triggers the
		// read-error path.
		notADir := filepath.Join(t.TempDir(), "file.txt")
		touch(t, notADir, time.Now())

		rec := &recorderConfig{}
		startup := make(chan time.Time, 1)
		startup <- time.Now()
		ticker := make(chan time.Time)
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			runScreenshotCleanupLoop(ctx, screenshotCleanupConfig{
				dirFn:        func() (string, error) { return notADir, nil },
				now:          time.Now,
				startupDelay: startup,
				tickerC:      ticker,
				maxAge:       72 * time.Hour,
				logEvent:     rec.emit,
			})
			close(done)
		}()
		for range 100 {
			if len(rec.snapshot()) > 0 {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		cancel()
		<-done

		events := rec.snapshot()
		if len(events) != 1 || events[0].Name != "screenshot_cleanup_read_error" {
			t.Fatalf("events = %v, want 1 read_error event", events)
		}
		for _, key := range []string{"dir", "error"} {
			if _, ok := events[0].Fields[key]; !ok {
				t.Errorf("read_error event missing field %q (fields=%v)", key, events[0].Fields)
			}
		}
	})
}

// TestRunScreenshotCleanupLoop_DedupesIdenticalErrors covers Correctness #6
// + Design #5: a permanent failure (e.g. permission denied on the state
// directory) should log once, not every hour forever. The loop tracks the
// last error message and suppresses identical repeats. When the error
// resolves, a recovery event fires.
func TestRunScreenshotCleanupLoop_DedupesIdenticalErrors(t *testing.T) {
	t.Parallel()
	rec := &recorderConfig{}
	ctx, cancel := context.WithCancel(context.Background())

	startup := make(chan time.Time, 1)
	startup <- time.Now()
	ticker := make(chan time.Time, 4)

	// First three sweeps fail with the same error; fourth succeeds.
	// sweepCount is read concurrently by the test thread (waitForSweepCount)
	// while the loop's goroutine increments it via dirFn — atomic keeps the
	// race detector happy.
	dir := t.TempDir()
	var sweepCount atomic.Int32
	dirFn := func() (string, error) {
		n := sweepCount.Add(1)
		if n <= 3 {
			return "", errors.New("permission denied")
		}
		return dir, nil
	}

	done := make(chan struct{})
	go func() {
		runScreenshotCleanupLoop(ctx, screenshotCleanupConfig{
			dirFn:        dirFn,
			now:          time.Now,
			startupDelay: startup,
			tickerC:      ticker,
			maxAge:       72 * time.Hour,
			logEvent:     rec.emit,
		})
		close(done)
	}()

	// Drive 4 sweeps: startup (fail) + 3 ticker (fail, fail, success).
	// Each tick is a manual send, so we know exactly when each sweep ran.
	waitForSweepCount := func(target int32) {
		for range 100 {
			if sweepCount.Load() >= target {
				return
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Fatalf("sweep count never reached %d (got %d)", target, sweepCount.Load())
	}
	waitForSweepCount(1)
	ticker <- time.Now()
	waitForSweepCount(2)
	ticker <- time.Now()
	waitForSweepCount(3)
	ticker <- time.Now()
	waitForSweepCount(4)
	cancel()
	<-done

	events := rec.snapshot()
	// Expectation: ONE error event (first sweep), ONE recovered event (fourth
	// sweep transition out of error). Sweeps 2 and 3 reported the same error
	// and are deduped to silence. Sweep 4 found no work to log so emits no
	// `swept` event (removed=0, failed=0).
	var errCount, recoveredCount int
	for _, e := range events {
		switch e.Name {
		case "screenshot_cleanup_dir_error":
			errCount++
		case "screenshot_cleanup_recovered":
			recoveredCount++
		}
	}
	if errCount != 1 {
		t.Errorf("dir_error count = %d, want 1 (events=%v)", errCount, events)
	}
	if recoveredCount != 1 {
		t.Errorf("recovered count = %d, want 1 (events=%v)", recoveredCount, events)
	}
}

// TestRunScreenshotCleanupLoop_CancelDuringStartupDelay verifies the
// goroutine exits cleanly when ctx is canceled before the startup delay
// elapses (i.e. the daemon shuts down within the first 30 seconds).
func TestRunScreenshotCleanupLoop_CancelDuringStartupDelay(t *testing.T) {
	t.Parallel()
	rec := &recorderConfig{}
	ctx, cancel := context.WithCancel(context.Background())

	// Startup channel never fires.
	startup := make(chan time.Time)
	ticker := make(chan time.Time)

	var dirCalled atomic.Bool
	done := make(chan struct{})
	go func() {
		runScreenshotCleanupLoop(ctx, screenshotCleanupConfig{
			dirFn: func() (string, error) {
				dirCalled.Store(true)
				return t.TempDir(), nil
			},
			now:          time.Now,
			startupDelay: startup,
			tickerC:      ticker,
			maxAge:       72 * time.Hour,
			logEvent:     rec.emit,
		})
		close(done)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("loop did not exit after ctx cancel during startup delay")
	}
	if dirCalled.Load() {
		t.Error("dirFn was called even though startup delay never elapsed")
	}
	if len(rec.snapshot()) != 0 {
		t.Errorf("expected no events, got %v", rec.snapshot())
	}
}
