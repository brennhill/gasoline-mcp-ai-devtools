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
	"strings"
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
	ExtSessionID string `json:"ext_session_id"`

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

	// Active commands currently executing in the extension.
	// Used to reconcile server/extension state and detect silent command loss.
	InProgress []SyncInProgress `json:"in_progress,omitempty"`
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
	Status        string          `json:"status"` // "complete", "error", "timeout", "cancelled"
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
}

// SyncInProgress represents extension-reported active command execution state.
type SyncInProgress struct {
	ID            string   `json:"id"`
	CorrelationID string   `json:"correlation_id,omitempty"`
	Type          string   `json:"type,omitempty"`
	Status        string   `json:"status,omitempty"` // running | pending
	ProgressPct   *float64 `json:"progress_pct,omitempty"`
	StartedAt     string   `json:"started_at,omitempty"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
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
	TraceID       string          `json:"trace_id,omitempty"`
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
	extSessionID      string
	pilotEnabled      bool
	inProgressCount   int
}

// updateSyncConnectionState updates connection state under lock and returns captured state.
func (c *Capture) updateSyncConnectionState(req SyncRequest, clientID string, now time.Time) syncConnectionState {
	c.mu.Lock()
	defer c.mu.Unlock()

	s := syncConnectionState{
		wasConnected:      c.ext.lastExtensionConnected,
		timeSinceLastPoll: now.Sub(c.ext.lastPollAt),
	}
	s.wasDisconnected = !c.ext.lastSyncSeen.IsZero() && now.Sub(c.ext.lastSyncSeen) >= extensionDisconnectThreshold
	// A reconnect should mean we actually crossed the disconnect threshold,
	// not merely that polls are slower than a short interval.
	s.isReconnect = s.wasDisconnected

	c.ext.lastPollAt = now
	c.ext.lastExtensionConnected = true
	c.ext.lastSyncSeen = now
	c.ext.lastSyncClientID = clientID

	if req.ExtSessionID != "" && req.ExtSessionID != c.ext.extSessionID {
		c.ext.extSessionID = req.ExtSessionID
		c.ext.extSessionChangedAt = now
	}
	s.extSessionID = c.ext.extSessionID

	if req.Settings != nil {
		c.ext.pilotEnabled = req.Settings.PilotEnabled
		c.ext.pilotStatusKnown = true
		c.ext.pilotUpdatedAt = now
		c.ext.pilotSource = PilotSourceExtensionSync
		c.ext.trackingEnabled = req.Settings.TrackingEnabled
		c.ext.trackedTabID = req.Settings.TrackedTabID
		c.ext.trackedTabURL = req.Settings.TrackedTabURL
		c.ext.trackedTabTitle = req.Settings.TrackedTabTitle
		c.ext.trackingUpdated = now
	}
	if req.InProgress != nil {
		c.ext.inProgress = normalizeInProgressList(req.InProgress)
		c.ext.inProgressUpdated = now
	}
	s.pilotEnabled = c.ext.pilotEnabled
	s.inProgressCount = len(c.ext.inProgress)
	return s
}

// processSyncCommandResults processes command results from the extension.
func (c *Capture) processSyncCommandResults(results []SyncCommandResult, clientID string) {
	for _, result := range results {
		if result.ID != "" {
			if result.CorrelationID != "" {
				// Correlated async commands carry explicit lifecycle status below.
				// Do not force "complete" from query-id bookkeeping.
				c.SetQueryResultWithClientNoCommandComplete(result.ID, result.Result, clientID)
			} else {
				c.SetQueryResultWithClient(result.ID, result.Result, clientID)
			}
		}
		if result.CorrelationID != "" {
			c.ApplyCommandResult(result.CorrelationID, result.Status, result.Result, result.Error)
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
		ExtSessionID: req.ExtSessionID,
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
			TraceID:       q.TraceID,
		}
	}
	return commands
}

func shouldEmitSyncSnapshot(req SyncRequest, state syncConnectionState, commandsOut int) bool {
	if state.isReconnect || state.wasDisconnected || !state.wasConnected {
		return true
	}
	if len(req.CommandResults) > 0 || commandsOut > 0 {
		return true
	}
	if req.LastCommandAck != "" {
		return true
	}
	return false
}

func normalizeInProgressList(in []SyncInProgress) []SyncInProgress {
	if in == nil {
		return nil
	}
	if len(in) == 0 {
		return []SyncInProgress{}
	}
	const maxInProgress = 100
	limit := len(in)
	if limit > maxInProgress {
		limit = maxInProgress
	}
	out := make([]SyncInProgress, 0, limit)
	for i := 0; i < limit; i++ {
		entry := in[i]
		entry.ID = strings.TrimSpace(entry.ID)
		entry.CorrelationID = strings.TrimSpace(entry.CorrelationID)
		entry.Type = strings.TrimSpace(entry.Type)
		entry.Status = strings.TrimSpace(strings.ToLower(entry.Status))
		if entry.Status == "" {
			entry.Status = "running"
		}
		if entry.ProgressPct != nil {
			p := *entry.ProgressPct
			if p < 0 {
				p = 0
			}
			if p > 100 {
				p = 100
			}
			entry.ProgressPct = &p
		}
		if entry.ID == "" && entry.CorrelationID == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func commandHasStarted(cmd *queries.CommandResult) bool {
	if cmd == nil {
		return false
	}
	for _, evt := range cmd.TraceEvents {
		if evt.Stage == "started" || evt.Stage == "resolved" || evt.Stage == "errored" || evt.Stage == "timed_out" {
			return true
		}
	}
	return strings.Contains(cmd.TraceTimeline, "started")
}

func (c *Capture) reconcileInProgressCommandState(inProgress []SyncInProgress) {
	if inProgress == nil {
		// Older extension/client that doesn't report in_progress yet.
		return
	}

	active := make(map[string]struct{}, len(inProgress))
	for _, entry := range inProgress {
		if entry.CorrelationID != "" {
			active[entry.CorrelationID] = struct{}{}
		}
	}

	pending := c.GetPendingCommands()
	pendingCorr := make(map[string]struct{}, len(pending))
	toFail := make([]string, 0)
	toFailIDs := make([]string, 0)

	c.mu.Lock()
	if c.ext.missingInProgressByCorr == nil {
		c.ext.missingInProgressByCorr = make(map[string]int)
	}
	for _, cmd := range pending {
		if cmd == nil || cmd.CorrelationID == "" {
			continue
		}
		corr := cmd.CorrelationID
		pendingCorr[corr] = struct{}{}

		if _, ok := active[corr]; ok {
			delete(c.ext.missingInProgressByCorr, corr)
			continue
		}
		if !commandHasStarted(cmd) {
			continue
		}
		c.ext.missingInProgressByCorr[corr]++
		if c.ext.missingInProgressByCorr[corr] >= 2 {
			toFail = append(toFail, corr)
			toFailIDs = append(toFailIDs, cmd.QueryID)
			delete(c.ext.missingInProgressByCorr, corr)
		}
	}

	for corr := range c.ext.missingInProgressByCorr {
		if _, stillPending := pendingCorr[corr]; !stillPending {
			delete(c.ext.missingInProgressByCorr, corr)
		}
	}
	c.mu.Unlock()

	for i, corr := range toFail {
		queryID := ""
		if i < len(toFailIDs) {
			queryID = toFailIDs[i]
		}
		c.ApplyCommandResult(
			corr,
			"error",
			nil,
			"extension_lost_command: command acknowledged by extension but missing from in_progress heartbeats",
		)
		util.SafeGo(func() {
			c.emitLifecycleEvent("command_state_desync", map[string]any{
				"correlation_id": corr,
				"query_id":       queryID,
				"reason":         "missing_in_progress_heartbeat",
			})
		})
	}
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
				"ext_session_id":     state.extSessionID,
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
				"ext_session_id": state.extSessionID,
				"client_id":      clientID,
			})
		})
	}

	// Reconcile started commands against extension heartbeat state.
	// If a command disappears from heartbeat in_progress without a terminal result,
	// fail it fast instead of waiting for eventual timeout.
	c.reconcileInProgressCommandState(req.InProgress)

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
	if shouldEmitSyncSnapshot(req, state, len(commands)) {
		util.SafeGo(func() {
			c.emitLifecycleEvent("sync_snapshot", map[string]any{
				"ext_session_id":       state.extSessionID,
				"client_id":            clientID,
				"pilot_enabled":        state.pilotEnabled,
				"in_progress_count":    state.inProgressCount,
				"pending_commands_out": len(commands),
				"command_results_in":   len(req.CommandResults),
				"last_command_ack":     req.LastCommandAck,
				"next_poll_ms":         nextPollMs,
			})
		})
	}

	resp := SyncResponse{
		Ack:              true,
		Commands:         commands,
		NextPollMs:       nextPollMs,
		ServerTime:       now.Format(time.RFC3339),
		ServerVersion:    c.GetServerVersion(),
		CaptureOverrides: c.buildCaptureOverrides(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (c *Capture) buildCaptureOverrides() map[string]string {
	mode, productionParity, rewrites := c.GetSecurityMode()
	if mode == SecurityModeNormal {
		return map[string]string{}
	}

	overrides := map[string]string{
		"security_mode":     mode,
		"production_parity": "false",
	}
	if productionParity {
		overrides["production_parity"] = "true"
	}
	if len(rewrites) > 0 {
		overrides["insecure_rewrites_applied"] = strings.Join(rewrites, ",")
	}
	return overrides
}
