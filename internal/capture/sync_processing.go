// Purpose: Applies sync payload side effects (results/log ingestion) to capture state.
// Why: Keeps mutable ingestion logic separate from request routing and response assembly.
// Docs: docs/features/feature/query-service/index.md

package capture

import "time"

// processSyncCommandResults applies extension result/status updates.
//
// Invariants:
// - Correlated commands use status from ApplyCommandResult as source of truth.
//
// Failure semantics:
// - Unknown query/command IDs are ignored to keep sync idempotent.
// - Query results can be stored even if lifecycle completion arrives separately.
func (c *Capture) processSyncCommandResults(results []SyncCommandResult, clientID string) {
	for _, result := range results {
		if result.ID != "" {
			if result.CorrelationID != "" {
				// Correlated async commands carry explicit lifecycle status below.
				// Do not force "complete" from query-id bookkeeping.
				c.SetQueryResultWithClientNoCommandComplete(result.ID, result.Result, clientID)
			} else {
				c.SetQueryResultWithClient(result.ID, result.Result, clientID)
			}
		}
		if result.CorrelationID != "" {
			c.ApplyCommandResult(result.CorrelationID, result.Status, result.Result, result.Error)
		}
	}
}

// updateSyncLogs ingests extension logs and metadata under c.mu.
//
// Invariants:
// - Extension log buffer uses amortized compaction (at 1.5x capacity) to avoid per-entry copying.
// - Redaction is applied before logs enter persistent in-memory buffers.
//
// Failure semantics:
// - Invalid/missing timestamps are normalized to server receive time.
func (c *Capture) updateSyncLogs(req SyncRequest, now time.Time, pilotEnabled bool, queryCount int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logPollingActivity(PollingLogEntry{
		Timestamp:    now,
		Endpoint:     "sync",
		Method:       "POST",
		ExtSessionID: req.ExtSessionID,
		PilotEnabled: &pilotEnabled,
		QueryCount:   queryCount,
	})

	for _, log := range req.ExtensionLogs {
		if log.Timestamp.IsZero() {
			log.Timestamp = now
		}
		log = c.redactExtensionLog(log)
		c.extensionLogs.logs = append(c.extensionLogs.logs, log)

		evictionThreshold := MaxExtensionLogs + MaxExtensionLogs/2
		if len(c.extensionLogs.logs) > evictionThreshold {
			kept := make([]ExtensionLog, MaxExtensionLogs)
			copy(kept, c.extensionLogs.logs[len(c.extensionLogs.logs)-MaxExtensionLogs:])
			c.extensionLogs.logs = kept
		}
	}

	if req.ExtensionVersion != "" {
		c.extensionState.extensionVersion = req.ExtensionVersion
	}
}
