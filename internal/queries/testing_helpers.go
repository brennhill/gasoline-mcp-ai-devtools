// testing_helpers.go — Exported accessors for cross-package test access.
// These methods expose internal state that tests in other packages (e.g., capture)
// need to verify query behavior. Not intended for production use.
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
