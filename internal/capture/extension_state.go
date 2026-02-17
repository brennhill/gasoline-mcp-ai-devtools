// Purpose: Owns extension_state.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// extension_state.go — Extension communication state and tab tracking.
// Groups 13 scattered fields from the Capture god object into a coherent sub-struct.
// Protected by parent Capture.mu (no separate lock — tightly coupled with event buffer writes).
package capture

import "time"

// ExtensionState tracks all extension-related state: connection, pilot, tracking, and test boundaries.
// Protected by parent Capture.mu (no separate lock) because activeTestIDs is read
// during hot-path event ingestion (AddWebSocketEvents, AddNetworkBodies, AddEnhancedActions).
type ExtensionState struct {
	// Connection tracking
	lastPollAt             time.Time // When extension last polled. Health endpoint uses 3s/5s thresholds.
	extensionSession       string    // Extension session ID (changes on reload/update).
	sessionChangedAt       time.Time // When extensionSession last changed.
	lastExtensionConnected bool      // Previous connection state for transition detection.
	extensionVersion       string    // Last reported extension version from sync request.

	// Disconnect detection (P0-1 hardening)
	lastSyncSeen     time.Time // When last /sync request was received. Zero = never synced.
	lastSyncClientID string    // Client ID from most recent /sync request.

	// AI Web Pilot
	pilotEnabled   bool      // Pilot toggle from POST /settings or sync. Check before dispatching actions.
	pilotUpdatedAt time.Time // When pilotEnabled was last updated. Staleness threshold: 10s.

	// Tab tracking
	trackingEnabled bool      // Single-tab mode active. true=specific tab, false=all tabs.
	trackedTabID    int       // Browser tab ID (0=none). Invariant: trackingEnabled → trackedTabID>0.
	trackedTabURL   string    // Tracked tab URL (informational, may be stale).
	trackedTabTitle string    // Tracked tab title (informational, may be stale).
	trackingUpdated time.Time // When tracking status last refreshed.

	// Test boundaries
	activeTestIDs map[string]bool // Active test boundary IDs. Used to tag events during ingestion.
}

// ============================================================================
// Capture delegation methods — preserve external API.
// ============================================================================

// GetTrackingStatus returns the current tab tracking state.
func (c *Capture) GetTrackingStatus() (enabled bool, tabID int, tabURL string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ext.trackingEnabled, c.ext.trackedTabID, c.ext.trackedTabURL
}

// GetTrackedTabTitle returns the tracked tab's title (may be stale).
func (c *Capture) GetTrackedTabTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ext.trackedTabTitle
}

// IsPilotEnabled returns whether AI Web Pilot is currently enabled.
func (c *Capture) IsPilotEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ext.pilotEnabled
}

// IsExtensionConnected returns true if the extension has synced within the
// disconnect threshold (10s). Returns false if never synced or stale.
func (c *Capture) IsExtensionConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return !c.ext.lastSyncSeen.IsZero() && time.Since(c.ext.lastSyncSeen) < extensionDisconnectThreshold
}

// GetExtensionStatus returns a snapshot of extension connection state.
// Fields: connected (bool), last_seen (RFC3339 string), client_id (string).
func (c *Capture) GetExtensionStatus() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	connected := !c.ext.lastSyncSeen.IsZero() && time.Since(c.ext.lastSyncSeen) < extensionDisconnectThreshold

	lastSeen := ""
	if !c.ext.lastSyncSeen.IsZero() {
		lastSeen = c.ext.lastSyncSeen.Format(time.RFC3339)
	}

	return map[string]any{
		"connected": connected,
		"last_seen": lastSeen,
		"client_id": c.ext.lastSyncClientID,
	}
}

// GetPilotStatus returns pilot status information.
// extension_connected uses the same threshold as IsExtensionConnected (10s on lastSyncSeen).
// extension_last_seen is the RFC3339 timestamp of the last /sync, empty if never synced.
func (c *Capture) GetPilotStatus() any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lastSeen := ""
	if !c.ext.lastSyncSeen.IsZero() {
		lastSeen = c.ext.lastSyncSeen.Format(time.RFC3339)
	}

	return map[string]any{
		"enabled":              c.ext.pilotEnabled,
		"source":               "extension_poll",
		"extension_connected":  !c.ext.lastSyncSeen.IsZero() && time.Since(c.ext.lastSyncSeen) < extensionDisconnectThreshold,
		"extension_last_seen":  lastSeen,
	}
}

// GetExtensionVersion returns the last reported extension version.
func (c *Capture) GetExtensionVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ext.extensionVersion
}

// GetVersionMismatch checks whether extension and server versions differ in major.minor.
// Returns the extension version, server version, and whether a mismatch exists.
// A mismatch is detected only when the extension has reported a version (non-empty)
// and the major.minor portions differ from the server version.
func (c *Capture) GetVersionMismatch() (extensionVersion string, serverVersion string, hasMismatch bool) {
	c.mu.RLock()
	extVer := c.ext.extensionVersion
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

// GetActiveTestIDs returns the list of currently active test IDs.
func (c *Capture) GetActiveTestIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0, len(c.ext.activeTestIDs))
	for testID := range c.ext.activeTestIDs {
		result = append(result, testID)
	}
	return result
}

// SetTestBoundaryStart adds a test ID to the active set for correlating entries.
func (c *Capture) SetTestBoundaryStart(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ext.activeTestIDs[id] = true
}

// SetTestBoundaryEnd removes a test ID from the active set.
func (c *Capture) SetTestBoundaryEnd(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.ext.activeTestIDs, id)
}

// ExtensionSnapshot contains a point-in-time view of extension state for health reporting.
type ExtensionSnapshot struct {
	LastPollAt        time.Time
	ExtensionSession  string
	SessionChangedAt  time.Time
	PilotEnabled      bool
	ActiveTestIDCount int
}

// getExtensionSnapshot returns a snapshot of extension state.
// MUST be called with c.mu held (RLock or Lock).
func (c *Capture) getExtensionSnapshot() ExtensionSnapshot {
	return ExtensionSnapshot{
		LastPollAt:        c.ext.lastPollAt,
		ExtensionSession:  c.ext.extensionSession,
		SessionChangedAt:  c.ext.sessionChangedAt,
		PilotEnabled:      c.ext.pilotEnabled,
		ActiveTestIDCount: len(c.ext.activeTestIDs),
	}
}
