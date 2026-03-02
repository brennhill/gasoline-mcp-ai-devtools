// Purpose: Shared media helpers for screenshot and draw-mode HTTP routes.
// Why: Keeps common filename/path/data-url utilities in one place to avoid duplication across handlers.
// Docs: docs/features/feature/tab-recording/index.md

package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
)

// Screenshot rate limiting.
const (
	screenshotMinInterval = 1 * time.Second // Max 1 screenshot per second per client
)

// sanitizeFilename removes characters unsafe for filenames.
var unsafeChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeForFilename(s string) string {
	s = unsafeChars.ReplaceAllString(s, "_")
	if len(s) > 50 {
		s = s[:50]
	}
	return s
}

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

// decodeDataURL extracts binary data from a data URL (e.g. "data:image/jpeg;base64,...").
// Returns the decoded bytes, or an error string suitable for HTTP responses.
func decodeDataURL(dataURL string) ([]byte, string) {
	if dataURL == "" {
		return nil, "Missing dataUrl"
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return nil, "Invalid dataUrl format"
	}
	imageData, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "Invalid base64 data"
	}
	return imageData, ""
}
