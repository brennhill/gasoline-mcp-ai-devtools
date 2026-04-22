// screenshot_cleanup.go — Background retention for screenshots written to
// state.ScreenshotsDir(). Each capture is a multi-MB JPEG/PNG; without this
// sweep the state directory grows without bound on daily use.

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// Retention policy. Hardcoded for now — no external configuration surface
// because screenshots aren't long-lived user artifacts (they're captured
// on-demand for AI tooling and become stale as the page state changes).
const (
	screenshotMaxAge           = 72 * time.Hour // 3 days
	screenshotCleanupInterval  = 1 * time.Hour
	screenshotCleanupStartupDelay = 30 * time.Second // avoid hammering disk at boot
)

// Image extensions we own and will remove. Anything else a user drops in the
// screenshots directory (notes, exports, etc.) is left alone. Extensions are
// matched case-insensitively so macOS's JPG-uppercase and Chrome's lowercase
// both match.
var screenshotImageExts = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
}

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
		if _, ok := screenshotImageExts[ext]; !ok {
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

// startScreenshotDiskCleanup runs a background goroutine that sweeps the
// screenshots directory once per screenshotCleanupInterval and removes any
// image older than screenshotMaxAge. Exits when ctx is canceled.
//
// A short startup delay avoids racing a fresh daemon's first screenshot
// write and keeps boot-time disk I/O quiet.
func (s *Server) startScreenshotDiskCleanup(ctx context.Context) {
	util.SafeGo(func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(screenshotCleanupStartupDelay):
		}

		runSweep := func() {
			dir, dirErr := screenshotsDir()
			if dirErr != nil {
				s.logLifecycle("screenshot_cleanup_dir_error", 0, map[string]any{
					"error": dirErr.Error(),
				})
				return
			}
			removed, failed, err := cleanupOldScreenshots(dir, screenshotMaxAge, time.Now())
			if err != nil {
				s.logLifecycle("screenshot_cleanup_read_error", 0, map[string]any{
					"dir":   dir,
					"error": err.Error(),
				})
				return
			}
			if removed > 0 || failed > 0 {
				s.logLifecycle("screenshot_cleanup_swept", 0, map[string]any{
					"removed": removed,
					"failed":  failed,
					"max_age": screenshotMaxAge.String(),
				})
			}
		}

		runSweep()

		ticker := time.NewTicker(screenshotCleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSweep()
			}
		}
	})
}
