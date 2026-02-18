// Purpose: Owns sync.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// sync.go — Unified sync endpoint consolidating multiple polling loops.
// POST /sync: Single bidirectional sync for extension ↔ server communication.
// Replaces: /pending-queries (GET), /settings (POST), /extension-logs (POST),
// /api/extension-status (POST).
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package capture

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/util"
)

// =============================================================================
// Request/Response Types
// =============================================================================

// SyncRequest is the POST body for /sync
type SyncRequest struct {
	// Session identification
	SessionID string `json:"session_id"`

	// Extension version for compatibility checking
	ExtensionVersion string `json:"extension_version,omitempty"`

	// Extension settings (replaces /settings POST)
	Settings *SyncSettings `json:"settings,omitempty"`

	// Extension logs batch (replaces /extension-logs POST)
	ExtensionLogs []ExtensionLog `json:"extension_logs,omitempty"`

	// Ack last processed command ID (for reliable delivery)
	LastCommandAck string `json:"last_command_ack,omitempty"`

	// Command results batch (replaces multiple result POST endpoints)
	CommandResults []SyncCommandResult `json:"command_results,omitempty"`
}

// SyncSettings contains extension settings from the sync request
type SyncSettings struct {
	PilotEnabled     bool   `json:"pilot_enabled"`
	TrackingEnabled  bool   `json:"tracking_enabled"`
	TrackedTabID     int    `json:"tracked_tab_id"`
	TrackedTabURL    string `json:"tracked_tab_url"`
	TrackedTabTitle  string `json:"tracked_tab_title"`
	CaptureLogs      bool   `json:"capture_logs"`
	CaptureNetwork   bool   `json:"capture_network"`
	CaptureWebSocket bool   `json:"capture_websocket"`
	CaptureActions   bool   `json:"capture_actions"`
}

// SyncCommandResult is a command result from the extension
type SyncCommandResult struct {
	ID            string          `json:"id"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Status        string          `json:"status"` // "complete", "error", "timeout"
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
}

// SyncResponse is the response body for /sync
type SyncResponse struct {
	// Server acknowledged the sync
	Ack bool `json:"ack"`

	// Commands for extension to execute (replaces /pending-queries GET)
	Commands []SyncCommand `json:"commands"`

	// Server-controlled poll interval (dynamic backoff)
	NextPollMs int `json:"next_poll_ms"`

	// Server time for drift detection
	ServerTime string `json:"server_time"`

	// Server version for compatibility
	ServerVersion string `json:"server_version,omitempty"`

	// Capture overrides from AI (empty for now, placeholder for future feature)
	CaptureOverrides map[string]string `json:"capture_overrides"`
}

// SyncCommand is a command from server to extension
type SyncCommand struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Params        json.RawMessage `json:"params"`
	TabID         int             `json:"tab_id,omitempty"`
	CorrelationID string          `json:"correlation_id,omitempty"`
}

// =============================================================================
// Handler
// =============================================================================

// syncConnectionState holds the state captured during the connection update lock scope.
type syncConnectionState struct {
	wasConnected      bool
	isReconnect       bool
	wasDisconnected   bool
	timeSinceLastPoll time.Duration
	sessionID         string
	pilotEnabled      bool
}

// updateSyncConnectionState updates connection state under lock and returns captured state.
func (c *Capture) updateSyncConnectionState(req SyncRequest, clientID string, now time.Time) syncConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := syncConnectionState{
		wasConnected:      c.ext.lastExtensionConnected,
		timeSinceLastPoll: now.Sub(c.ext.lastPollAt),
	}
	s.isReconnect = !c.ext.lastPollAt.IsZero() && s.timeSinceLastPoll > 3*time.Second
	s.wasDisconnected = !c.ext.lastSyncSeen.IsZero() && now.Sub(c.ext.lastSyncSeen) >= extensionDisconnectThreshold

	c.ext.lastPollAt = now
	c.ext.lastExtensionConnected = true
	c.ext.lastSyncSeen = now
	c.ext.lastSyncClientID = clientID

	if req.SessionID != "" && req.SessionID != c.ext.extensionSession {
		c.ext.extensionSession = req.SessionID
		c.ext.sessionChangedAt = now
	}
	s.sessionID = c.ext.extensionSession

	if req.Settings != nil {
		c.ext.pilotEnabled = req.Settings.PilotEnabled
		c.ext.pilotUpdatedAt = now
		c.ext.trackingEnabled = req.Settings.TrackingEnabled
		c.ext.trackedTabID = req.Settings.TrackedTabID
		c.ext.trackedTabURL = req.Settings.TrackedTabURL
		c.ext.trackedTabTitle = req.Settings.TrackedTabTitle
		c.ext.trackingUpdated = now
	}
	s.pilotEnabled = c.ext.pilotEnabled
	return s
}

// processSyncCommandResults processes command results from the extension.
// Query results are stored via SetQueryResultOnly, then command completion is
// applied once with CompleteCommandWithStatus, preferring explicit correlation_id
// from the extension and falling back to the correlation mapped to query id.
func (c *Capture) processSyncCommandResults(results []SyncCommandResult, clientID string) {
	for _, result := range results {
		if result.ID != "" {
			mappedCorrelationID := c.SetQueryResultOnly(result.ID, result.Result, clientID)
			correlationID := result.CorrelationID
			if correlationID == "" {
				correlationID = mappedCorrelationID
			}
			if correlationID != "" {
				c.CompleteCommandWithStatus(correlationID, result.Result, result.Status, result.Error)
			}
		} else if result.CorrelationID != "" {
			c.CompleteCommandWithStatus(result.CorrelationID, result.Result, result.Status, result.Error)
		}
	}
}

// updateSyncLogs processes extension logs and version under lock.
func (c *Capture) updateSyncLogs(req SyncRequest, now time.Time, pilotEnabled bool, queryCount int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logPollingActivity(PollingLogEntry{
		Timestamp:    now,
		Endpoint:     "sync",
		Method:       "POST",
		SessionID:    req.SessionID,
		PilotEnabled: &pilotEnabled,
		QueryCount:   queryCount,
	})

	for _, log := range req.ExtensionLogs {
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		log = c.redactExtensionLog(log)
		c.elb.logs = append(c.elb.logs, log)
		// Amortized eviction: only compact when buffer exceeds 1.5x capacity.
		evictionThreshold := MaxExtensionLogs + MaxExtensionLogs/2
		if len(c.elb.logs) > evictionThreshold {
			kept := make([]ExtensionLog, MaxExtensionLogs)
			copy(kept, c.elb.logs[len(c.elb.logs)-MaxExtensionLogs:])
			c.elb.logs = kept
		}
	}

	if req.ExtensionVersion != "" {
		c.ext.extensionVersion = req.ExtensionVersion
	}
}

// buildSyncCommands converts pending queries to sync commands.
func buildSyncCommands(pending []queries.PendingQueryResponse) []SyncCommand {
	commands := make([]SyncCommand, len(pending))
	for i, q := range pending {
		commands[i] = SyncCommand{
			ID:            q.ID,
			Type:          q.Type,
			Params:        q.Params,
			TabID:         q.TabID,
			CorrelationID: q.CorrelationID,
		}
	}
	return commands
}

// HandleSync processes the unified sync endpoint.
// POST: Receives settings, logs, command results; returns pending commands.
func (c *Capture) HandleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxExtensionPostBody)
	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	now := time.Now()
	clientID := r.Header.Get("X-Gasoline-Client")

	state := c.updateSyncConnectionState(req, clientID, now)

	if !state.wasConnected || state.isReconnect {
		util.SafeGo(func() {
			c.emitLifecycleEvent("extension_connected", map[string]any{
				"session_id":         state.sessionID,
				"is_reconnect":       state.isReconnect,
				"disconnect_seconds": state.timeSinceLastPoll.Seconds(),
			})
		})
	}

	c.processSyncCommandResults(req.CommandResults, clientID)
	if req.LastCommandAck != "" {
		c.AcknowledgePendingQuery(req.LastCommandAck)
	}

	if state.wasDisconnected {
		c.qd.ExpireAllPendingQueries("extension_disconnected")
		util.SafeGo(func() {
			c.emitLifecycleEvent("extension_disconnected", map[string]any{
				"session_id": state.sessionID,
				"client_id":  clientID,
			})
		})
	}

	// Long-polling: if no commands, wait for up to 5 seconds
	pendingQueries := c.GetPendingQueries()
	if len(pendingQueries) == 0 {
		c.WaitForPendingQueries(5 * time.Second)
		pendingQueries = c.GetPendingQueries()
	}

	c.updateSyncLogs(req, now, state.pilotEnabled, len(pendingQueries))

	commands := buildSyncCommands(pendingQueries)

	nextPollMs := 1000
	if len(commands) > 0 {
		nextPollMs = 200
	}

	resp := SyncResponse{
		Ack:              true,
		Commands:         commands,
		NextPollMs:       nextPollMs,
		ServerTime:       now.Format(time.RFC3339),
		ServerVersion:    c.GetServerVersion(),
		CaptureOverrides: map[string]string{},
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
