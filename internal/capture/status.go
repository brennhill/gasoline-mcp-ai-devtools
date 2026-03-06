// Purpose: Defines /sync request-response envelopes and sync command reconciliation payload contracts.
// Why: Keeps extension-daemon synchronization protocol explicit and versionable in one location.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"time"
)

// ExtensionStatus is a legacy status envelope for non-/sync update paths.
//
// Invariants:
// - Tracking fields are merged directly into ExtensionState under c.mu.
type ExtensionStatus struct {
	Type               string `json:"type"`
	TrackingEnabled    bool   `json:"tracking_enabled"`
	TrackedTabID       int    `json:"tracked_tab_id"`
	TrackedTabURL      string `json:"tracked_tab_url"`
	Message            string `json:"message,omitempty"`
	ExtensionConnected bool   `json:"extension_connected"`
	Timestamp          string `json:"timestamp"`
}

// UpdateExtensionStatus applies legacy extension tracking updates.
//
// Failure semantics:
// - Timestamp parsing/validation is not enforced here; caller-provided fields are trusted.
func (c *Capture) UpdateExtensionStatus(status ExtensionStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extensionState.trackingEnabled = status.TrackingEnabled
	c.extensionState.trackedTabID = status.TrackedTabID
	c.extensionState.trackedTabURL = status.TrackedTabURL
	c.extensionState.trackingUpdated = time.Now()
}
