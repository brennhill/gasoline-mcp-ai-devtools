// status.go — Extension status ping endpoint.
// Receives periodic status pings from the browser extension reporting
// the current tab tracking state. This allows the MCP tools to check
// whether tracking is enabled before processing observe/interact requests.
package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// ExtensionStatus represents a status ping from the browser extension
type ExtensionStatus struct {
	Type              string `json:"type"`
	TrackingEnabled   bool   `json:"tracking_enabled"`
	TrackedTabID      int    `json:"tracked_tab_id"`
	TrackedTabURL     string `json:"tracked_tab_url"`
	Message           string `json:"message,omitempty"`
	ExtensionConnected bool  `json:"extension_connected"`
	Timestamp         string `json:"timestamp"`
}

// HandleExtensionStatus handles POST /api/extension-status from the browser extension.
// Updates the capture state with the latest tracking information.
func (c *Capture) HandleExtensionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == "GET" {
		// Return current tracking status
		c.mu.RLock()
		status := ExtensionStatus{
			Type:              "status",
			TrackingEnabled:   c.trackingEnabled,
			TrackedTabID:      c.trackedTabID,
			TrackedTabURL:     c.trackedTabURL,
			ExtensionConnected: !c.trackingUpdated.IsZero() && time.Since(c.trackingUpdated) < 2*time.Minute,
			Timestamp:         c.trackingUpdated.Format(time.RFC3339),
		}
		c.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		//nolint:errcheck -- HTTP response encoding errors are logged by client; no recovery possible
		_ = json.NewEncoder(w).Encode(status)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var status ExtensionStatus
	if err := json.NewDecoder(r.Body).Decode(&status); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	c.mu.Lock()
	c.trackingEnabled = status.TrackingEnabled
	c.trackedTabID = status.TrackedTabID
	c.trackedTabURL = status.TrackedTabURL
	c.trackingUpdated = time.Now()
	c.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	//nolint:errcheck -- HTTP response encoding errors are logged by client; no recovery possible
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"received": true,
	})
}

// GetTrackingStatus returns the current tab tracking state.
// Used by MCP tools to check if tracking is enabled before processing requests.
func (c *Capture) GetTrackingStatus() (enabled bool, tabID int, tabURL string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.trackingEnabled, c.trackedTabID, c.trackedTabURL
}
