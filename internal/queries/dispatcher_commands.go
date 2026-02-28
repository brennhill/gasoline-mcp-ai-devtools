// Purpose: Implements async command/query dispatch and correlation state tracking.
// Why: Coordinates async command flow so extension/server state stays coherent under concurrency.
// Docs: docs/features/feature/query-service/index.md

package queries

import (
	"encoding/json"
	"strings"
	"time"
)

// NormalizeCommandStatus maps extension-provided status text into canonical lifecycle states.
//
// Failure semantics:
// - Unknown/non-empty status values are coerced to "error" so protocol drift is visible.
// - Empty/success-like statuses normalize to "complete" for backward compatibility.
func NormalizeCommandStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "", "ok", "success", "succeeded", "done":
		return "complete"
	case "pending", "queued", "running", "still_processing":
		return "pending"
	case "complete", "error", "timeout", "expired", "cancelled":
		return normalized
	case "canceled":
		return "cancelled"
	default:
		// Unknown status values are treated as errors to surface protocol drift.
		return "error"
	}
}

// normalizeCommandOutcome reconciles status + error into a single lifecycle outcome.
//
// Invariants:
// - Any non-empty err field takes precedence over success-like statuses.
//
// Failure semantics:
// - Ambiguous payloads (e.g. status=complete + err set) are treated as "error".
func normalizeCommandOutcome(status string, err string) string {
	normalizedStatus := NormalizeCommandStatus(status)
	if strings.TrimSpace(err) != "" && (normalizedStatus == "complete" || normalizedStatus == "pending") {
		return "error"
	}
	return normalizedStatus
}

// IsFailedCommandStatus returns true for terminal failure statuses.
func IsFailedCommandStatus(status string) bool {
	switch status {
	case "error", "timeout", "expired", "cancelled":
		return true
	default:
		return false
	}
}

// ============================================
// Correlation ID Tracking (Async Commands)
// ============================================

// RegisterCommand creates the initial "pending" lifecycle record for an async command.
//
// Invariants:
// - correlationID is the primary key across pending/completed/failed views.
// - ExpiresAt is always set to an absolute deadline (provided timeout or AsyncCommandTimeout).
//
// Failure semantics:
// - Empty correlation IDs are ignored (non-async command path).
// - Existing entries are overwritten intentionally to keep latest queue registration authoritative.
func (qd *QueryDispatcher) RegisterCommand(correlationID string, queryID string, timeout time.Duration) {
	if correlationID == "" {
		return // No correlation ID = not an async command
	}

	now := time.Now()
	expiresAt := now.Add(timeout)
	if timeout <= 0 {
		expiresAt = now.Add(AsyncCommandTimeout)
	}

	qd.resultsMu.Lock()
	defer qd.resultsMu.Unlock()

	cmd := &CommandResult{
		CorrelationID: correlationID,
		TraceID:       deriveTraceID("", correlationID, queryID),
		QueryID:       queryID,
		Status:        "pending",
		CreatedAt:     now,
		ExpiresAt:     expiresAt,
		UpdatedAt:     now,
	}
	qd.appendTraceEventLocked(cmd, traceStageQueued, "queue", "pending", "", now)
	qd.completedResults[correlationID] = cmd
}

// ApplyCommandResult applies one extension lifecycle update to a command record.
//
// Invariants:
// - State transitions are monotonic: once status != pending, subsequent updates are ignored.
// - Signaling is edge-triggered by closing commandNotify then replacing it under resultsMu.
//
// Failure semantics:
// - Unknown correlation IDs are ignored (idempotent for late/duplicate extension callbacks).
// - Failed terminal states are moved to failedCommands and evicted from completedResults.
func (qd *QueryDispatcher) ApplyCommandResult(correlationID string, status string, result json.RawMessage, err string) {
	if correlationID == "" {
		return
	}

	normalizedStatus := normalizeCommandOutcome(status, err)

	qd.resultsMu.Lock()
	cmd, exists := qd.completedResults[correlationID]
	if !exists {
		qd.resultsMu.Unlock()
		return
	}

	// Preserve current race behavior: once no longer pending, do not overwrite.
	if cmd.Status != "pending" {
		qd.resultsMu.Unlock()
		return
	}

	cmd.Status = normalizedStatus
	cmd.Result = result
	cmd.Error = err
	eventAt := time.Now()
	stage := traceStageFromStatus(normalizedStatus)
	if (stage == traceStageResolved || stage == traceStageErrored || stage == traceStageTimedOut) && !qd.hasTraceStageLocked(cmd, traceStageStarted) {
		qd.appendTraceEventLocked(cmd, traceStageStarted, "extension", "pending", "", eventAt)
	}
	qd.appendTraceEventLocked(cmd, stage, "extension", normalizedStatus, err, eventAt)
	if normalizedStatus != "pending" {
		cmd.CompletedAt = eventAt
	}

	if IsFailedCommandStatus(normalizedStatus) {
		// Move failures to failedCommands ring buffer (observe failed_commands source).
		qd.failedCommands = append(qd.failedCommands, cmd)
		if len(qd.failedCommands) > 100 {
			qd.failedCommands = qd.failedCommands[1:]
		}
		delete(qd.completedResults, correlationID)
	}

	// Signal waiters: close current channel, create a fresh one.
	ch := qd.commandNotify
	qd.commandNotify = make(chan struct{})
	qd.resultsMu.Unlock()
	close(ch)
}

// CompleteCommand is compatibility sugar for ApplyCommandResult(..., "complete", ...).
//
// Failure semantics:
// - If err is non-empty, normalizeCommandOutcome downgrades to "error".
func (qd *QueryDispatcher) CompleteCommand(correlationID string, result json.RawMessage, err string) {
	qd.ApplyCommandResult(correlationID, "complete", result, err)
}

// ExpireCommand marks a pending command as expired due to timeout/dequeue loss.
//
// Failure semantics:
// - No-op for unknown/already-terminal commands.
// - Emits terminal "expired" result so waiters unblock with deterministic outcome.
func (qd *QueryDispatcher) ExpireCommand(correlationID string) {
	qd.ApplyCommandResult(correlationID, "expired", nil, "Command expired before extension could execute it")
}

// expireCommandWithReason is ExpireCommand with an explicit diagnostic reason.
//
// Failure semantics:
// - Empty correlation IDs are ignored.
func (qd *QueryDispatcher) expireCommandWithReason(correlationID string, reason string) {
	if correlationID == "" {
		return
	}
	qd.ApplyCommandResult(correlationID, "expired", nil, reason)
}

// WaitForCommand blocks for lifecycle transition or timeout.
//
// Invariants:
// - Returns immutable snapshots from GetCommandResult; callers must not rely on pointer identity.
//
// Failure semantics:
// - On timeout, returns latest observed state (possibly still pending).
// - If command was never registered, returns (nil,false) immediately.
func (qd *QueryDispatcher) WaitForCommand(correlationID string, timeout time.Duration) (*CommandResult, bool) {
	// Check immediately
	cmd, found := qd.GetCommandResult(correlationID)
	if !found || cmd.Status != "pending" {
		return cmd, found
	}

	deadline := time.Now().Add(timeout)
	for {
		qd.resultsMu.RLock()
		ch := qd.commandNotify
		qd.resultsMu.RUnlock()

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return qd.GetCommandResult(correlationID)
		}

		timer := time.NewTimer(remaining)
		select {
		case <-ch:
			timer.Stop()
			cmd, found = qd.GetCommandResult(correlationID)
			if !found || cmd.Status != "pending" {
				return cmd, found
			}
			continue
		case <-timer.C:
			return qd.GetCommandResult(correlationID)
		}
	}
}

// GetCommandResult returns the latest lifecycle snapshot for one correlation ID.
//
// Invariants:
// - Returned value is a detached copy; internal slices/maps stay lock-owned.
// - Eagerly cleans expired commands so wait loops see timely expiration.
//
// Failure semantics:
// - Expired commands are also cleaned by the periodic 30s ticker in startResultCleanup.
func (qd *QueryDispatcher) GetCommandResult(correlationID string) (*CommandResult, bool) {
	qd.cleanExpiredCommands()

	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	// Check active commands
	if cmd, exists := qd.completedResults[correlationID]; exists {
		return copyCommandResultWithTrace(cmd), true
	}

	// Check failed/expired commands
	for _, cmd := range qd.failedCommands {
		if cmd.CorrelationID == correlationID {
			return copyCommandResultWithTrace(cmd), true
		}
	}

	return nil, false
}

// cleanExpiredCommands reconciles pending/queued deadlines into terminal expired states.
//
// Invariants:
// - Must run lock-free at entry; acquires mu/resultsMu internally in ordered phases.
//
// Failure semantics:
// - Safe under races with extension completion: ApplyCommandResult's pending guard prevents double-terminal transitions.
func (qd *QueryDispatcher) cleanExpiredCommands() {
	qd.mu.Lock()
	now := time.Now()
	expiredSet := make(map[string]struct{})
	var expiredCorrelationIDs []string

	for _, pq := range qd.pendingQueries {
		if !pq.Expires.After(now) && pq.Query.CorrelationID != "" {
			if _, seen := expiredSet[pq.Query.CorrelationID]; !seen {
				expiredSet[pq.Query.CorrelationID] = struct{}{}
				expiredCorrelationIDs = append(expiredCorrelationIDs, pq.Query.CorrelationID)
			}
		}
	}
	qd.mu.Unlock()

	// Also expire any command results that were already dequeued to the extension
	// but never received a completion callback.
	qd.resultsMu.RLock()
	for corrID, cmd := range qd.completedResults {
		if cmd.Status != "pending" {
			continue
		}
		expiresAt := cmd.ExpiresAt
		if expiresAt.IsZero() {
			expiresAt = cmd.CreatedAt.Add(AsyncCommandTimeout)
		}
		if !expiresAt.After(now) {
			if _, seen := expiredSet[corrID]; !seen {
				expiredSet[corrID] = struct{}{}
				expiredCorrelationIDs = append(expiredCorrelationIDs, corrID)
			}
		}
	}
	qd.resultsMu.RUnlock()

	// Mark expired commands (idempotent due cmd.Status != "pending" guard).
	for _, correlationID := range expiredCorrelationIDs {
		qd.expireCommandWithReason(correlationID, "Command expired waiting for extension result")
	}
}

// GetPendingCommands returns all commands with status "pending".
// Used by toolObservePendingCommands.
func (qd *QueryDispatcher) GetPendingCommands() []*CommandResult {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*CommandResult, 0)
	for _, cmd := range qd.completedResults {
		if cmd.Status == "pending" {
			result = append(result, copyCommandResultWithTrace(cmd))
		}
	}
	return result
}

// GetCompletedCommands returns all commands with status "complete".
// Used by toolObservePendingCommands.
func (qd *QueryDispatcher) GetCompletedCommands() []*CommandResult {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*CommandResult, 0)
	for _, cmd := range qd.completedResults {
		if cmd.Status == "complete" {
			result = append(result, copyCommandResultWithTrace(cmd))
		}
	}
	return result
}

// GetFailedCommands returns recent failed/expired commands.
// Used by toolObserveFailedCommands.
// Eagerly cleans expired commands so callers see freshly expired entries.
func (qd *QueryDispatcher) GetFailedCommands() []*CommandResult {
	qd.cleanExpiredCommands()

	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	// Return detached copies to avoid concurrent modification and include
	// non-expired failures that remain in completedResults.
	result := make([]*CommandResult, 0, len(qd.failedCommands)+len(qd.completedResults))
	seen := make(map[string]struct{}, len(qd.failedCommands))

	for _, cmd := range qd.failedCommands {
		if cmd == nil {
			continue
		}
		cp := copyCommandResultWithTrace(cmd)
		result = append(result, cp)
		if cp.CorrelationID != "" {
			seen[cp.CorrelationID] = struct{}{}
		}
	}
	for _, cmd := range qd.completedResults {
		if cmd == nil || !IsFailedCommandStatus(cmd.Status) {
			continue
		}
		if _, exists := seen[cmd.CorrelationID]; exists {
			continue
		}
		result = append(result, copyCommandResultWithTrace(cmd))
	}
	return result
}
