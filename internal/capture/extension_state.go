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

// GetPilotStatus returns pilot status information.
// extension_connected is true only if the extension polled within the last 5 seconds.
func (c *Capture) GetPilotStatus() any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]any{
		"enabled":             c.ext.pilotEnabled,
		"source":              "extension_poll",
		"extension_connected": !c.ext.lastPollAt.IsZero() && time.Since(c.ext.lastPollAt) < 5*time.Second,
	}
}

// GetExtensionVersion returns the last reported extension version.
func (c *Capture) GetExtensionVersion() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ext.extensionVersion
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
