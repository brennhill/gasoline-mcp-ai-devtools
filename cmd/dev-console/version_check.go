// version_check.go — GitHub version checking for update notifications.
// Checks GitHub for new releases and notifies users of available updates.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	versionCheckCacheTTL = 6 * time.Hour
	versionCheckInterval = 24 * time.Hour
	httpClientTimeout    = 10 * time.Second
)

var (
	availableVersion string
	lastVersionCheck time.Time
	versionCheckMu   sync.Mutex
)

var (
	// githubAPIURL can be overridden via GASOLINE_RELEASES_URL env var for forked repos
	githubAPIURL = getEnvOrDefault("GASOLINE_RELEASES_URL",
		"https://api.github.com/repos/brennhill/gasoline-mcp-ai-devtools/releases/latest")
)

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}

// checkGitHubVersion fetches the latest version from GitHub
// Returns early if cache is still valid (within 6 hours)
// Used to determine if a newer version is available to notify users
func checkGitHubVersion() {
	versionCheckMu.Lock()
	// Check if cache is still valid (6 hour TTL)
	if !lastVersionCheck.IsZero() && time.Since(lastVersionCheck) < versionCheckCacheTTL {
		versionCheckMu.Unlock()
		return
	}
	versionCheckMu.Unlock()

	// Fetch from GitHub (silent on errors - version check is optional/non-critical)
	client := &http.Client{Timeout: httpClientTimeout}
	resp, err := client.Get(githubAPIURL) // #nosec G107 -- constant GitHub API URL
	if err != nil {
		// Silent: network errors are common, version check is non-critical
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Silent: GitHub rate limits (403) and other errors are expected
		// Version info is available via get_health tool when it succeeds
		return
	}

	var releaseInfo struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releaseInfo); err != nil {
		// Silent: parse errors don't affect core functionality
		return
	}

	// Extract version from tag (e.g., "v5.2.6" -> "5.2.6")
	newVersion := strings.TrimPrefix(releaseInfo.TagName, "v")
	if newVersion == "" {
		// Silent: invalid tag doesn't affect core functionality
		return
	}

	versionCheckMu.Lock()
	availableVersion = newVersion
	lastVersionCheck = time.Now()
	versionCheckMu.Unlock()

	// Quiet mode: Version check results are available via get_health tool
	// No need to spam stderr - LLMs don't care about version updates
}

// startVersionCheckLoop starts a periodic check for new versions on GitHub (daily)
// Checks immediately on startup if no cached value, then periodically
// The goroutine stops cleanly when ctx is cancelled
func startVersionCheckLoop(ctx context.Context) {
	go func() {
		// Check immediately on startup
		checkGitHubVersion()

		// Then check periodically
		ticker := time.NewTicker(versionCheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				checkGitHubVersion()
			case <-ctx.Done():
				// Context cancelled - clean shutdown
				return
			}
		}
	}()
}
