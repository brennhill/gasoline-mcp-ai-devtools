// Purpose: Manages tracked tab state including tab ID, URL, title, and navigation history.
// Why: Isolates tab-tracking mutations from other extension state accessors.
package capture

import "time"

// GetTrackingStatus returns the current tab tracking state.
func (c *Capture) GetTrackingStatus() (enabled bool, tabID int, tabURL string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.trackingEnabled, c.extensionState.trackedTabID, c.extensionState.trackedTabURL
}

// UpdateTrackedTab programmatically updates the tracked tab state.
// Used by switch_tab to retarget subsequent commands to the newly activated tab.
//
// Invariants:
// - tabID must be > 0; zero/negative values are silently ignored.
// - trackingEnabled is set to true when a valid tabID is provided.
func (c *Capture) UpdateTrackedTab(tabID int, tabURL string, tabTitle string) {
	if tabID <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.trackingEnabled = true
	c.extensionState.trackedTabID = tabID
	c.extensionState.trackedTabURL = tabURL
	c.extensionState.trackedTabTitle = tabTitle
	c.extensionState.trackingUpdated = time.Now()
}

// GetTrackedTabTitle returns the tracked tab's title (may be stale).
func (c *Capture) GetTrackedTabTitle() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.trackedTabTitle
}

// GetTabStatus returns the Chrome tab status ("loading", "complete", or empty if unknown).
func (c *Capture) GetTabStatus() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.tabStatus
}

// IsTrackedTabActive returns whether the tracked tab is the foreground tab.
// Returns (active, known). known=false means the extension has not reported this yet.
func (c *Capture) IsTrackedTabActive() (bool, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.extensionState.trackedTabActive == nil {
		return false, false
	}
	return *c.extensionState.trackedTabActive, true
}

// SetTrackedTabActiveForTest sets the tracked tab active state for testing.
func (c *Capture) SetTrackedTabActiveForTest(active bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.trackedTabActive = &active
}
