// Purpose: Creates pending queries, delivers them to the extension, and stores/retrieves one-time query results.
// Docs: docs/features/feature/query-service/index.md

package queries

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// ErrQueueFull is returned when a new command is rejected because the queue is at capacity.
// Callers should return an immediate error to the LLM so it knows the command was not accepted.
var ErrQueueFull = errors.New("queue_full")

// Constants for query management
const (
	QueryResultTTL    = 5 * time.Minute // How long to keep query results before cleanup
	MaxPendingQueries = 15              // Max pending queries in queue
)

// ============================================
// Pending Query Creation
// ============================================

// CreatePendingQuery creates a pending query with default timeout and no client ID.
// Returns the query ID that extension will use to post the result, or ErrQueueFull.
func (qd *QueryDispatcher) CreatePendingQuery(query PendingQuery) (string, error) {
	return qd.CreatePendingQueryWithTimeout(query, qd.queryTimeout, "")
}

// CreatePendingQueryWithClient creates a pending query for a specific client.
// Used in multi-client mode to isolate queries between different MCP clients.
func (qd *QueryDispatcher) CreatePendingQueryWithClient(query PendingQuery, clientID string) (string, error) {
	return qd.CreatePendingQueryWithTimeout(query, qd.queryTimeout, clientID)
}

// CreatePendingQueryWithTimeout enqueues one command for extension pickup.
// This is the core implementation that all other CreatePending* methods call.
//
// Returns ErrQueueFull if the queue is at capacity. When rejected, the command
// is registered and immediately failed so callers using MaybeWaitForCommand
// get an instant "queue_full" error without waiting.
//
// Invariants:
// - Queue is strict FIFO and capped at MaxPendingQueries.
// - queryIDCounter is monotonic within process lifetime.
// - resultsMu is never acquired while mu is held.
//
// Failure semantics:
// - Queue saturation rejects new command; existing pending queue is never truncated.
// - Rejected correlated commands still produce terminal error lifecycle events.
//
// Flow:
// 1. Check queue capacity — reject if full (never drop existing commands)
// 2. Generate unique query ID (q-1, q-2, etc.)
// 3. Add to pendingQueries queue (FIFO, max 15)
// 4. Return query ID for extension to use when posting result
func (qd *QueryDispatcher) CreatePendingQueryWithTimeout(query PendingQuery, timeout time.Duration, clientID string) (string, error) {
	qd.mu.Lock()

	// Reject new commands when queue is full — never drop existing commands.
	// Silent drops corrupt LLM state because the caller believes the command is queued.
	if len(qd.pendingQueries) >= MaxPendingQueries {
		correlationID := query.CorrelationID
		qd.mu.Unlock()

		fmt.Fprintf(os.Stderr, "[gasoline] Queue full (%d/%d): rejecting command type=%s correlation_id=%s\n",
			MaxPendingQueries, MaxPendingQueries, query.Type, correlationID)

		// Register and immediately fail the command so MaybeWaitForCommand returns instantly.
		if correlationID != "" {
			qd.RegisterCommand(correlationID, "", timeout)
			qd.ApplyCommandResult(correlationID, "error", nil,
				fmt.Sprintf("Queue full: %d commands pending. Wait for in-flight commands to complete.", MaxPendingQueries))
		}

		return "", ErrQueueFull
	}

	// Generate unique query ID
	qd.queryIDCounter++
	id := fmt.Sprintf("q-%d", qd.queryIDCounter)

	// Create query entry
	entry := PendingQueryEntry{
		Query: PendingQueryResponse{
			ID:            id,
			Type:          query.Type,
			Params:        query.Params,
			TabID:         query.TabID,
			CorrelationID: query.CorrelationID,
			TraceID:       deriveTraceID(query.TraceID, query.CorrelationID, id),
		},
		Expires:  time.Now().Add(timeout),
		ClientID: clientID,
	}

	qd.pendingQueries = append(qd.pendingQueries, entry)
	correlationID := query.CorrelationID
	qd.mu.Unlock()

	// Notify long-pollers that a new query is available
	select {
	case qd.queryNotify <- struct{}{}:
	default:
	}

	// Register command outside mu lock to respect lock ordering (resultsMu must not be acquired under mu)
	if correlationID != "" {
		qd.RegisterCommand(correlationID, id, timeout)
	}

	// Query expires are checked during periodic cleanup cycles, not individual goroutines.
	// This is more efficient than spawning a goroutine per query.

	return id, nil
}

// WaitForPendingQueries blocks until queue is non-empty or timeout elapses.
//
// Invariants:
// - queryNotify is edge-triggered/best-effort; callers must re-check queue after wakeup.
//
// Failure semantics:
// - Spurious wakeups are expected; method provides no guarantee a command is available on return.
func (qd *QueryDispatcher) WaitForPendingQueries(timeout time.Duration) {
	qd.mu.Lock()
	if len(qd.pendingQueries) > 0 {
		qd.mu.Unlock()
		return
	}
	qd.mu.Unlock()

	select {
	case <-qd.queryNotify:
	case <-time.After(timeout):
	}
}

// ============================================
// Query Cleanup
// ============================================

// cleanExpiredQueries removes past-deadline queue entries in-place.
//
// Invariants:
// - Caller must hold qd.mu (Lock, not RLock).
// - Surviving slice preserves original order.
//
// Failure semantics:
// - Only trims pending queue; command lifecycle expiration is handled separately.
func (qd *QueryDispatcher) cleanExpiredQueries() {
	now := time.Now()
	remaining := qd.pendingQueries[:0]

	for _, pq := range qd.pendingQueries {
		if pq.Expires.After(now) {
			remaining = append(remaining, pq)
		}
	}
	qd.pendingQueries = remaining
}

// ============================================
// Query Retrieval (Extension Polling)
// ============================================

// GetPendingQueries snapshots all currently deliverable queued commands.
//
// Invariants:
// - Snapshot order matches FIFO queue order.
// - Trace "sent" events are recorded after lock release to avoid lock inversion with resultsMu.
//
// Failure semantics:
// - Expired commands are cleaned before snapshot; callers never receive already-dead entries.
func (qd *QueryDispatcher) GetPendingQueries() []PendingQueryResponse {
	qd.mu.Lock()

	// Clean expired queries from pending queue
	qd.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0, len(qd.pendingQueries))
	sentCorrelationIDs := make([]string, 0, len(qd.pendingQueries))
	for _, pq := range qd.pendingQueries {
		result = append(result, pq.Query)
		if pq.Query.CorrelationID != "" {
			sentCorrelationIDs = append(sentCorrelationIDs, pq.Query.CorrelationID)
		}
	}
	qd.mu.Unlock()

	for _, correlationID := range sentCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageSent, "sync", "pending", "", time.Now())
	}
	return result
}

// GetPendingQueriesForClient snapshots queued commands scoped to one client.
//
// Invariants:
// - Cross-client entries are excluded even if pending globally.
//
// Failure semantics:
// - If client has no queued entries, returns empty slice (not nil).
func (qd *QueryDispatcher) GetPendingQueriesForClient(clientID string) []PendingQueryResponse {
	qd.mu.Lock()

	// Clean expired queries from pending queue
	qd.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0)
	sentCorrelationIDs := make([]string, 0, len(qd.pendingQueries))
	for _, pq := range qd.pendingQueries {
		if pq.ClientID == clientID {
			result = append(result, pq.Query)
			if pq.Query.CorrelationID != "" {
				sentCorrelationIDs = append(sentCorrelationIDs, pq.Query.CorrelationID)
			}
		}
	}
	qd.mu.Unlock()

	for _, correlationID := range sentCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageSent, "sync", "pending", "", time.Now())
	}
	return result
}

// AcknowledgePendingQuery advances queue head through queryID (inclusive).
//
// Invariants:
// - Ack semantics are cumulative; all earlier commands become non-deliverable.
//
// Failure semantics:
// - Unknown queryID is ignored to remain idempotent for delayed/duplicate acks.
func (qd *QueryDispatcher) AcknowledgePendingQuery(queryID string) {
	if queryID == "" {
		return
	}

	qd.mu.Lock()

	ackIndex := -1
	for i, pq := range qd.pendingQueries {
		if pq.Query.ID == queryID {
			ackIndex = i
			break
		}
	}
	if ackIndex < 0 {
		qd.mu.Unlock()
		return
	}

	startedCorrelationIDs := make([]string, 0, ackIndex+1)
	for _, pq := range qd.pendingQueries[:ackIndex+1] {
		if pq.Query.CorrelationID != "" {
			startedCorrelationIDs = append(startedCorrelationIDs, pq.Query.CorrelationID)
		}
	}

	remaining := make([]PendingQueryEntry, 0, len(qd.pendingQueries)-ackIndex-1)
	remaining = append(remaining, qd.pendingQueries[ackIndex+1:]...)
	qd.pendingQueries = remaining

	qd.mu.Unlock()
	for _, correlationID := range startedCorrelationIDs {
		qd.recordTraceEvent(correlationID, traceStageStarted, "sync", "pending", "", time.Now())
	}
}

// ExpireAllPendingQueries fails every queued command with a shared reason.
//
// Invariants:
// - Queue is atomically cleared under mu before terminal updates are emitted.
// - Waiters are signaled once per batch via commandNotify rotation.
//
// Failure semantics:
// - Commands missing from completedResults are skipped (already terminal or never registered).
// - Batch expiration never partially leaves pending queue entries.
func (qd *QueryDispatcher) ExpireAllPendingQueries(reason string) {
	qd.mu.Lock()

	var correlationIDs []string
	for _, pq := range qd.pendingQueries {
		if pq.Query.CorrelationID != "" {
			correlationIDs = append(correlationIDs, pq.Query.CorrelationID)
		}
	}
	// Clear all pending queries
	qd.pendingQueries = qd.pendingQueries[:0]

	qd.mu.Unlock()

	if len(correlationIDs) == 0 {
		return
	}

	// Mark commands as expired with disconnect reason (outside mu lock).
	// Expire individually without signaling, then signal once at the end.
	qd.resultsMu.Lock()
	for _, correlationID := range correlationIDs {
		cmd, exists := qd.completedResults[correlationID]
		if !exists {
			continue
		}
		// Guard: only expire commands still in pending state.
		// Commands already completed by processSyncCommandResults must not be overwritten.
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

	// Signal waiters once for the entire batch
	ch := qd.commandNotify
	qd.commandNotify = make(chan struct{})
	qd.resultsMu.Unlock()
	close(ch)
}

// ============================================
// Result Storage (Extension Posts Results)
// ============================================

// SetQueryResult stores the result for a pending query.
// Called when extension posts result back to server.
func (qd *QueryDispatcher) SetQueryResult(id string, result json.RawMessage) {
	qd.SetQueryResultWithClient(id, result, "")
}

// SetQueryResultWithClient stores a result and marks correlated command complete.
//
// Flow:
// 1. Store result in queryResults map
// 2. Remove from pendingQueries
// 3. Broadcast to wake up any WaitForResult callers
func (qd *QueryDispatcher) SetQueryResultWithClient(id string, result json.RawMessage, clientID string) {
	qd.setQueryResultWithClient(id, result, clientID, true)
}

// SetQueryResultWithClientNoCommandComplete stores result without lifecycle transition.
//
// Invariants:
// - Used when extension reports command status through ApplyCommandResult separately.
//
// Failure semantics:
// - Prevents accidental status downgrade/upgrade caused by out-of-order query/result transport.
func (qd *QueryDispatcher) SetQueryResultWithClientNoCommandComplete(id string, result json.RawMessage, clientID string) {
	qd.setQueryResultWithClient(id, result, clientID, false)
}

// setQueryResultWithClient performs result insertion + pending queue cleanup.
//
// Invariants:
// - queryCond.Broadcast is emitted after map/slice mutation so waiters observe committed state.
//
// Failure semantics:
// - Missing pending entry is tolerated; result is still stored for one-time retrieval.
// - markComplete=false leaves command lifecycle unchanged by design.
func (qd *QueryDispatcher) setQueryResultWithClient(id string, result json.RawMessage, clientID string, markComplete bool) {
	qd.mu.Lock()

	// Store result
	qd.queryResults[id] = QueryResultEntry{
		Result:    result,
		ClientID:  clientID,
		CreatedAt: time.Now(),
	}

	// Find correlation ID before removing from pending
	var correlationID string
	for _, pq := range qd.pendingQueries {
		if pq.Query.ID == id {
			correlationID = pq.Query.CorrelationID
			break
		}
	}

	// Remove from pending
	remaining := qd.pendingQueries[:0]
	for _, pq := range qd.pendingQueries {
		if pq.Query.ID != id {
			remaining = append(remaining, pq)
		}
	}
	qd.pendingQueries = remaining

	qd.mu.Unlock()

	// Wake up waiters
	qd.queryCond.Broadcast()

	// Mark command as complete if it has a correlation ID
	if markComplete && correlationID != "" {
		qd.CompleteCommand(correlationID, result, "")
	}
}

// ============================================
// Result Retrieval
// ============================================

// GetQueryResult retrieves and deletes a query result.
// Returns (result, found).
func (qd *QueryDispatcher) GetQueryResult(id string) (json.RawMessage, bool) {
	return qd.GetQueryResultForClient(id, "")
}

// GetQueryResultForClient retrieves and consumes a result scoped to one client.
//
// Invariants:
// - Successful reads are destructive (single-consumer semantics).
//
// Failure semantics:
// - Client mismatch returns (nil,false) without consuming the stored result.
func (qd *QueryDispatcher) GetQueryResultForClient(id string, clientID string) (json.RawMessage, bool) {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	entry, found := qd.queryResults[id]
	if !found {
		return nil, false
	}

	// Check client isolation
	if clientID != "" && entry.ClientID != clientID {
		return nil, false
	}

	// Delete after retrieval (one-time use)
	delete(qd.queryResults, id)
	return entry.Result, true
}

// ============================================
// Blocking Wait (For Synchronous Tools)
// ============================================

// WaitForResult blocks until result is available or timeout.
// Used by synchronous tool handlers that need immediate results.
func (qd *QueryDispatcher) WaitForResult(id string, timeout time.Duration) (json.RawMessage, error) {
	return qd.WaitForResultWithClient(id, timeout, "")
}

// WaitForResultWithClient waits for one result under client-isolated view.
// Uses a single wakeup goroutine (not per-iteration) to avoid goroutine explosion.
//
// Flow:
// 1. Check if result already exists
// 2. If not, wait on condition variable
// 3. Recheck periodically (10ms intervals)
// 4. Return result or timeout error
//
// Invariants:
// - done channel is always closed before Unlock (defer LIFO) to stop ticker goroutine.
// - qd.queryCond.Wait is called only with qd.mu held.
//
// Failure semantics:
// - Timeout returns deterministic error; caller decides retry/abort policy.
// - Missing result after wakeups is expected (spurious or unrelated broadcasts).
func (qd *QueryDispatcher) WaitForResultWithClient(id string, timeout time.Duration, clientID string) (json.RawMessage, error) {
	deadline := time.Now().Add(timeout)

	// Single wakeup goroutine: broadcasts every 10ms to recheck condition.
	// Replaces per-iteration goroutine spawn that caused ~3000 goroutines per 30s call.
	done := make(chan struct{})
	util.SafeGo(func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				qd.queryCond.Broadcast()
			case <-done:
				return
			}
		}
	})

	qd.mu.Lock()
	defer qd.mu.Unlock()
	defer close(done) // Stop wakeup goroutine on return (runs before Unlock per LIFO)

	for {
		// Check if result exists
		if entry, found := qd.queryResults[id]; found {
			// Check client isolation
			if clientID == "" || entry.ClientID == clientID {
				delete(qd.queryResults, id)
				return entry.Result, nil
			}
		}

		// Check timeout
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for result %s", id)
		}

		qd.queryCond.Wait()
	}
}

// ============================================
// Result Cleanup (Background Goroutine)
// ============================================

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

	// 1. Clean expired query results (under mu)
	qd.mu.Lock()
	for id, entry := range qd.queryResults {
		if now.Sub(entry.CreatedAt) > QueryResultTTL {
			delete(qd.queryResults, id)
		}
	}

	// 2. Collect orphaned pending queries (expired + grace period)
	var orphanedCorrelationIDs []string
	gracePeriod := AsyncCommandTimeout + 10*time.Second
	remaining := qd.pendingQueries[:0]
	for _, pq := range qd.pendingQueries {
		if now.Sub(pq.Expires) > gracePeriod {
			if pq.Query.CorrelationID != "" {
				orphanedCorrelationIDs = append(orphanedCorrelationIDs, pq.Query.CorrelationID)
			}
			// Drop from pending
			continue
		}
		remaining = append(remaining, pq)
	}
	qd.pendingQueries = remaining
	qd.mu.Unlock()

	// 3. Expire orphaned pending commands (outside mu to respect lock ordering)
	for _, correlationID := range orphanedCorrelationIDs {
		qd.ExpireCommand(correlationID)
	}

	// 4. Clean stale completedResults entries (under resultsMu)
	qd.resultsMu.Lock()
	for id, cmd := range qd.completedResults {
		if now.Sub(cmd.CreatedAt) > QueryResultTTL {
			delete(qd.completedResults, id)
		}
	}
	qd.resultsMu.Unlock()
}

// ============================================
// Configuration
// ============================================

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
