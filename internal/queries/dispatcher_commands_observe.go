// Purpose: Exposes command lifecycle snapshots for observe/debug tooling.
// Why: Separates read-only command views from mutation and wait-path logic.
// Docs: docs/features/feature/query-service/index.md

package queries

// GetPendingCommands returns all commands with status "pending".
// Used by toolObservePendingCommands.
func (qd *QueryDispatcher) GetPendingCommands() []*CommandResult {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*CommandResult, 0)
	for _, cmd := range qd.completedResults {
		if cmd.Status == "pending" {
			result = append(result, copyCommandResultWithTrace(cmd))
		}
	}
	return result
}

// GetCompletedCommands returns all commands with status "complete".
// Used by toolObservePendingCommands.
func (qd *QueryDispatcher) GetCompletedCommands() []*CommandResult {
	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*CommandResult, 0)
	for _, cmd := range qd.completedResults {
		if cmd.Status == "complete" {
			result = append(result, copyCommandResultWithTrace(cmd))
		}
	}
	return result
}

// GetFailedCommands returns recent failed/expired commands.
// Used by toolObserveFailedCommands.
// Eagerly cleans expired commands so callers see freshly expired entries.
func (qd *QueryDispatcher) GetFailedCommands() []*CommandResult {
	qd.cleanExpiredCommands()

	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	result := make([]*CommandResult, 0, len(qd.failedCommands)+len(qd.completedResults))
	seen := make(map[string]struct{}, len(qd.failedCommands))

	for _, cmd := range qd.failedCommands {
		if cmd == nil {
			continue
		}
		cp := copyCommandResultWithTrace(cmd)
		result = append(result, cp)
		if cp.CorrelationID != "" {
			seen[cp.CorrelationID] = struct{}{}
		}
	}
	for _, cmd := range qd.completedResults {
		if cmd == nil || !IsFailedCommandStatus(cmd.Status) {
			continue
		}
		if _, exists := seen[cmd.CorrelationID]; exists {
			continue
		}
		result = append(result, copyCommandResultWithTrace(cmd))
	}
	return result
}
