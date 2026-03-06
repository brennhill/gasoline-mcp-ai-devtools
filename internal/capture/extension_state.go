// Purpose: Defines extension connection/tracking/pilot/security state and related capture delegation methods.
// Why: Centralizes extension lifecycle state used for routing, health checks, and command reconciliation.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import "time"

const (
	PilotStateAssumedEnabled     = "assumed_enabled"
	PilotStateEnabled            = "enabled"
	PilotStateExplicitlyDisabled = "explicitly_disabled"

	PilotSourceAssumedStartup = "assumed_startup"
	PilotSourceExtensionSync  = "extension_sync"
	PilotSourceSettingsCache  = "settings_cache"
	PilotSourceTestHelper     = "test_helper"

	SecurityModeNormal        = "normal"
	SecurityModeInsecureProxy = "insecure_proxy"
)

// ExtensionState tracks all extension-related state: connection, pilot, tracking, and test boundaries.
// Protected by parent Capture.mu (no separate lock) because activeTestIDs is read
// during hot-path event ingestion (AddWebSocketEvents, AddNetworkBodies, AddEnhancedActions).
//
// Invariants:
// - trackingEnabled implies trackedTabID > 0 for authoritative single-tab mode.
// - lastSyncSeen.IsZero() means extension has never synced in this process lifecycle.
// - missingInProgressByCorr tracks only commands currently pending in QueryDispatcher.
//
// Failure semantics:
//   - pilotStatusKnown=false intentionally defaults effective pilot access to enabled
//     until an authoritative sync/settings signal arrives.
type ExtensionState struct {
	// Connection tracking
	lastPollAt             time.Time // When extension last polled. Health endpoint uses 3s/5s thresholds.
	extSessionID           string    // Extension session ID (changes on reload/update).
	extSessionChangedAt    time.Time // When extSessionID last changed.
	lastExtensionConnected bool      // Previous connection state for transition detection.
	extensionVersion       string    // Last reported extension version from sync request.

	// Disconnect detection (P0-1 hardening)
	lastSyncSeen     time.Time // When last /sync request was received. Zero = never synced.
	lastSyncClientID string    // Client ID from most recent /sync request.

	// AI Web Pilot
	pilotEnabled     bool      // Last known pilot toggle from sync/settings cache.
	pilotStatusKnown bool      // False until authoritative pilot status is observed.
	pilotUpdatedAt   time.Time // When pilotEnabled was last updated.
	pilotSource      string    // Source of last authoritative pilot signal (sync/cache/test helper).

	// Tab tracking
	trackingEnabled  bool      // Single-tab mode active. true=specific tab, false=all tabs.
	trackedTabID     int       // Browser tab ID (0=none). Invariant: trackingEnabled → trackedTabID>0.
	trackedTabURL    string    // Tracked tab URL (informational, may be stale).
	trackedTabTitle  string    // Tracked tab title (informational, may be stale).
	tabStatus        string    // Chrome tab status: "loading" or "complete". Empty if unknown.
	trackedTabActive *bool     // Whether the tracked tab is the active (foreground) tab. nil=unknown.
	trackingUpdated  time.Time // When tracking status last refreshed.

	// Extension-reported active command execution state from /sync heartbeats.
	inProgress              []SyncInProgress // Last heartbeat snapshot of active commands.
	inProgressUpdated       time.Time        // When inProgress was last refreshed.
	missingInProgressByCorr map[string]int   // Consecutive missed heartbeats for started commands.

	// CSP detection: probed after each navigation to surface restrictions proactively.
	cspRestricted bool   // true if page CSP blocks execute_js (new Function).
	cspLevel      string // "none", "script_exec", or "page_blocked".

	// Last-resort altered-environment debug mode.
	securityMode     string   // "normal" (default) or "insecure_proxy".
	insecureRewrites []string // Rewrite set active in insecure mode (for transparent reporting).

	// Test boundaries
	activeTestIDs map[string]bool // Active test boundary IDs. Used to tag events during ingestion.
}

// ExtensionSnapshot contains a point-in-time view of extension state for health reporting.
type ExtensionSnapshot struct {
	LastPollAt          time.Time
	ExtSessionID        string
	ExtSessionChangedAt time.Time
	PilotEnabled        bool
	ActiveTestIDCount   int
}

type pilotStatusSnapshot struct {
	ConfiguredEnabled bool
	EffectiveEnabled  bool
	Authoritative     bool
	State             string
	Source            string
}
