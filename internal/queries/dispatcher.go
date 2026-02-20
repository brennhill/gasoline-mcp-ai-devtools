// dispatcher.go — QueryDispatcher struct, factory, and snapshot.
// Manages pending query queues, result storage, and async command tracking.
// Owns its own sync.Mutex and sync.RWMutex — independent of Capture.mu.
package queries

import (
	"encoding/json"
	"sync"
	"time"
)

// PendingQueryEntry tracks a pending query with timeout.
type PendingQueryEntry struct {
	Query    PendingQueryResponse
	Expires  time.Time
	ClientID string // owning client for multi-client isolation
}

// QueryResultEntry stores a query result with client ownership.
type QueryResultEntry struct {
	Result    json.RawMessage
	ClientID  string // owning client for multi-client isolation
	CreatedAt time.Time
}

// QueryDispatcher manages pending query queues, result storage, and async command tracking.
// Owns two locks:
//   - mu (sync.Mutex): protects pendingQueries, queryResults, queryCond, queryIDCounter, queryTimeout
//   - resultsMu (sync.RWMutex): protects completedResults, failedCommands
//
// Lock ordering: mu released BEFORE resultsMu acquired (never reverse).
type QueryDispatcher struct {
	mu             sync.Mutex
	pendingQueries []PendingQueryEntry
	queryResults   map[string]QueryResultEntry
	queryCond      *sync.Cond
	queryIDCounter int
	queryTimeout   time.Duration

	resultsMu        sync.RWMutex
	completedResults map[string]*CommandResult
	failedCommands   []*CommandResult
	commandNotify    chan struct{} // closed on CompleteCommand, then recreated
	queryNotify      chan struct{} // signaled when new pending queries are added

	stopCleanup func()
}

// NewQueryDispatcher creates a QueryDispatcher with initialized state.
func NewQueryDispatcher() *QueryDispatcher {
	qd := &QueryDispatcher{
		pendingQueries:   make([]PendingQueryEntry, 0),
		queryResults:     make(map[string]QueryResultEntry),
		queryTimeout:     DefaultQueryTimeout,
		completedResults: make(map[string]*CommandResult),
		failedCommands:   make([]*CommandResult, 0, 100),
		commandNotify:    make(chan struct{}),
		queryNotify:      make(chan struct{}, 1),
	}
	qd.queryCond = sync.NewCond(&qd.mu)
	qd.stopCleanup = qd.startResultCleanup()
	return qd
}

// Close stops background goroutines. Safe to call multiple times.
func (qd *QueryDispatcher) Close() {
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
