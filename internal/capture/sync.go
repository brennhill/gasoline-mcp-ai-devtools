// sync.go — Unified sync endpoint consolidating multiple polling loops.
// POST /sync: Single bidirectional sync for extension ↔ server communication.
// Replaces: /pending-queries (GET), /settings (POST), /extension-logs (POST),
// /api/extension-status (POST).
package capture

import (
	"encoding/json"
	"net/http"
	"time"
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
	CorrelationID string          `json:"correlation_id,omitempty"`
}

// =============================================================================
// Handler
// =============================================================================

// HandleSync processes the unified sync endpoint.
// POST: Receives settings, logs, command results; returns pending commands.
func (c *Capture) HandleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	now := time.Now()

	// Get client ID for multi-client support
	clientID := r.Header.Get("X-Gasoline-Client")

	// Update connection state and settings (single lock scope)
	c.mu.Lock()

	// Detect extension connection state transitions
	wasConnected := c.ext.lastExtensionConnected
	timeSinceLastPoll := now.Sub(c.ext.lastPollAt)
	isReconnect := !c.ext.lastPollAt.IsZero() && timeSinceLastPoll > 3*time.Second

	// Update last poll time (for health endpoint extension_connected status)
	c.ext.lastPollAt = now
	c.ext.lastExtensionConnected = true

	// Handle session ID (detect session changes)
	if req.SessionID != "" && req.SessionID != c.ext.extensionSession {
		c.ext.extensionSession = req.SessionID
		c.ext.sessionChangedAt = now
	}
	sessionID := c.ext.extensionSession

	// Store extension settings
	if req.Settings != nil {
		c.ext.pilotEnabled = req.Settings.PilotEnabled
		c.ext.pilotUpdatedAt = now
		c.ext.trackingEnabled = req.Settings.TrackingEnabled
		c.ext.trackedTabID = req.Settings.TrackedTabID
		c.ext.trackedTabURL = req.Settings.TrackedTabURL
		c.ext.trackedTabTitle = req.Settings.TrackedTabTitle
		c.ext.trackingUpdated = now
	}

	// Read state for later use (captured inside lock for consistency)
	pilotEnabled := c.ext.pilotEnabled

	c.mu.Unlock()

	// Emit extension connection lifecycle event (outside lock, non-blocking)
	if !wasConnected || isReconnect {
		go c.emitLifecycleEvent("extension_connected", map[string]any{
			"session_id":         sessionID,
			"is_reconnect":       isReconnect,
			"disconnect_seconds": timeSinceLastPoll.Seconds(),
		})
	}

	// Process command results (these methods have their own locks)
	for _, result := range req.CommandResults {
		if result.ID != "" {
			c.SetQueryResultWithClient(result.ID, result.Result, clientID)
		}
		// Handle async commands (navigate, execute_js) that use correlation_id
		if result.CorrelationID != "" {
			c.CompleteCommand(result.CorrelationID, result.Result, result.Error)
		}
	}

	// Get all pending commands (single-client: no filtering needed)
	pendingQueries := c.GetPendingQueries()

	// Update remaining state (single lock scope for logging, extension logs, and version)
	c.mu.Lock()

	c.logPollingActivity(PollingLogEntry{
		Timestamp:    now,
		Endpoint:     "sync",
		Method:       "POST",
		SessionID:    req.SessionID,
		PilotEnabled: &pilotEnabled,
		QueryCount:   len(pendingQueries),
	})

	for _, log := range req.ExtensionLogs {
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		c.elb.logs = append(c.elb.logs, log)
		if len(c.elb.logs) > MaxExtensionLogs {
			kept := make([]ExtensionLog, MaxExtensionLogs)
			copy(kept, c.elb.logs[len(c.elb.logs)-MaxExtensionLogs:])
			c.elb.logs = kept
		}
	}

	if req.ExtensionVersion != "" {
		c.ext.extensionVersion = req.ExtensionVersion
	}

	c.mu.Unlock()

	// Convert pending queries to sync commands
	commands := make([]SyncCommand, len(pendingQueries))
	for i, q := range pendingQueries {
		paramsJSON, _ := json.Marshal(q.Params)
		commands[i] = SyncCommand{
			ID:            q.ID,
			Type:          q.Type,
			Params:        paramsJSON,
			CorrelationID: q.CorrelationID,
		}
	}

	// Build response
	resp := SyncResponse{
		Ack:              true,
		Commands:         commands,
		NextPollMs:       1000, // Default 1 second, can be made dynamic later
		ServerTime:       now.Format(time.RFC3339),
		ServerVersion:    c.GetServerVersion(),
		CaptureOverrides: map[string]string{}, // Empty for now, placeholder for AI capture control
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
