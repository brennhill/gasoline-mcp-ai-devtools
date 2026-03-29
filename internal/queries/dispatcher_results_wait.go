// Purpose: Blocks until a query result arrives or timeout expires, with client-isolated views.
// Why: Separates synchronous wait logic from result storage and cleanup.
package queries

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

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
