// queries.go — Pending query queue management for extension ↔ server RPC.
// Implements the async queue-and-poll pattern where MCP server queues commands
// and extension polls to pick them up.
package capture

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// Constants for query management
const (
	queryResultTTL = 60 * time.Second // How long to keep query results before cleanup
	// Note: maxPendingQueries is defined in types.go (=5)
)

// ============================================
// Pending Query Creation
// ============================================

// CreatePendingQuery creates a pending query with default timeout and no client ID.
// Returns the query ID that extension will use to post the result.
func (qd *QueryDispatcher) CreatePendingQuery(query queries.PendingQuery) string {
	return qd.CreatePendingQueryWithTimeout(query, qd.queryTimeout, "")
}

// CreatePendingQueryWithClient creates a pending query for a specific client.
// Used in multi-client mode to isolate queries between different MCP clients.
func (qd *QueryDispatcher) CreatePendingQueryWithClient(query queries.PendingQuery, clientID string) string {
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
func (qd *QueryDispatcher) CreatePendingQueryWithTimeout(query queries.PendingQuery, timeout time.Duration, clientID string) string {
	qd.mu.Lock()

	// Enforce max pending queries (drop oldest if full)
	if len(qd.pendingQueries) >= maxPendingQueries {
		qd.pendingQueries = qd.pendingQueries[1:]
	}

	// Generate unique query ID
	qd.queryIDCounter++
	id := fmt.Sprintf("q-%d", qd.queryIDCounter)

	// Create query entry
	entry := pendingQueryEntry{
		query: queries.PendingQueryResponse{
			ID:            id,
			Type:          query.Type,
			Params:        query.Params,
			TabID:         query.TabID,
			CorrelationID: query.CorrelationID,
		},
		expires:  time.Now().Add(timeout),
		clientID: clientID,
	}

	qd.pendingQueries = append(qd.pendingQueries, entry)
	correlationID := query.CorrelationID
	qd.mu.Unlock()

	// Register command outside mu lock to respect lock ordering (resultsMu must not be acquired under mu)
	if correlationID != "" {
		qd.RegisterCommand(correlationID, id, timeout)
	}

	// Schedule cleanup after timeout
	go func() {
		time.Sleep(timeout)
		qd.mu.Lock()
		qd.cleanExpiredQueries()
		qd.queryCond.Broadcast()
		qd.mu.Unlock()

		// ExpireCommand acquires resultsMu — called outside mu to respect lock ordering
		if correlationID != "" {
			qd.ExpireCommand(correlationID)
		}
	}()

	return id
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
		if pq.expires.After(now) {
			remaining = append(remaining, pq)
		}
	}
	qd.pendingQueries = remaining
}

// ============================================
// Query Retrieval (Extension Polling)
// ============================================

// GetPendingQueries returns all pending queries for extension to execute.
// Used by HandlePendingQueries HTTP handler.
// Cleans expired queries before returning.
func (qd *QueryDispatcher) GetPendingQueries() []queries.PendingQueryResponse {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	qd.cleanExpiredQueries()

	result := make([]queries.PendingQueryResponse, 0, len(qd.pendingQueries))
	for _, pq := range qd.pendingQueries {
		result = append(result, pq.query)
	}
	return result
}

// GetPendingQueriesForClient returns pending queries for a specific client.
// Used in multi-client mode.
func (qd *QueryDispatcher) GetPendingQueriesForClient(clientID string) []queries.PendingQueryResponse {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	qd.cleanExpiredQueries()

	result := make([]queries.PendingQueryResponse, 0)
	for _, pq := range qd.pendingQueries {
		if pq.clientID == clientID {
			result = append(result, pq.query)
		}
	}
	return result
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
	qd.mu.Lock()

	// Store result
	qd.queryResults[id] = queryResultEntry{
		result:    result,
		clientID:  clientID,
		createdAt: time.Now(),
	}

	// Find correlation ID before removing from pending
	var correlationID string
	for _, pq := range qd.pendingQueries {
		if pq.query.ID == id {
			correlationID = pq.query.CorrelationID
			break
		}
	}

	// Remove from pending
	remaining := qd.pendingQueries[:0]
	for _, pq := range qd.pendingQueries {
		if pq.query.ID != id {
			remaining = append(remaining, pq)
		}
	}
	qd.pendingQueries = remaining

	qd.mu.Unlock()

	// Wake up waiters
	qd.queryCond.Broadcast()

	// Mark command as complete if it has a correlation ID
	if correlationID != "" {
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
	if clientID != "" && entry.clientID != clientID {
		return nil, false
	}

	// Delete after retrieval (one-time use)
	delete(qd.queryResults, id)
	return entry.result, true
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
	go func() {
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
	}()

	qd.mu.Lock()
	defer qd.mu.Unlock()
	defer close(done) // Stop wakeup goroutine on return (runs before Unlock per LIFO)

	for {
		// Check if result exists
		if entry, found := qd.queryResults[id]; found {
			// Check client isolation
			if clientID == "" || entry.clientID == clientID {
				delete(qd.queryResults, id)
				return entry.result, nil
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
// expired query results (60s TTL).
// Returns a stop function that terminates the goroutine.
// Called once during QueryDispatcher initialization; stop func stored in stopCleanup.
func (qd *QueryDispatcher) startResultCleanup() func() {
	stop := make(chan struct{})
	go func() {
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
	}()
	return func() { close(stop) }
}

// cleanExpiredResults removes query results older than queryResultTTL.
func (qd *QueryDispatcher) cleanExpiredResults() {
	qd.mu.Lock()
	defer qd.mu.Unlock()

	now := time.Now()
	for id, entry := range qd.queryResults {
		if now.Sub(entry.createdAt) > queryResultTTL {
			delete(qd.queryResults, id)
		}
	}
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

	qd.completedResults[correlationID] = &queries.CommandResult{
		CorrelationID: correlationID,
		Status:        "pending",
		CreatedAt:     time.Now(),
	}
}

// CompleteCommand updates a command's status to "complete" with result.
// Called when extension posts result back.
func (qd *QueryDispatcher) CompleteCommand(correlationID string, result json.RawMessage, err string) {
	if correlationID == "" {
		return
	}

	qd.resultsMu.Lock()
	defer qd.resultsMu.Unlock()

	cmd, exists := qd.completedResults[correlationID]
	if !exists {
		// Command may have expired and been moved to failedCommands
		return
	}

	cmd.Status = "complete"
	cmd.Result = result
	cmd.Error = err
	cmd.CompletedAt = time.Now()
}

// ExpireCommand marks a command as "expired" and moves it to failedCommands.
// Called by cleanup goroutine when command times out without result.
func (qd *QueryDispatcher) ExpireCommand(correlationID string) {
	if correlationID == "" {
		return
	}

	qd.resultsMu.Lock()
	defer qd.resultsMu.Unlock()

	cmd, exists := qd.completedResults[correlationID]
	if !exists {
		return
	}

	// Update status
	cmd.Status = "expired"
	cmd.Error = "Command expired before extension could execute it"

	// Move to failedCommands ring buffer
	qd.failedCommands = append(qd.failedCommands, cmd)
	if len(qd.failedCommands) > 100 {
		qd.failedCommands = qd.failedCommands[1:]
	}

	// Remove from active tracking
	delete(qd.completedResults, correlationID)
}

// GetCommandResult retrieves command status by correlation ID.
// Returns (CommandResult, found). Used by toolObserveCommandResult.
func (qd *QueryDispatcher) GetCommandResult(correlationID string) (*queries.CommandResult, bool) {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	// Check active commands
	if cmd, exists := qd.completedResults[correlationID]; exists {
		return cmd, true
	}

	// Check failed/expired commands
	for _, cmd := range qd.failedCommands {
		if cmd.CorrelationID == correlationID {
			return cmd, true
		}
	}

	return nil, false
}

// GetPendingCommands returns all commands with status "pending".
// Used by toolObservePendingCommands.
func (qd *QueryDispatcher) GetPendingCommands() []*queries.CommandResult {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*queries.CommandResult, 0)
	for _, cmd := range qd.completedResults {
		if cmd.Status == "pending" {
			result = append(result, cmd)
		}
	}
	return result
}

// GetCompletedCommands returns all commands with status "complete".
// Used by toolObservePendingCommands.
func (qd *QueryDispatcher) GetCompletedCommands() []*queries.CommandResult {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*queries.CommandResult, 0)
	for _, cmd := range qd.completedResults {
		if cmd.Status == "complete" {
			result = append(result, cmd)
		}
	}
	return result
}

// GetFailedCommands returns recent failed/expired commands.
// Used by toolObserveFailedCommands.
func (qd *QueryDispatcher) GetFailedCommands() []*queries.CommandResult {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	// Return copy to avoid concurrent modification
	result := make([]*queries.CommandResult, len(qd.failedCommands))
	copy(result, qd.failedCommands)
	return result
}
