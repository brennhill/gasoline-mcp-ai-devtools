// Purpose: Implements /sync transport flow for settings, logs, command results, and pending command delivery.
// Why: Consolidates extension-daemon synchronization into a single resilient protocol surface.
// Docs: docs/features/feature/backend-log-streaming/index.md
// Docs: docs/features/feature/query-service/index.md

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

// SyncRequest is the authoritative extension heartbeat payload for /sync.
//
// Invariants:
// - ExtSessionID is stable per extension runtime and changes on reload/update.
// - CommandResults and InProgress are best-effort snapshots; server must tolerate partial batches.
//
// Failure semantics:
// - Missing optional fields are treated as "no update" rather than protocol errors.
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
	PilotEnabled      bool   `json:"pilot_enabled"`
	TrackingEnabled   bool   `json:"tracking_enabled"`
	TrackedTabID      int    `json:"tracked_tab_id"`
	TrackedTabURL     string `json:"tracked_tab_url"`
	TrackedTabTitle   string `json:"tracked_tab_title"`
	TabStatus         string `json:"tab_status,omitempty"`
	TrackedTabActive  *bool  `json:"tracked_tab_active,omitempty"`
	CaptureLogs       bool   `json:"capture_logs"`
	CaptureNetwork    bool   `json:"capture_network"`
	CaptureWebSocket  bool   `json:"capture_websocket"`
	CaptureActions    bool   `json:"capture_actions"`
	CspRestricted     bool   `json:"csp_restricted"`
	CspLevel          string `json:"csp_level"`
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
//
// Invariants:
// - Either ID or CorrelationID must be present after normalization.
// - Status is normalized to lower-case running/pending vocabulary.
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

	s := syncConnectionState{
		wasConnected:      c.extensionState.lastExtensionConnected,
		timeSinceLastPoll: now.Sub(c.extensionState.lastPollAt),
	}
	s.wasDisconnected = !c.extensionState.lastSyncSeen.IsZero() && now.Sub(c.extensionState.lastSyncSeen) >= extensionDisconnectThreshold
	// A reconnect should mean we actually crossed the disconnect threshold,
	// not merely that polls are slower than a short interval.
	s.isReconnect = s.wasDisconnected

	c.extensionState.lastPollAt = now
	c.extensionState.lastExtensionConnected = true
	c.extensionState.lastSyncSeen = now
	c.extensionState.lastSyncClientID = clientID

	if req.ExtSessionID != "" && req.ExtSessionID != c.extensionState.extSessionID {
		c.extensionState.extSessionID = req.ExtSessionID
		c.extensionState.extSessionChangedAt = now
	}
	s.extSessionID = c.extensionState.extSessionID

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
	s.pilotEnabled = c.extensionState.pilotEnabled
	s.inProgressCount = len(c.extensionState.inProgress)
	return s
}

// processSyncCommandResults applies extension result/status updates.
//
// Invariants:
// - Correlated commands use status from ApplyCommandResult as source of truth.
//
// Failure semantics:
// - Unknown query/command IDs are ignored to keep sync idempotent.
// - Query results can be stored even if lifecycle completion arrives separately.
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

// updateSyncLogs ingests extension logs and metadata under c.mu.
//
// Invariants:
// - Extension log buffer uses amortized compaction (at 1.5x capacity) to avoid per-entry copying.
// - Redaction is applied before logs enter persistent in-memory buffers.
//
// Failure semantics:
// - Invalid/missing timestamps are normalized to server receive time.
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
		c.extensionLogs.logs = append(c.extensionLogs.logs, log)
		// Amortized eviction: only compact when buffer exceeds 1.5x capacity.
		evictionThreshold := MaxExtensionLogs + MaxExtensionLogs/2
		if len(c.extensionLogs.logs) > evictionThreshold {
			kept := make([]ExtensionLog, MaxExtensionLogs)
			copy(kept, c.extensionLogs.logs[len(c.extensionLogs.logs)-MaxExtensionLogs:])
			c.extensionLogs.logs = kept
		}
	}

	if req.ExtensionVersion != "" {
		c.extensionState.extensionVersion = req.ExtensionVersion
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

// normalizeInProgressList sanitizes extension heartbeat command state for reconciliation.
//
// Invariants:
// - Returns nil only when caller supplied nil (distinguishes "unsupported" vs "empty list").
// - Output is capped to maxInProgress to bound memory and CPU cost per heartbeat.
//
// Failure semantics:
// - Malformed/empty entries are dropped rather than failing the whole sync request.
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

// commandHasStarted returns true once trace evidence indicates extension execution began.
//
// Failure semantics:
// - Missing trace context returns false, which delays desync failure until stronger evidence exists.
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

// reconcileInProgressCommandState detects commands lost after extension acknowledgement.
//
// Invariants:
// - A command is failed only after two consecutive missed heartbeats once "started" is observed.
// - missingInProgressByCorr map is pruned for no-longer-pending commands each cycle.
//
// Failure semantics:
// - nil inProgress means "client does not support heartbeat reporting" and reconciliation is skipped.
// - Desync failures emit terminal command errors so callers do not wait for full timeout.
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
	if c.extensionState.missingInProgressByCorr == nil {
		c.extensionState.missingInProgressByCorr = make(map[string]int)
	}
	for _, cmd := range pending {
		if cmd == nil || cmd.CorrelationID == "" {
			continue
		}
		corr := cmd.CorrelationID
		pendingCorr[corr] = struct{}{}

		if _, ok := active[corr]; ok {
			delete(c.extensionState.missingInProgressByCorr, corr)
			continue
		}
		if !commandHasStarted(cmd) {
			continue
		}
		c.extensionState.missingInProgressByCorr[corr]++
		if c.extensionState.missingInProgressByCorr[corr] >= 2 {
			toFail = append(toFail, corr)
			toFailIDs = append(toFailIDs, cmd.QueryID)
			delete(c.extensionState.missingInProgressByCorr, corr)
		}
	}

	for corr := range c.extensionState.missingInProgressByCorr {
		if _, stillPending := pendingCorr[corr]; !stillPending {
			delete(c.extensionState.missingInProgressByCorr, corr)
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

// HandleSync processes extension heartbeats and command transport in one endpoint.
//
// Invariants:
// - Connection state is updated before command/result reconciliation.
// - Lifecycle callbacks are emitted out-of-lock via util.SafeGo.
//
// Failure semantics:
// - Invalid JSON returns 400 and does not mutate capture state.
// - Extension disconnect transitions expire pending queries to avoid indefinite LLM waits.
// - Long-poll returns within bounded timeout even when no commands are queued.
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
		c.queryDispatcher.ExpireAllPendingQueries("extension_disconnected")
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
