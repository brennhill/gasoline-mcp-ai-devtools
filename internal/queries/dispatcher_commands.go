// Purpose: Normalizes extension command status values and handles command lifecycle state transitions.
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

	ch, shouldSignal := func() (chan struct{}, bool) {
		qd.resultsMu.Lock()
		defer qd.resultsMu.Unlock()

		cmd, exists := qd.completedResults[correlationID]
		if !exists {
			return nil, false
		}

		if cmd.Status != "pending" {
			return nil, false
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
			qd.failedCommands = append(qd.failedCommands, cmd)
			if len(qd.failedCommands) > 100 {
				qd.failedCommands = qd.failedCommands[1:]
			}
			delete(qd.completedResults, correlationID)
		}

		ch := qd.commandNotify
		qd.commandNotify = make(chan struct{})
		return ch, true
	}()
	if shouldSignal {
		close(ch)
	}
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
