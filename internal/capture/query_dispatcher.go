// Purpose: Owns query_dispatcher.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// query_dispatcher.go — Query lifecycle, result storage, and async command tracking.
// Extracted from the Capture god object. Owns its own sync.Mutex (for pending queries
// and condition variable) and sync.RWMutex (for async command results).
// Zero cross-cutting dependencies.
package capture

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// pendingQueryEntry tracks a pending query with timeout
type pendingQueryEntry struct {
	query    queries.PendingQueryResponse
	expires  time.Time
	clientID string // owning client for multi-client isolation
}

// queryResultEntry stores a query result with client ownership
type queryResultEntry struct {
	result    json.RawMessage
	clientID  string // owning client for multi-client isolation
	createdAt time.Time
}

// QueryDispatcher manages pending query queues, result storage, and async command tracking.
// Owns two locks:
//   - mu (sync.Mutex): protects pendingQueries, queryResults, queryCond, queryIDCounter, queryTimeout
//   - resultsMu (sync.RWMutex): protects completedResults, failedCommands
//
// Lock ordering: mu released BEFORE resultsMu acquired (never reverse).
type QueryDispatcher struct {
	mu             sync.Mutex
	pendingQueries []pendingQueryEntry
	queryResults   map[string]queryResultEntry
	queryCond      *sync.Cond
	queryIDCounter int
	queryTimeout   time.Duration

	resultsMu        sync.RWMutex
	completedResults map[string]*queries.CommandResult
	failedCommands   []*queries.CommandResult
	commandNotify    chan struct{} // closed on CompleteCommand, then recreated
	queryNotify      chan struct{} // signaled when new pending queries are added

	stopCleanup     func()
	stopBroadcaster func()
}

// NewQueryDispatcher creates a QueryDispatcher with initialized state.
func NewQueryDispatcher() *QueryDispatcher {
	qd := &QueryDispatcher{
		pendingQueries:   make([]pendingQueryEntry, 0),
		queryResults:     make(map[string]queryResultEntry),
		queryTimeout:     queries.DefaultQueryTimeout,
		completedResults: make(map[string]*queries.CommandResult),
		failedCommands:   make([]*queries.CommandResult, 0, 100),
		commandNotify:    make(chan struct{}),
		queryNotify:      make(chan struct{}, 1),
	}
	qd.queryCond = sync.NewCond(&qd.mu)
	qd.stopCleanup = qd.startResultCleanup()
	qd.stopBroadcaster = qd.startCondBroadcaster()
	return qd
}

// Close stops background goroutines. Safe to call multiple times.
func (qd *QueryDispatcher) Close() {
	if qd.stopBroadcaster != nil {
		qd.stopBroadcaster()
		qd.stopBroadcaster = nil
	}
	if qd.stopCleanup != nil {
		qd.stopCleanup()
		qd.stopCleanup = nil
	}
}

// QuerySnapshot contains a point-in-time view of query state for health reporting.
type QuerySnapshot struct {
	PendingQueryCount int
	QueryResultCount  int
	QueryTimeout      time.Duration
}

// GetSnapshot returns a thread-safe snapshot of query state.
func (qd *QueryDispatcher) GetSnapshot() QuerySnapshot {
	qd.mu.Lock()
	defer qd.mu.Unlock()
	return QuerySnapshot{
		PendingQueryCount: len(qd.pendingQueries),
		QueryResultCount:  len(qd.queryResults),
		QueryTimeout:      qd.queryTimeout,
	}
}

// ============================================================================
// Capture delegation methods — preserve external API.
// ============================================================================

// CreatePendingQuery delegates to QueryDispatcher.
func (c *Capture) CreatePendingQuery(query queries.PendingQuery) string {
	return c.qd.CreatePendingQuery(query)
}

// CreatePendingQueryWithClient delegates to QueryDispatcher.
func (c *Capture) CreatePendingQueryWithClient(query queries.PendingQuery, clientID string) string {
	return c.qd.CreatePendingQueryWithClient(query, clientID)
}

// CreatePendingQueryWithTimeout delegates to QueryDispatcher.
func (c *Capture) CreatePendingQueryWithTimeout(query queries.PendingQuery, timeout time.Duration, clientID string) string {
	return c.qd.CreatePendingQueryWithTimeout(query, timeout, clientID)
}

// GetPendingQueries delegates to QueryDispatcher.
func (c *Capture) GetPendingQueries() []queries.PendingQueryResponse {
	return c.qd.GetPendingQueries()
}

// GetPendingQueriesForClient delegates to QueryDispatcher.
func (c *Capture) GetPendingQueriesForClient(clientID string) []queries.PendingQueryResponse {
	return c.qd.GetPendingQueriesForClient(clientID)
}

// WaitForPendingQueries delegates to QueryDispatcher.
func (c *Capture) WaitForPendingQueries(timeout time.Duration) {
	c.qd.WaitForPendingQueries(timeout)
}

// AcknowledgePendingQuery delegates to QueryDispatcher.
func (c *Capture) AcknowledgePendingQuery(queryID string) {
	c.qd.AcknowledgePendingQuery(queryID)
}

// GetPendingQueriesDisconnectAware returns pending queries with disconnect detection.
// If the extension has not synced within extensionDisconnectThreshold (10s) and has
// synced at least once, all pending queries are expired with "extension_disconnected".
// This prevents queries from hanging indefinitely when the extension crashes or disconnects.
func (c *Capture) GetPendingQueriesDisconnectAware() []queries.PendingQueryResponse {
	c.mu.RLock()
	neverSynced := c.ext.lastSyncSeen.IsZero()
	disconnected := !neverSynced && time.Since(c.ext.lastSyncSeen) >= extensionDisconnectThreshold
	c.mu.RUnlock()

	// If extension was previously connected but is now stale, expire all pending queries
	if disconnected {
		c.qd.ExpireAllPendingQueries("extension_disconnected")
		return nil
	}

	return c.qd.GetPendingQueries()
}

// SetQueryResult delegates to QueryDispatcher.
func (c *Capture) SetQueryResult(id string, result json.RawMessage) {
	c.qd.SetQueryResult(id, result)
}

// SetQueryResultOnly delegates to QueryDispatcher.
func (c *Capture) SetQueryResultOnly(id string, result json.RawMessage, clientID string) string {
	return c.qd.SetQueryResultOnly(id, result, clientID)
}

// SetQueryResultWithClient delegates to QueryDispatcher.
func (c *Capture) SetQueryResultWithClient(id string, result json.RawMessage, clientID string) {
	c.qd.SetQueryResultWithClient(id, result, clientID)
}

// GetQueryResult delegates to QueryDispatcher.
func (c *Capture) GetQueryResult(id string) (json.RawMessage, bool) {
	return c.qd.GetQueryResult(id)
}

// GetQueryResultForClient delegates to QueryDispatcher.
func (c *Capture) GetQueryResultForClient(id string, clientID string) (json.RawMessage, bool) {
	return c.qd.GetQueryResultForClient(id, clientID)
}

// WaitForResult delegates to QueryDispatcher.
func (c *Capture) WaitForResult(id string, timeout time.Duration) (json.RawMessage, error) {
	return c.qd.WaitForResult(id, timeout)
}

// WaitForResultWithClient delegates to QueryDispatcher.
func (c *Capture) WaitForResultWithClient(id string, timeout time.Duration, clientID string) (json.RawMessage, error) {
	return c.qd.WaitForResultWithClient(id, timeout, clientID)
}

// SetQueryTimeout delegates to QueryDispatcher.
func (c *Capture) SetQueryTimeout(timeout time.Duration) {
	c.qd.SetQueryTimeout(timeout)
}

// GetQueryTimeout delegates to QueryDispatcher.
func (c *Capture) GetQueryTimeout() time.Duration {
	return c.qd.GetQueryTimeout()
}

// RegisterCommand delegates to QueryDispatcher.
func (c *Capture) RegisterCommand(correlationID string, queryID string, timeout time.Duration) {
	c.qd.RegisterCommand(correlationID, queryID, timeout)
}

// RegisterCommandForClient delegates to QueryDispatcher.
func (c *Capture) RegisterCommandForClient(correlationID string, queryID string, timeout time.Duration, clientID string) {
	c.qd.RegisterCommandForClient(correlationID, queryID, timeout, clientID)
}

// CompleteCommand delegates to QueryDispatcher.
func (c *Capture) CompleteCommand(correlationID string, result json.RawMessage, err string) {
	c.qd.CompleteCommand(correlationID, result, err)
}

// CompleteCommandWithStatus delegates to QueryDispatcher.
func (c *Capture) CompleteCommandWithStatus(correlationID string, result json.RawMessage, status string, err string) {
	c.qd.CompleteCommandWithStatus(correlationID, result, status, err)
}

// ExpireCommand delegates to QueryDispatcher.
func (c *Capture) ExpireCommand(correlationID string) {
	c.qd.ExpireCommand(correlationID)
}

// WaitForCommand delegates to QueryDispatcher.
func (c *Capture) WaitForCommand(correlationID string, timeout time.Duration) (*queries.CommandResult, bool) {
	return c.qd.WaitForCommand(correlationID, timeout)
}

// WaitForCommandForClient delegates to QueryDispatcher.
func (c *Capture) WaitForCommandForClient(correlationID string, timeout time.Duration, clientID string) (*queries.CommandResult, bool) {
	return c.qd.WaitForCommandForClient(correlationID, timeout, clientID)
}

// GetCommandResult delegates to QueryDispatcher.
func (c *Capture) GetCommandResult(correlationID string) (*queries.CommandResult, bool) {
	return c.qd.GetCommandResult(correlationID)
}

// GetCommandResultForClient delegates to QueryDispatcher.
func (c *Capture) GetCommandResultForClient(correlationID string, clientID string) (*queries.CommandResult, bool) {
	return c.qd.GetCommandResultForClient(correlationID, clientID)
}

// GetPendingCommands delegates to QueryDispatcher.
func (c *Capture) GetPendingCommands() []*queries.CommandResult {
	return c.qd.GetPendingCommands()
}

// GetCompletedCommands delegates to QueryDispatcher.
func (c *Capture) GetCompletedCommands() []*queries.CommandResult {
	return c.qd.GetCompletedCommands()
}

// GetFailedCommands delegates to QueryDispatcher.
func (c *Capture) GetFailedCommands() []*queries.CommandResult {
	return c.qd.GetFailedCommands()
}
