package capture

import "time"

// IsPilotEnabled returns whether AI Web Pilot is currently enabled.
func (c *Capture) IsPilotEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.extensionState.pilotEnabled
}

// IsPilotActionAllowed returns whether pilot-gated actions should be allowed.
// Startup/reconnect uncertainty defaults to allowed until explicit disable arrives.
func (c *Capture) IsPilotActionAllowed() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap := pilotStatusSnapshotFromExtensionState(c.extensionState)
	return snap.EffectiveEnabled
}

// GetPilotStatus returns pilot and heartbeat command status.
// extension_connected uses the same threshold as IsExtensionConnected (10s on lastSyncSeen).
// extension_last_seen is the RFC3339 timestamp of the last /sync, empty if never synced.
//
// Invariants:
// - Returned in_progress slice is copied to prevent external mutation.
func (c *Capture) GetPilotStatus() any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snap := pilotStatusSnapshotFromExtensionState(c.extensionState)

	lastSeen := ""
	if !c.extensionState.lastSyncSeen.IsZero() {
		lastSeen = c.extensionState.lastSyncSeen.Format(time.RFC3339)
	}

	inProgress := make([]SyncInProgress, len(c.extensionState.inProgress))
	copy(inProgress, c.extensionState.inProgress)
	inProgressUpdated := ""
	if !c.extensionState.inProgressUpdated.IsZero() {
		inProgressUpdated = c.extensionState.inProgressUpdated.Format(time.RFC3339)
	}

	return map[string]any{
		"enabled":             snap.EffectiveEnabled,
		"configured_enabled":  snap.ConfiguredEnabled,
		"authoritative":       snap.Authoritative,
		"state":               snap.State,
		"source":              snap.Source,
		"extension_connected": !c.extensionState.lastSyncSeen.IsZero() && time.Since(c.extensionState.lastSyncSeen) < extensionDisconnectThreshold,
		"extension_last_seen": lastSeen,
		"in_progress_count":   len(inProgress),
		"in_progress":         inProgress,
		"in_progress_updated": inProgressUpdated,
	}
}

// GetInProgressCommands returns a copy of latest extension-reported active commands.
//
// Failure semantics:
// - Returns empty slice when no heartbeat data is available.
func (c *Capture) GetInProgressCommands() []SyncInProgress {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]SyncInProgress, len(c.extensionState.inProgress))
	copy(out, c.extensionState.inProgress)
	return out
}

// pilotStatusSnapshotFromExtensionState converts raw extension state to API-level pilot semantics.
//
// Failure semantics:
// - Unknown/unset source fields fall back to conservative defaults.
func pilotStatusSnapshotFromExtensionState(ext ExtensionState) pilotStatusSnapshot {
	snap := pilotStatusSnapshot{
		ConfiguredEnabled: ext.pilotEnabled,
		Authoritative:     ext.pilotStatusKnown,
	}

	if !ext.pilotStatusKnown {
		snap.EffectiveEnabled = true
		snap.State = PilotStateAssumedEnabled
		snap.Source = PilotSourceAssumedStartup
		return snap
	}

	if ext.pilotEnabled {
		snap.EffectiveEnabled = true
		snap.State = PilotStateEnabled
		if ext.pilotSource != "" {
			snap.Source = ext.pilotSource
		} else {
			snap.Source = PilotSourceExtensionSync
		}
		return snap
	}

	snap.EffectiveEnabled = false
	snap.State = PilotStateExplicitlyDisabled
	if ext.pilotSource != "" {
		snap.Source = ext.pilotSource
	} else {
		snap.Source = PilotSourceExtensionSync
	}
	return snap
}
