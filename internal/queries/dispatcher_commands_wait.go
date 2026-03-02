// Purpose: Provides blocking wait and expiry reconciliation for command lifecycle records.
// Why: Isolates command wait/timeout behavior from status mutation and observer views.
// Docs: docs/features/feature/query-service/index.md

package queries

import "time"

// WaitForCommand blocks for lifecycle transition or timeout.
//
// Invariants:
// - Returns immutable snapshots from GetCommandResult; callers must not rely on pointer identity.
//
// Failure semantics:
// - On timeout, returns latest observed state (possibly still pending).
// - If command was never registered, returns (nil,false) immediately.
func (qd *QueryDispatcher) WaitForCommand(correlationID string, timeout time.Duration) (*CommandResult, bool) {
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

	if cmd, exists := qd.completedResults[correlationID]; exists {
		return copyCommandResultWithTrace(cmd), true
	}

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
	now := time.Now()
	expiredSet := make(map[string]struct{})
	expiredCorrelationIDs := func() []string {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		var ids []string
		for _, pending := range qd.pendingQueries {
			if !pending.Expires.After(now) && pending.Query.CorrelationID != "" {
				if _, seen := expiredSet[pending.Query.CorrelationID]; !seen {
					expiredSet[pending.Query.CorrelationID] = struct{}{}
					ids = append(ids, pending.Query.CorrelationID)
				}
			}
		}
		return ids
	}()

	qd.resultsMu.RLock()
	for correlationID, cmd := range qd.completedResults {
		if cmd.Status != "pending" {
			continue
		}
		expiresAt := cmd.ExpiresAt
		if expiresAt.IsZero() {
			expiresAt = cmd.CreatedAt.Add(AsyncCommandTimeout)
		}
		if !expiresAt.After(now) {
			if _, seen := expiredSet[correlationID]; !seen {
				expiredSet[correlationID] = struct{}{}
				expiredCorrelationIDs = append(expiredCorrelationIDs, correlationID)
			}
		}
	}
	qd.resultsMu.RUnlock()

	for _, correlationID := range expiredCorrelationIDs {
		qd.expireCommandWithReason(correlationID, "Command expired waiting for extension result")
	}
}
