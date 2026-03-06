// Purpose: Exposes test-only accessors for query dispatcher internals (last pending query, state inspection).
// Why: Allows tests to verify internal state without exporting fields.
// Docs: docs/features/feature/query-service/index.md

package queries

// GetLastPendingQuery returns the most recently created pending query.
// Returns nil if no queries exist. For test verification only.
func (qd *QueryDispatcher) GetLastPendingQuery() *PendingQuery {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	if len(qd.pendingQueries) == 0 {
		return nil
	}
	last := qd.pendingQueries[len(qd.pendingQueries)-1]
	return &PendingQuery{
		Type:          last.Query.Type,
		Params:        last.Query.Params,
		TabID:         last.Query.TabID,
		CorrelationID: last.Query.CorrelationID,
	}
}
