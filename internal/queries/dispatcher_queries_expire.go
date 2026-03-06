// Purpose: Expires all queued commands and emits terminal lifecycle updates in batch.
// Docs: docs/features/feature/query-service/index.md

package queries

import "time"

// ExpireAllPendingQueries fails every queued command with a shared reason.
func (qd *QueryDispatcher) ExpireAllPendingQueries(reason string) {
	correlationIDs := func() []string {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		var ids []string
		for _, pending := range qd.pendingQueries {
			if pending.Query.CorrelationID != "" {
				ids = append(ids, pending.Query.CorrelationID)
			}
		}
		qd.pendingQueries = qd.pendingQueries[:0]
		return ids
	}()

	if len(correlationIDs) == 0 {
		return
	}

	ch := func() chan struct{} {
		qd.resultsMu.Lock()
		defer qd.resultsMu.Unlock()
		for _, correlationID := range correlationIDs {
			cmd, exists := qd.completedResults[correlationID]
			if !exists {
				continue
			}
			if cmd.Status != "pending" {
				continue
			}
			cmd.Status = "expired"
			cmd.Error = reason
			now := time.Now()
			qd.appendTraceEventLocked(cmd, traceStageTimedOut, "timeout", "expired", reason, now)
			cmd.CompletedAt = now
			qd.failedCommands = append(qd.failedCommands, cmd)
			if len(qd.failedCommands) > 100 {
				qd.failedCommands = qd.failedCommands[1:]
			}
			delete(qd.completedResults, correlationID)
		}

		ch := qd.commandNotify
		qd.commandNotify = make(chan struct{})
		return ch
	}()
	close(ch)
}
