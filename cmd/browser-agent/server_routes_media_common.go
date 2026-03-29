// Purpose: Shared media helpers for screenshot and draw-mode HTTP routes.
// Why: Keeps common filename/path/data-url utilities in one place to avoid duplication across handlers.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/state"
)

// Screenshot rate limiting.
const (
	screenshotMinInterval = 1 * time.Second // Max 1 screenshot per second per client
)

// screenshotsDir returns the runtime screenshots directory, creating it if needed.
func screenshotsDir() (string, error) {
	dir, err := state.ScreenshotsDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine screenshots directory: %w", err)
	}
	// #nosec G301 -- directory: owner rwx, group rx for traversal
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("cannot create screenshots directory: %w", err)
	}
	return dir, nil
}
