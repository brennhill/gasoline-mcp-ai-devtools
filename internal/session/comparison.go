// comparison.go — Main comparison logic.
// Compare function and related comparison orchestration.
package session

import (
	"fmt"
)

// Compare diffs two snapshots. Use "current" as b to compare against live state.
func (sm *SessionManager) Compare(a, b string) (*SessionDiffResult, error) {
	sm.mu.RLock()
	snapA, existsA := sm.snaps[a]
	sm.mu.RUnlock()

	if !existsA {
		return nil, fmt.Errorf("snapshot %q not found", a)
	}

	var snapB *NamedSnapshot
	if b == reservedSnapshotName {
		// Compare against current live state
		snapB = sm.captureCurrentState("current", snapA.URLFilter)
	} else {
		sm.mu.RLock()
		found, exists := sm.snaps[b]
		sm.mu.RUnlock()
		if !exists {
			return nil, fmt.Errorf("snapshot %q not found", b)
		}
		snapB = found
	}

	result := &SessionDiffResult{
		A: a,
		B: b,
	}

	// Compute error diff
	result.Errors = sm.diffErrors(snapA, snapB)

	// Compute network diff
	result.Network = sm.diffNetwork(snapA, snapB)

	// Compute performance diff
	result.Performance = sm.diffPerformance(snapA, snapB)

	// Compute summary and verdict
	result.Summary = sm.computeSummary(result)

	return result, nil
}
