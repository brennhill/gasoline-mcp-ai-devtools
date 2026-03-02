// Purpose: Stores, retrieves, and expires query result payloads for synchronous/asynchronous tool paths.
// Why: Separates queueing logic from result-lifecycle logic to keep QueryDispatcher modules focused.
// Docs: docs/features/feature/query-service/index.md
//
// Layout:
// - dispatcher_results_store.go: result insertion and retrieval
// - dispatcher_results_wait.go: blocking wait helpers
// - dispatcher_results_cleanup.go: TTL cleanup and orphan reconciliation
// - dispatcher_results.go: timeout/queue configuration accessors

package queries

import "time"

// SetQueryTimeout sets the default timeout for queries.
func (qd *QueryDispatcher) SetQueryTimeout(timeout time.Duration) {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	qd.queryTimeout = timeout
}

// GetQueryTimeout returns the current query timeout.
func (qd *QueryDispatcher) GetQueryTimeout() time.Duration {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	return qd.queryTimeout
}

// QueuePosition returns the 0-based position of a command in the pending queue
// by correlation ID, or -1 if not found. Used for progress reporting.
func (qd *QueryDispatcher) QueuePosition(correlationID string) int {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	for i, pq := range qd.pendingQueries {
		if pq.Query.CorrelationID == correlationID {
			return i
		}
	}
	return -1
}

// QueueDepth returns the current number of pending queries.
func (qd *QueryDispatcher) QueueDepth() int {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	return len(qd.pendingQueries)
}
