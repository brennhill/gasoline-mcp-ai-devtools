// Purpose: Tracks extension connection heartbeat state transitions for /sync.
// Why: Keeps lock-scoped state mutation isolated from HTTP transport flow.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import "time"

// syncConnectionState is an immutable lock-scope snapshot used after releasing c.mu.
//
// Invariants:
// - Values are derived from one atomic read/update cycle in updateSyncConnectionState.
// - Safe for use in async callbacks because it does not reference mutable capture internals.
type syncConnectionState struct {
	wasConnected      bool
	isReconnect       bool
	wasDisconnected   bool
	timeSinceLastPoll time.Duration
	extSessionID      string
	pilotEnabled      bool
	inProgressCount   int
}

// updateSyncConnectionState applies heartbeat state transitions under c.mu.
//
// Invariants:
// - Caller receives a detached snapshot for post-lock lifecycle emission.
// - req.Settings/in_progress updates overwrite prior extension view atomically.
//
// Failure semantics:
// - Absent settings/in_progress leaves previous values intact.
func (c *Capture) updateSyncConnectionState(req SyncRequest, clientID string, now time.Time) syncConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()

	state := syncConnectionState{
		wasConnected:      c.extensionState.lastExtensionConnected,
		timeSinceLastPoll: now.Sub(c.extensionState.lastPollAt),
	}
	state.wasDisconnected = !c.extensionState.lastSyncSeen.IsZero() && now.Sub(c.extensionState.lastSyncSeen) >= extensionDisconnectThreshold
	state.isReconnect = state.wasDisconnected

	c.extensionState.lastPollAt = now
	c.extensionState.lastExtensionConnected = true
	c.extensionState.lastSyncSeen = now
	c.extensionState.lastSyncClientID = clientID

	if req.ExtSessionID != "" && req.ExtSessionID != c.extensionState.extSessionID {
		c.extensionState.extSessionID = req.ExtSessionID
		c.extensionState.extSessionChangedAt = now
	}
	state.extSessionID = c.extensionState.extSessionID

	if req.Settings != nil {
		c.extensionState.pilotEnabled = req.Settings.PilotEnabled
		c.extensionState.pilotStatusKnown = true
		c.extensionState.pilotUpdatedAt = now
		c.extensionState.pilotSource = PilotSourceExtensionSync
		c.extensionState.trackingEnabled = req.Settings.TrackingEnabled
		c.extensionState.trackedTabID = req.Settings.TrackedTabID
		c.extensionState.trackedTabURL = req.Settings.TrackedTabURL
		c.extensionState.trackedTabTitle = req.Settings.TrackedTabTitle
		c.extensionState.trackingUpdated = now
		switch req.Settings.TabStatus {
		case "loading", "complete":
			c.extensionState.tabStatus = req.Settings.TabStatus
		default:
			c.extensionState.tabStatus = ""
		}
		c.extensionState.trackedTabActive = req.Settings.TrackedTabActive
		c.extensionState.cspRestricted = req.Settings.CspRestricted
		c.extensionState.cspLevel = req.Settings.CspLevel
	}
	if req.InProgress != nil {
		c.extensionState.inProgress = normalizeInProgressList(req.InProgress)
		c.extensionState.inProgressUpdated = now
	}
	state.pilotEnabled = c.extensionState.pilotEnabled
	state.inProgressCount = len(c.extensionState.inProgress)
	return state
}
