// status.go — Extension tracking status types.
// Used by /sync for tab tracking state updates.
package capture

import (
	"time"
)

// ExtensionStatus represents a status ping from the browser extension
type ExtensionStatus struct {
	Type               string `json:"type"`
	TrackingEnabled    bool   `json:"tracking_enabled"`
	TrackedTabID       int    `json:"tracked_tab_id"`
	TrackedTabURL      string `json:"tracked_tab_url"`
	Message            string `json:"message,omitempty"`
	ExtensionConnected bool   `json:"extension_connected"`
	Timestamp          string `json:"timestamp"`
}

// UpdateExtensionStatus updates the capture state with extension tracking info.
// Used by the /sync endpoint.
func (c *Capture) UpdateExtensionStatus(status ExtensionStatus) {
	c.mu.Lock()
	c.ext.trackingEnabled = status.TrackingEnabled
	c.ext.trackedTabID = status.TrackedTabID
	c.ext.trackedTabURL = status.TrackedTabURL
	c.ext.trackingUpdated = time.Now()
	c.mu.Unlock()
}
