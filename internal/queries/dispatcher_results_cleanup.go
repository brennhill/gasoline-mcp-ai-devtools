// Purpose: Runs periodic TTL-based cleanup of expired query results and pending queries.
// Why: Separates background cleanup lifecycle from result storage and wait logic.
package queries

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// startResultCleanup starts periodic TTL/lifecycle reconciliation.
//
// Invariants:
// - Returned stop func is single-use and owned by QueryDispatcher.Close.
//
// Failure semantics:
// - Cleanup failures are contained (no panic path); next ticker cycle retries.
func (qd *QueryDispatcher) startResultCleanup() func() {
	stop := make(chan struct{})
	util.SafeGo(func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				qd.cleanExpiredResults()
			case <-stop:
				return
			}
		}
	})
	return func() { close(stop) }
}

// cleanExpiredResults prunes stale query results and reconciles orphaned commands.
//
// Invariants:
// - Runs in three lock phases: mu-only collection, out-of-lock expiration, resultsMu cleanup.
//
// Failure semantics:
// - Orphaned correlations are expired rather than silently dropped to preserve LLM-visible outcomes.
func (qd *QueryDispatcher) cleanExpiredResults() {
	now := time.Now()

	// 1. Clean expired query results and collect orphaned pending correlations (under mu).
	orphanedCorrelationIDs := func() []string {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		for id, entry := range qd.queryResults {
			if now.Sub(entry.CreatedAt) > QueryResultTTL {
				delete(qd.queryResults, id)
			}
		}

		var orphaned []string
		gracePeriod := AsyncCommandTimeout + 10*time.Second
		remaining := qd.pendingQueries[:0]
		for _, pq := range qd.pendingQueries {
			if now.Sub(pq.Expires) > gracePeriod {
				if pq.Query.CorrelationID != "" {
					orphaned = append(orphaned, pq.Query.CorrelationID)
				}
				// Drop from pending.
				continue
			}
			remaining = append(remaining, pq)
		}
		qd.pendingQueries = remaining
		return orphaned
	}()

	// 3. Expire orphaned pending commands (outside mu to respect lock ordering)
	for _, correlationID := range orphanedCorrelationIDs {
		qd.ExpireCommand(correlationID)
	}

	// 4. Clean stale completedResults entries (under resultsMu).
	func() {
		qd.resultsMu.Lock()
		defer qd.resultsMu.Unlock()
		for id, cmd := range qd.completedResults {
			if now.Sub(cmd.CreatedAt) > QueryResultTTL {
				delete(qd.completedResults, id)
			}
		}
	}()
}
