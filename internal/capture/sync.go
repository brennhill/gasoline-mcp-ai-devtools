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

	// Update state (lock scope 1)
	c.mu.Lock()

	// Detect extension connection state transitions
	wasConnected := c.lastExtensionConnected
	timeSinceLastPoll := now.Sub(c.lastPollAt)
	isReconnect := !c.lastPollAt.IsZero() && timeSinceLastPoll > 3*time.Second

	// Update last poll time (for health endpoint extension_connected status)
	c.lastPollAt = now
	c.lastExtensionConnected = true

	// Handle session ID (detect session changes)
	if req.SessionID != "" && req.SessionID != c.extensionSession {
		c.extensionSession = req.SessionID
		c.sessionChangedAt = now
	}

	// Capture data for lifecycle event before releasing lock
	sessionID := c.extensionSession
	c.mu.Unlock()

	// Emit extension connection lifecycle event (outside lock)
	if !wasConnected || isReconnect {
		go c.emitLifecycleEvent("extension_connected", map[string]any{
			"session_id":         sessionID,
			"is_reconnect":       isReconnect,
			"disconnect_seconds": timeSinceLastPoll.Seconds(),
		})
	}

	// Re-acquire lock for settings
	c.mu.Lock()

	// Store extension settings
	if req.Settings != nil {
		c.pilotEnabled = req.Settings.PilotEnabled
		c.pilotUpdatedAt = now
		c.trackingEnabled = req.Settings.TrackingEnabled
		c.trackedTabID = req.Settings.TrackedTabID
		c.trackedTabURL = req.Settings.TrackedTabURL
		c.trackedTabTitle = req.Settings.TrackedTabTitle
		c.trackingUpdated = now
	}
	c.mu.Unlock()

	// Process command results (these methods have their own locks)
	for _, result := range req.CommandResults {
		if result.ID != "" {
			c.SetQueryResultWithClient(result.ID, result.Result, clientID)
		}
	}

	// Get pending commands for this client (these methods have their own locks)
	var pendingQueries []PendingQueryResponse
	if clientID != "" {
		pendingQueries = c.GetPendingQueriesForClient(clientID)
	} else {
		pendingQueries = c.GetPendingQueries()
	}

	// Log the sync (lock scope 2)
	c.mu.Lock()
	pilotEnabled := c.pilotEnabled
	c.logPollingActivity(PollingLogEntry{
		Timestamp:    now,
		Endpoint:     "sync",
		Method:       "POST",
		SessionID:    req.SessionID,
		PilotEnabled: &pilotEnabled,
		QueryCount:   len(pendingQueries),
	})
	c.mu.Unlock()

	// Buffer extension logs (lock scope 3)
	if len(req.ExtensionLogs) > 0 {
		c.mu.Lock()
		for _, log := range req.ExtensionLogs {
			// Set server-side timestamp if not provided
			if log.Timestamp.IsZero() {
				log.Timestamp = now
			}
			c.extensionLogs = append(c.extensionLogs, log)
			// Evict oldest entries if over capacity
			if len(c.extensionLogs) > maxExtensionLogs {
				c.extensionLogs = c.extensionLogs[len(c.extensionLogs)-maxExtensionLogs:]
			}
		}
		c.mu.Unlock()
	}

	// Convert pending queries to sync commands
	commands := make([]SyncCommand, len(pendingQueries))
	for i, q := range pendingQueries {
		// Convert params to json.RawMessage
		paramsJSON, _ := json.Marshal(q.Params)
		commands[i] = SyncCommand{
			ID:            q.ID,
			Type:          q.Type,
			Params:        paramsJSON,
			CorrelationID: q.CorrelationID,
		}
	}

	// Update extension version tracking
	if req.ExtensionVersion != "" {
		c.mu.Lock()
		c.extensionVersion = req.ExtensionVersion
		c.mu.Unlock()
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
