// Purpose: Snapshots/acknowledges pending query queue state for extension polling.
// Docs: docs/features/feature/query-service/index.md

package queries

import "time"

// cleanExpiredQueries removes past-deadline queue entries in-place.
func (qd *QueryDispatcher) cleanExpiredQueries() {
	now := time.Now()
	remaining := qd.pendingQueries[:0]

	for _, pending := range qd.pendingQueries {
		if pending.Expires.After(now) {
			remaining = append(remaining, pending)
		}
	}
	qd.pendingQueries = remaining
}

// ============================================
// Query Retrieval (Extension Polling)
// ============================================

type pendingQuerySnapshot struct {
	result             []PendingQueryResponse
	sentCorrelationIDs []string
}

func (qd *QueryDispatcher) snapshotPendingQueries(clientID string) pendingQuerySnapshot {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	qd.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0, len(qd.pendingQueries))
	sentCorrelationIDs := make([]string, 0, len(qd.pendingQueries))
	for _, pending := range qd.pendingQueries {
		if clientID != "" && pending.ClientID != clientID {
			continue
		}
		result = append(result, pending.Query)
		if pending.Query.CorrelationID != "" {
			sentCorrelationIDs = append(sentCorrelationIDs, pending.Query.CorrelationID)
		}
	}
	return pendingQuerySnapshot{
		result:             result,
		sentCorrelationIDs: sentCorrelationIDs,
	}
}

// GetPendingQueries snapshots all currently deliverable queued commands.
func (qd *QueryDispatcher) GetPendingQueries() []PendingQueryResponse {
	snapshot := qd.snapshotPendingQueries("")

	for _, correlationID := range snapshot.sentCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageSent, "sync", "pending", "", time.Now())
	}
	return snapshot.result
}

// GetPendingQueriesForClient snapshots queued commands scoped to one client.
func (qd *QueryDispatcher) GetPendingQueriesForClient(clientID string) []PendingQueryResponse {
	snapshot := qd.snapshotPendingQueries(clientID)

	for _, correlationID := range snapshot.sentCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageSent, "sync", "pending", "", time.Now())
	}
	return snapshot.result
}

// AcknowledgePendingQuery advances queue head through queryID (inclusive).
func (qd *QueryDispatcher) AcknowledgePendingQuery(queryID string) {
	if queryID == "" {
		return
	}

	type acknowledgePlan struct {
		acknowledged          bool
		startedCorrelationIDs []string
	}
	plan := func() acknowledgePlan {
		qd.mu.Lock()
		defer qd.mu.Unlock()

		ackIndex := -1
		for i, pending := range qd.pendingQueries {
			if pending.Query.ID == queryID {
				ackIndex = i
				break
			}
		}
		if ackIndex < 0 {
			return acknowledgePlan{}
		}

		startedCorrelationIDs := make([]string, 0, ackIndex+1)
		for _, pending := range qd.pendingQueries[:ackIndex+1] {
			if pending.Query.CorrelationID != "" {
				startedCorrelationIDs = append(startedCorrelationIDs, pending.Query.CorrelationID)
			}
		}

		remaining := make([]PendingQueryEntry, 0, len(qd.pendingQueries)-ackIndex-1)
		remaining = append(remaining, qd.pendingQueries[ackIndex+1:]...)
		qd.pendingQueries = remaining
		return acknowledgePlan{
			acknowledged:          true,
			startedCorrelationIDs: startedCorrelationIDs,
		}
	}()
	if !plan.acknowledged {
		return
	}
	for _, correlationID := range plan.startedCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageStarted, "sync", "pending", "", time.Now())
	}
}
