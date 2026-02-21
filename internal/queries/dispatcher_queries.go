// dispatcher_queries.go — Pending query queue management for extension <-> server RPC.
// Implements the async queue-and-poll pattern where MCP server queues commands
// and extension polls to pick them up. Uses mu for pending query state.
// Async command correlation tracking lives in dispatcher_commands.go (uses resultsMu).
package queries

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/dev-console/dev-console/internal/util"
)

// Constants for query management
const (
	QueryResultTTL   = 5 * time.Minute // How long to keep query results before cleanup
	MaxPendingQueries = 5               // Max pending queries in queue
)

// ============================================
// Pending Query Creation
// ============================================

// CreatePendingQuery creates a pending query with default timeout and no client ID.
// Returns the query ID that extension will use to post the result.
func (qd *QueryDispatcher) CreatePendingQuery(query PendingQuery) string {
	return qd.CreatePendingQueryWithTimeout(query, qd.queryTimeout, "")
}

// CreatePendingQueryWithClient creates a pending query for a specific client.
// Used in multi-client mode to isolate queries between different MCP clients.
func (qd *QueryDispatcher) CreatePendingQueryWithClient(query PendingQuery, clientID string) string {
	return qd.CreatePendingQueryWithTimeout(query, qd.queryTimeout, clientID)
}

// CreatePendingQueryWithTimeout creates a pending query with custom timeout.
// This is the core implementation that all other CreatePending* methods call.
//
// Flow:
// 1. Generate unique query ID (q-1, q-2, etc.)
// 2. Add to pendingQueries queue (FIFO, max 5)
// 3. Schedule cleanup goroutine after timeout
// 4. Return query ID for extension to use when posting result
func (qd *QueryDispatcher) CreatePendingQueryWithTimeout(query PendingQuery, timeout time.Duration, clientID string) string {
	qd.mu.Lock()

	// Enforce max pending queries (drop oldest if full)
	var droppedCorrelationID string
	if len(qd.pendingQueries) >= MaxPendingQueries {
		dropped := qd.pendingQueries[0]
		droppedCorrelationID = dropped.Query.CorrelationID
		fmt.Fprintf(os.Stderr, "[gasoline] Query queue overflow: dropping query %s (correlation_id=%s)\n",
			dropped.Query.ID, dropped.Query.CorrelationID)
		qd.pendingQueries = qd.pendingQueries[1:]
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
		},
		Expires:  time.Now().Add(timeout),
		ClientID: clientID,
	}

	qd.pendingQueries = append(qd.pendingQueries, entry)
	correlationID := query.CorrelationID
	qd.mu.Unlock()

	// Expire dropped command outside mu lock to respect lock ordering
	if droppedCorrelationID != "" {
		qd.expireCommandWithReason(droppedCorrelationID, "Query queue overflow: command was dropped to make room for newer commands")
	}

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

	return id
}

// WaitForPendingQueries blocks until a pending query is available or timeout.
// Used by /sync long-polling to deliver commands instantly.
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

// cleanExpiredQueries removes expired pending queries.
// MUST be called with qd.mu held (Lock, not RLock).
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

// GetPendingQueries returns all pending queries for extension to execute.
// Used by /sync endpoint to deliver commands to the extension.
// Cleans expired queries before returning.
func (qd *QueryDispatcher) GetPendingQueries() []PendingQueryResponse {
	// First ensure any expired queries are marked as failed
	qd.cleanExpiredCommands()

	qd.mu.Lock()
	defer qd.mu.Unlock()

	// Clean expired queries from pending queue
	qd.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0, len(qd.pendingQueries))
	for _, pq := range qd.pendingQueries {
		result = append(result, pq.Query)
	}
	return result
}

// GetPendingQueriesForClient returns pending queries for a specific client.
// Used in multi-client mode.
func (qd *QueryDispatcher) GetPendingQueriesForClient(clientID string) []PendingQueryResponse {
	// First ensure any expired queries are marked as failed
	qd.cleanExpiredCommands()

	qd.mu.Lock()
	defer qd.mu.Unlock()

	// Clean expired queries from pending queue
	qd.cleanExpiredQueries()

	result := make([]PendingQueryResponse, 0)
	for _, pq := range qd.pendingQueries {
		if pq.ClientID == clientID {
			result = append(result, pq.Query)
		}
	}
	return result
}

// AcknowledgePendingQuery removes delivered queries up to and including queryID.
// last_command_ack semantics are "last processed command", so older entries are
// also considered acknowledged and should not be redelivered.
func (qd *QueryDispatcher) AcknowledgePendingQuery(queryID string) {
	if queryID == "" {
		return
	}

	qd.mu.Lock()
	defer qd.mu.Unlock()

	ackIndex := -1
	for i, pq := range qd.pendingQueries {
		if pq.Query.ID == queryID {
			ackIndex = i
			break
		}
	}
	if ackIndex < 0 {
		return
	}

	remaining := make([]PendingQueryEntry, 0, len(qd.pendingQueries)-ackIndex-1)
	remaining = append(remaining, qd.pendingQueries[ackIndex+1:]...)
	qd.pendingQueries = remaining
}

// ExpireAllPendingQueries expires all pending queries with the given error reason.
// Used when the extension disconnects to prevent queries from hanging indefinitely.
// Collects correlation IDs under mu lock, then expires commands under resultsMu.
// Signals commandNotify once after batch expiration to wake all WaitForCommand waiters.
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
		cmd.Status = "expired"
		cmd.Error = reason
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

// SetQueryResultWithClient stores result with client isolation.
//
// Flow:
// 1. Store result in queryResults map
// 2. Remove from pendingQueries
// 3. Broadcast to wake up any WaitForResult callers
func (qd *QueryDispatcher) SetQueryResultWithClient(id string, result json.RawMessage, clientID string) {
	qd.setQueryResultWithClient(id, result, clientID, true)
}

// SetQueryResultWithClientNoCommandComplete stores result with client isolation
// but does NOT force a correlated command into "complete".
// Use this when command lifecycle status is supplied separately via ApplyCommandResult.
func (qd *QueryDispatcher) SetQueryResultWithClientNoCommandComplete(id string, result json.RawMessage, clientID string) {
	qd.setQueryResultWithClient(id, result, clientID, false)
}

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

// GetQueryResultForClient retrieves result with client isolation.
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

// WaitForResultWithClient waits with client isolation.
// Uses a single wakeup goroutine (not per-iteration) to avoid goroutine explosion.
//
// Flow:
// 1. Check if result already exists
// 2. If not, wait on condition variable
// 3. Recheck periodically (10ms intervals)
// 4. Return result or timeout error
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

// startResultCleanup starts a background goroutine that periodically cleans
// expired query results (QueryResultTTL, currently 5m).
// Returns a stop function that terminates the goroutine.
// Called once during QueryDispatcher initialization; stop func stored in stopCleanup.
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

// cleanExpiredResults removes query results older than QueryResultTTL,
// expires orphaned completedResults entries, and expires stale pending queries.
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
