// screenshot_cleanup.go — Background retention for screenshots written to
// state.ScreenshotsDir(). Each capture is a multi-MB JPEG/PNG; without this
// sweep the state directory grows without bound on daily use.
//
// Metrics emitted from this file (all via logLifecycle):
//   - screenshot_cleanup_swept       {removed, failed, max_age} — every
//                                    sweep that found work.
//   - screenshot_cleanup_dir_error   {error}                    — first
//                                    occurrence of a failure to resolve
//                                    state.ScreenshotsDir() (deduped).
//   - screenshot_cleanup_read_error  {dir, error}               — first
//                                    occurrence of a directory-read
//                                    failure (deduped).
//   - screenshot_cleanup_recovered   {prior_error}              — fires on
//                                    the transition from a sustained
//                                    error state back to healthy.
//
// These are local-only structured logs. No app-telemetry beacons.

package main

import (
	"context"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Retention policy. Hardcoded for now — no external configuration surface
// because screenshots aren't long-lived user artifacts (they're captured
// on-demand for AI tooling and become stale as the page state changes).
const (
	screenshotMaxAge              = 72 * time.Hour // 3 days
	screenshotCleanupInterval     = 1 * time.Hour
	screenshotCleanupStartupDelay = 30 * time.Second // avoid hammering disk at boot
)

// cleanupOldScreenshots removes image files in `dir` with ModTime strictly
// older than `now - maxAge`. Returns (removed, failed, err):
//   - removed: count of files successfully deleted.
//   - failed:  count of image files that were stale but os.Remove errored
//     on — sweep continues so a single bad inode doesn't block the rest.
//   - err:     only for unrecoverable directory-read errors. A missing
//     directory is NOT an error — first-boot and fresh installs have no
//     screenshots yet.
//
// Subdirectories are ignored entirely (no recursion). The draw-mode flow
// and standard screenshot flow both write flat into this directory, so
// recursion would surprise future features that add subdirs on purpose.
//
// File-set is gated by state.ScreenshotImageExts so a future producer adding
// a new format (e.g. WebP) is retired on the same schedule once the
// extension is registered there.
//
// TOCTOU: between Info() and Remove() a writer could overwrite the file with
// fresh content. The window is microseconds; producers always write to a
// freshly-named file (BuildScreenshotFilename uses ts+correlation_id as a
// uniqueness key) so name reuse is essentially impossible.
func cleanupOldScreenshots(dir string, maxAge time.Duration, now time.Time) (removed, failed int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	cutoff := now.Add(-maxAge)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := state.ScreenshotImageExts[ext]; !ok {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			failed++
			continue
		}
		if !info.ModTime().Before(cutoff) {
			continue
		}
		if rmErr := os.Remove(filepath.Join(dir, name)); rmErr != nil {
			failed++
			continue
		}
		removed++
	}
	return removed, failed, nil
}

// screenshotCleanupConfig is the injectable surface of the cleanup goroutine.
// Tests pass fakes for `now`, `dirFn`, `startupDelay`, and `tickerC` so the
// cancel-from-startup-delay and cancel-from-ticker paths are covered without
// real-time waits. Production wiring (startScreenshotDiskCleanup) plumbs
// real implementations.
type screenshotCleanupConfig struct {
	dirFn        func() (string, error)
	now          func() time.Time
	startupDelay <-chan time.Time
	tickerC      <-chan time.Time
	tickerStop   func()
	maxAge       time.Duration
	logEvent     func(event string, fields map[string]any)
}

// runScreenshotCleanupLoop drives the sweep until ctx is canceled. It is
// the unit-testable core of startScreenshotDiskCleanup; production callers
// should use the latter.
//
// Error dedup: identical consecutive errors are logged once. When the error
// clears (or changes), a `_recovered` event fires so dashboards can see the
// transition. Without this, a permanent permission-denied state would log
// every hour forever.
func runScreenshotCleanupLoop(ctx context.Context, cfg screenshotCleanupConfig) {
	if cfg.tickerStop != nil {
		defer cfg.tickerStop()
	}

	var lastErr string // empty means "last sweep was healthy"

	sweep := func() {
		dir, dirErr := cfg.dirFn()
		if dirErr != nil {
			emitDedupedError(cfg, "screenshot_cleanup_dir_error", &lastErr, dirErr.Error(), nil)
			return
		}
		removed, failed, err := cleanupOldScreenshots(dir, cfg.maxAge, cfg.now())
		if err != nil {
			emitDedupedError(cfg, "screenshot_cleanup_read_error", &lastErr, err.Error(), map[string]any{"dir": dir})
			return
		}
		// Healthy sweep: log recovery on the transition out of an error state,
		// then emit the result only when there's something to report.
		if lastErr != "" {
			cfg.logEvent("screenshot_cleanup_recovered", map[string]any{"prior_error": lastErr})
			lastErr = ""
		}
		if removed > 0 || failed > 0 {
			cfg.logEvent("screenshot_cleanup_swept", map[string]any{
				"removed": removed,
				"failed":  failed,
				"max_age": cfg.maxAge.String(),
			})
		}
	}

	select {
	case <-ctx.Done():
		return
	case <-cfg.startupDelay:
	}

	sweep()
	for {
		select {
		case <-ctx.Done():
			return
		case <-cfg.tickerC:
			sweep()
		}
	}
}

// emitDedupedError logs only if the error message differs from the previous
// one. lastErr is updated by reference so the caller's state moves forward.
func emitDedupedError(cfg screenshotCleanupConfig, event string, lastErr *string, msg string, extras map[string]any) {
	if *lastErr == msg {
		return
	}
	fields := map[string]any{"error": msg}
	maps.Copy(fields, extras)
	cfg.logEvent(event, fields)
	*lastErr = msg
}

// startScreenshotDiskCleanup runs a background goroutine that sweeps the
// screenshots directory once per screenshotCleanupInterval and removes any
// image older than screenshotMaxAge. Exits when ctx is canceled.
//
// A short startup delay avoids racing a fresh daemon's first screenshot
// write and keeps boot-time disk I/O quiet.
func (s *Server) startScreenshotDiskCleanup(ctx context.Context) {
	util.SafeGo(func() {
		ticker := time.NewTicker(screenshotCleanupInterval)
		runScreenshotCleanupLoop(ctx, screenshotCleanupConfig{
			dirFn:        screenshotsDir,
			now:          time.Now,
			startupDelay: time.After(screenshotCleanupStartupDelay),
			tickerC:      ticker.C,
			tickerStop:   ticker.Stop,
			maxAge:       screenshotMaxAge,
			logEvent: func(event string, fields map[string]any) {
				s.logLifecycle(event, 0, fields)
			},
		})
	})
}
