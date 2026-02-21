// dispatcher_commands.go — Async command correlation tracker for extension command lifecycle.
// Tracks command registration, completion, expiry, and failure using resultsMu.
// Separated from dispatcher_queries.go which owns the pending query queue (mu).
package queries

import (
	"encoding/json"
	"strings"
	"time"
)

// NormalizeCommandStatus normalizes raw extension status strings to canonical values.
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
		// Preserve current behavior for unknown values by treating as complete.
		return "complete"
	}
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

// RegisterCommand creates a "pending" CommandResult for an async command.
// Called when command is queued. Uses resultsMu (separate from mu).
func (qd *QueryDispatcher) RegisterCommand(correlationID string, queryID string, timeout time.Duration) {
	if correlationID == "" {
		return // No correlation ID = not an async command
	}

	qd.resultsMu.Lock()
	defer qd.resultsMu.Unlock()

	qd.completedResults[correlationID] = &CommandResult{
		CorrelationID: correlationID,
		Status:        "pending",
		CreatedAt:     time.Now(),
	}
}

// ApplyCommandResult updates command state from extension status values.
// Normalized statuses: pending, complete, error, timeout, expired, cancelled.
func (qd *QueryDispatcher) ApplyCommandResult(correlationID string, status string, result json.RawMessage, err string) {
	if correlationID == "" {
		return
	}

	normalizedStatus := NormalizeCommandStatus(status)

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
	if normalizedStatus != "pending" {
		cmd.CompletedAt = time.Now()
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

// CompleteCommand updates a command's status to "complete" with result.
// Called when extension posts result back. Signals any WaitForCommand waiters.
func (qd *QueryDispatcher) CompleteCommand(correlationID string, result json.RawMessage, err string) {
	qd.ApplyCommandResult(correlationID, "complete", result, err)
}

// ExpireCommand marks a command as "expired" and moves it to failedCommands.
// Called by cleanup goroutine when command times out without result.
// Signals commandNotify to wake any WaitForCommand waiters.
func (qd *QueryDispatcher) ExpireCommand(correlationID string) {
	qd.ApplyCommandResult(correlationID, "expired", nil, "Command expired before extension could execute it")
}

// expireCommandWithReason marks a command as expired with a custom error message.
// Similar to ExpireCommand but allows specifying the error reason.
// Signals commandNotify to wake any WaitForCommand waiters.
func (qd *QueryDispatcher) expireCommandWithReason(correlationID string, reason string) {
	if correlationID == "" {
		return
	}
	qd.ApplyCommandResult(correlationID, "expired", nil, reason)
}

// WaitForCommand blocks until the command completes or timeout expires.
// Returns (CommandResult, found). If still pending after timeout, returns the pending result.
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

// GetCommandResult retrieves command status by correlation ID.
// Returns a snapshot copy of the CommandResult (safe to read without locks).
// Used by toolObserveCommandResult and WaitForCommand.
func (qd *QueryDispatcher) GetCommandResult(correlationID string) (*CommandResult, bool) {
	// First ensure any expired queries are marked as failed
	qd.cleanExpiredCommands()

	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	// Check active commands
	if cmd, exists := qd.completedResults[correlationID]; exists {
		cp := *cmd
		return &cp, true
	}

	// Check failed/expired commands
	for _, cmd := range qd.failedCommands {
		if cmd.CorrelationID == correlationID {
			cp := *cmd
			return &cp, true
		}
	}

	return nil, false
}

// cleanExpiredCommands marks any pending commands with expired queries as expired.
// Called by command getter methods to ensure consistency.
// MUST NOT hold any locks when called (may acquire resultsMu).
func (qd *QueryDispatcher) cleanExpiredCommands() {
	qd.mu.Lock()
	now := time.Now()
	var expiredCorrelationIDs []string

	for _, pq := range qd.pendingQueries {
		if !pq.Expires.After(now) && pq.Query.CorrelationID != "" {
			expiredCorrelationIDs = append(expiredCorrelationIDs, pq.Query.CorrelationID)
		}
	}
	qd.mu.Unlock()

	// Mark expired commands
	for _, correlationID := range expiredCorrelationIDs {
		qd.ExpireCommand(correlationID)
	}
}

// GetPendingCommands returns all commands with status "pending".
// Used by toolObservePendingCommands.
func (qd *QueryDispatcher) GetPendingCommands() []*CommandResult {
	// First ensure any expired queries are marked as failed
	qd.cleanExpiredCommands()

	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*CommandResult, 0)
	for _, cmd := range qd.completedResults {
		if cmd.Status == "pending" {
			result = append(result, cmd)
		}
	}
	return result
}

// GetCompletedCommands returns all commands with status "complete".
// Used by toolObservePendingCommands.
func (qd *QueryDispatcher) GetCompletedCommands() []*CommandResult {
	// First ensure any expired queries are marked as failed
	qd.cleanExpiredCommands()

	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*CommandResult, 0)
	for _, cmd := range qd.completedResults {
		if cmd.Status == "complete" {
			result = append(result, cmd)
		}
	}
	return result
}

// GetFailedCommands returns recent failed/expired commands.
// Used by toolObserveFailedCommands.
func (qd *QueryDispatcher) GetFailedCommands() []*CommandResult {
	// First ensure any expired queries are marked as failed
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
		cp := *cmd
		result = append(result, &cp)
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
		cp := *cmd
		result = append(result, &cp)
	}
	return result
}

// HasRecentCSPRestriction returns true when recent command history indicates
// the tracked page is likely blocking script execution due to CSP/restricted context.
func (qd *QueryDispatcher) HasRecentCSPRestriction() bool {
	qd.cleanExpiredCommands()

	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	latest := time.Time{}
	for _, cmd := range qd.failedCommands {
		latest = newestCSPTimestamp(latest, cmd)
	}
	for _, cmd := range qd.completedResults {
		latest = newestCSPTimestamp(latest, cmd)
	}
	if latest.IsZero() {
		return false
	}
	return time.Since(latest) <= QueryResultTTL
}

func newestCSPTimestamp(current time.Time, cmd *CommandResult) time.Time {
	if cmd == nil || !commandLooksLikeCSP(cmd) {
		return current
	}

	ts := cmd.CompletedAt
	if ts.IsZero() {
		ts = cmd.CreatedAt
	}
	if ts.After(current) {
		return ts
	}
	return current
}

func commandLooksLikeCSP(cmd *CommandResult) bool {
	if cmd == nil {
		return false
	}

	if looksLikeCSPString(cmd.Error) {
		return true
	}

	if len(cmd.Result) == 0 {
		return false
	}

	var payload map[string]any
	if err := json.Unmarshal(cmd.Result, &payload); err != nil {
		return false
	}

	if v, ok := payload["csp_blocked"].(bool); ok && v {
		return true
	}
	if v, ok := payload["failure_cause"].(string); ok && strings.EqualFold(strings.TrimSpace(v), "csp") {
		return true
	}
	for _, key := range []string{"error", "message", "hint"} {
		if v, ok := payload[key].(string); ok && looksLikeCSPString(v) {
			return true
		}
	}

	return false
}

func looksLikeCSPString(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return false
	}

	return strings.Contains(v, "csp") ||
		strings.Contains(v, "content security policy") ||
		strings.Contains(v, "trusted type") ||
		strings.Contains(v, "unsafe-eval") ||
		strings.Contains(v, "blocks content scripts") ||
		strings.Contains(v, "blocked content scripts") ||
		strings.Contains(v, "blocks script execution") ||
		strings.Contains(v, "restricted_page")
}
