// Purpose: Reconciles extension in-progress heartbeats with command lifecycle state.
// Why: Keeps desync detection and heartbeat normalization isolated from main /sync handler flow.
// Docs: docs/features/feature/query-service/index.md

package capture

import (
	"strings"

	"github.com/dev-console/dev-console/internal/queries"
	"github.com/dev-console/dev-console/internal/util"
)

// normalizeInProgressList sanitizes extension heartbeat command state for reconciliation.
//
// Invariants:
// - Returns nil only when caller supplied nil (distinguishes "unsupported" vs "empty list").
// - Output is capped to maxInProgress to bound memory and CPU cost per heartbeat.
//
// Failure semantics:
// - Malformed/empty entries are dropped rather than failing the whole sync request.
func normalizeInProgressList(in []SyncInProgress) []SyncInProgress {
	if in == nil {
		return nil
	}
	if len(in) == 0 {
		return []SyncInProgress{}
	}
	const maxInProgress = 100
	limit := len(in)
	if limit > maxInProgress {
		limit = maxInProgress
	}
	out := make([]SyncInProgress, 0, limit)
	for i := 0; i < limit; i++ {
		entry := in[i]
		entry.ID = strings.TrimSpace(entry.ID)
		entry.CorrelationID = strings.TrimSpace(entry.CorrelationID)
		entry.Type = strings.TrimSpace(entry.Type)
		entry.Status = strings.TrimSpace(strings.ToLower(entry.Status))
		if entry.Status == "" {
			entry.Status = "running"
		}
		if entry.ProgressPct != nil {
			p := *entry.ProgressPct
			if p < 0 {
				p = 0
			}
			if p > 100 {
				p = 100
			}
			entry.ProgressPct = &p
		}
		if entry.ID == "" && entry.CorrelationID == "" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// commandHasStarted returns true once trace evidence indicates extension execution began.
//
// Failure semantics:
// - Missing trace context returns false, which delays desync failure until stronger evidence exists.
func commandHasStarted(cmd *queries.CommandResult) bool {
	if cmd == nil {
		return false
	}
	for _, evt := range cmd.TraceEvents {
		if evt.Stage == "started" || evt.Stage == "resolved" || evt.Stage == "errored" || evt.Stage == "timed_out" {
			return true
		}
	}
	return strings.Contains(cmd.TraceTimeline, "started")
}

// reconcileInProgressCommandState detects commands lost after extension acknowledgement.
//
// Invariants:
// - A command is failed only after two consecutive missed heartbeats once "started" is observed.
// - missingInProgressByCorr map is pruned for no-longer-pending commands each cycle.
//
// Failure semantics:
// - nil inProgress means "client does not support heartbeat reporting" and reconciliation is skipped.
// - Desync failures emit terminal command errors so callers do not wait for full timeout.
func (c *Capture) reconcileInProgressCommandState(inProgress []SyncInProgress) {
	if inProgress == nil {
		// Older extension/client that doesn't report in_progress yet.
		return
	}

	active := make(map[string]struct{}, len(inProgress))
	for _, entry := range inProgress {
		if entry.CorrelationID != "" {
			active[entry.CorrelationID] = struct{}{}
		}
	}

	pending := c.GetPendingCommands()
	pendingCorr := make(map[string]struct{}, len(pending))
	toFail := make([]string, 0)
	toFailIDs := make([]string, 0)

	func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.extensionState.missingInProgressByCorr == nil {
			c.extensionState.missingInProgressByCorr = make(map[string]int)
		}
		for _, cmd := range pending {
			if cmd == nil || cmd.CorrelationID == "" {
				continue
			}
			corr := cmd.CorrelationID
			pendingCorr[corr] = struct{}{}

			if _, ok := active[corr]; ok {
				delete(c.extensionState.missingInProgressByCorr, corr)
				continue
			}
			if !commandHasStarted(cmd) {
				continue
			}
			c.extensionState.missingInProgressByCorr[corr]++
			if c.extensionState.missingInProgressByCorr[corr] >= 2 {
				toFail = append(toFail, corr)
				toFailIDs = append(toFailIDs, cmd.QueryID)
				delete(c.extensionState.missingInProgressByCorr, corr)
			}
		}

		for corr := range c.extensionState.missingInProgressByCorr {
			if _, stillPending := pendingCorr[corr]; !stillPending {
				delete(c.extensionState.missingInProgressByCorr, corr)
			}
		}
	}()

	for i, corr := range toFail {
		queryID := ""
		if i < len(toFailIDs) {
			queryID = toFailIDs[i]
		}
		c.ApplyCommandResult(
			corr,
			"error",
			nil,
			"extension_lost_command: command acknowledged by extension but missing from in_progress heartbeats",
		)
		util.SafeGo(func() {
			c.emitLifecycleEvent("command_state_desync", map[string]any{
				"correlation_id": corr,
				"query_id":       queryID,
				"reason":         "missing_in_progress_heartbeat",
			})
		})
	}
}
