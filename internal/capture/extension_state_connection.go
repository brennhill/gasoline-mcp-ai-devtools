// Purpose: Polls for extension connection readiness with timeout and context cancellation.
// Why: Isolates connection-wait logic from other extension state accessors.
package capture

import (
	"context"
	"time"
)

// WaitForExtensionConnected polls until the extension connects, the timeout elapses,
// or ctx is cancelled. Returns true if connected within the window, false otherwise.
// A zero or negative timeout returns false immediately (no polling).
// Poll interval is extensionReadinessPollInterval (200ms). ctx cancellation is
// honoured between polls.
func (c *Capture) WaitForExtensionConnected(ctx context.Context, timeout time.Duration) bool {
	if c.IsExtensionConnected() {
		return true
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(extensionReadinessPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if c.IsExtensionConnected() {
				return true
			}
		}
	}
}

// IsExtensionConnected returns true if the extension has synced within the
// disconnect threshold (10s). Returns false if never synced or stale.
func (c *Capture) IsExtensionConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.extensionState.lastSyncSeen.IsZero() && time.Since(c.extensionState.lastSyncSeen) < extensionDisconnectThreshold
}

// GetExtensionStatus returns a detached connection snapshot.
// Fields: connected (bool), last_seen (RFC3339 string), client_id (string).
//
// Failure semantics:
// - If extension has never synced, last_seen is empty and connected=false.
func (c *Capture) GetExtensionStatus() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	connected := !c.extensionState.lastSyncSeen.IsZero() && time.Since(c.extensionState.lastSyncSeen) < extensionDisconnectThreshold

	lastSeen := ""
	if !c.extensionState.lastSyncSeen.IsZero() {
		lastSeen = c.extensionState.lastSyncSeen.Format(time.RFC3339)
	}

	return map[string]any{
		"connected": connected,
		"last_seen": lastSeen,
		"client_id": c.extensionState.lastSyncClientID,
	}
}

// GetExtensionVersion returns the last reported extension version.
func (c *Capture) GetExtensionVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.extensionVersion
}

// GetVersionMismatch checks whether extension and server versions differ in major.minor.
// Returns the extension version, server version, and whether a mismatch exists.
// A mismatch is detected only when the extension has reported a version (non-empty)
// and the major.minor portions differ from the server version.
func (c *Capture) GetVersionMismatch() (extensionVersion string, serverVersion string, hasMismatch bool) {
	c.mu.RLock()
	extVer := c.extensionState.extensionVersion
	srvVer := c.serverVersion
	c.mu.RUnlock()

	if extVer == "" || srvVer == "" {
		return extVer, srvVer, false
	}

	extMajorMinor := majorMinor(extVer)
	srvMajorMinor := majorMinor(srvVer)
	if extMajorMinor == "" || srvMajorMinor == "" {
		return extVer, srvVer, false
	}

	return extVer, srvVer, extMajorMinor != srvMajorMinor
}

// majorMinor extracts "X.Y" from a semver string "X.Y.Z".
// Returns empty string if the version is not in a recognized format.
func majorMinor(v string) string {
	firstDot := -1
	for i := 0; i < len(v); i++ {
		if v[i] == '.' {
			if firstDot == -1 {
				firstDot = i
			} else {
				// Found second dot — return up to (but not including) it
				return v[:i]
			}
		}
	}
	// No second dot found — not a valid semver with patch
	if firstDot != -1 {
		return v // "X.Y" format, return as-is
	}
	return ""
}
