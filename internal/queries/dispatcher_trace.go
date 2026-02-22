package queries

import (
	"sort"
	"strings"
	"time"
)

const (
	traceStageQueued   = "queued"
	traceStageSent     = "sent"
	traceStageStarted  = "started"
	traceStageResolved = "resolved"
	traceStageTimedOut = "timed_out"
	traceStageErrored  = "errored"
)

func deriveTraceID(explicit string, correlationID string, queryID string) string {
	if explicit != "" {
		return explicit
	}
	if correlationID != "" {
		return correlationID
	}
	if queryID != "" {
		return "trace_" + queryID
	}
	return ""
}

func buildTraceTimeline(events []CommandTraceEvent) string {
	if len(events) == 0 {
		return ""
	}
	stages := make([]string, 0, len(events))
	for _, evt := range events {
		if evt.Stage == "" {
			continue
		}
		stages = append(stages, evt.Stage)
	}
	return strings.Join(stages, " -> ")
}

func traceStageFromStatus(status string) string {
	switch NormalizeCommandStatus(status) {
	case "pending":
		return traceStageStarted
	case "complete":
		return traceStageResolved
	case "error", "cancelled":
		return traceStageErrored
	case "timeout", "expired":
		return traceStageTimedOut
	default:
		return ""
	}
}

func (qd *QueryDispatcher) ensureTraceContextLocked(cmd *CommandResult, correlationID string, queryID string, traceID string, now time.Time) {
	if cmd == nil {
		return
	}
	if cmd.CorrelationID == "" {
		cmd.CorrelationID = correlationID
	}
	if cmd.TraceID == "" {
		cmd.TraceID = deriveTraceID(traceID, correlationID, queryID)
	}
	if cmd.QueryID == "" && queryID != "" {
		cmd.QueryID = queryID
	}
	if cmd.UpdatedAt.IsZero() {
		cmd.UpdatedAt = now
	}
}

func (qd *QueryDispatcher) hasTraceStageLocked(cmd *CommandResult, stage string) bool {
	if cmd == nil || stage == "" {
		return false
	}
	for _, evt := range cmd.TraceEvents {
		if evt.Stage == stage {
			return true
		}
	}
	return false
}

func (qd *QueryDispatcher) appendTraceEventLocked(cmd *CommandResult, stage string, source string, status string, message string, at time.Time) {
	if cmd == nil || stage == "" {
		return
	}
	if qd.hasTraceStageLocked(cmd, stage) {
		if at.After(cmd.UpdatedAt) {
			cmd.UpdatedAt = at
		}
		return
	}

	cmd.TraceEvents = append(cmd.TraceEvents, CommandTraceEvent{
		Stage:   stage,
		At:      at,
		Source:  source,
		Status:  status,
		Message: message,
	})
	cmd.TraceTimeline = buildTraceTimeline(cmd.TraceEvents)
	cmd.UpdatedAt = at
}

func (qd *QueryDispatcher) recordTraceEvent(correlationID string, stage string, source string, status string, message string, at time.Time) {
	if correlationID == "" || stage == "" {
		return
	}

	qd.resultsMu.Lock()
	defer qd.resultsMu.Unlock()

	if cmd, exists := qd.completedResults[correlationID]; exists {
		qd.ensureTraceContextLocked(cmd, correlationID, "", "", at)
		qd.appendTraceEventLocked(cmd, stage, source, status, message, at)
		return
	}
	for _, cmd := range qd.failedCommands {
		if cmd == nil || cmd.CorrelationID != correlationID {
			continue
		}
		qd.ensureTraceContextLocked(cmd, correlationID, "", "", at)
		qd.appendTraceEventLocked(cmd, stage, source, status, message, at)
		return
	}
}

func copyCommandResultWithTrace(src *CommandResult) *CommandResult {
	if src == nil {
		return nil
	}
	cp := *src
	if len(src.TraceEvents) > 0 {
		cp.TraceEvents = make([]CommandTraceEvent, len(src.TraceEvents))
		copy(cp.TraceEvents, src.TraceEvents)
	}
	return &cp
}

// GetRecentCommandTraces returns the latest command traces across active and failed commands.
// Sorted by UpdatedAt descending and bounded by limit (if > 0).
func (qd *QueryDispatcher) GetRecentCommandTraces(limit int) []*CommandResult {
	qd.cleanExpiredCommands()

	qd.resultsMu.RLock()
	defer qd.resultsMu.RUnlock()

	combined := make([]*CommandResult, 0, len(qd.completedResults)+len(qd.failedCommands))
	seen := make(map[string]struct{}, len(qd.completedResults)+len(qd.failedCommands))
	add := func(cmd *CommandResult) {
		if cmd == nil {
			return
		}
		key := cmd.CorrelationID
		if key == "" {
			key = cmd.TraceID
		}
		if key != "" {
			if _, exists := seen[key]; exists {
				return
			}
			seen[key] = struct{}{}
		}
		combined = append(combined, copyCommandResultWithTrace(cmd))
	}

	for _, cmd := range qd.failedCommands {
		add(cmd)
	}
	for _, cmd := range qd.completedResults {
		add(cmd)
	}

	sort.Slice(combined, func(i, j int) bool {
		left := combined[i].UpdatedAt
		right := combined[j].UpdatedAt
		if left.IsZero() {
			left = combined[i].CreatedAt
		}
		if right.IsZero() {
			right = combined[j].CreatedAt
		}
		return left.After(right)
	})

	if limit > 0 && len(combined) > limit {
		combined = combined[:limit]
	}
	return combined
}
