// ai_checkpoint_compute.go — Diff computation and summary building for checkpoints.
package ai

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/dev-console/dev-console/internal/capture"
)

// ============================================
// Diff computation
// ============================================

func (cm *CheckpointManager) computeConsoleDiff(cp *Checkpoint, severity string) *ConsoleDiff {
	// Get snapshot with proper locking
	snapshot := cm.server.GetLogSnapshot()
	currentTotal := snapshot.TotalAdded
	newCount := int(currentTotal - cp.LogTotal)
	if newCount <= 0 {
		return &ConsoleDiff{}
	}
	available := len(snapshot.Entries)
	toRead := newCount
	if toRead > available {
		toRead = available
	}
	// Snapshot already copied the entries, slice them as needed
	newEntries := snapshot.Entries[available-toRead:]

	// Separate into errors and warnings, deduplicate by fingerprint
	type fingerprintEntry struct {
		message string
		source  string
		count   int
	}
	errorMap := make(map[string]*fingerprintEntry)
	warningMap := make(map[string]*fingerprintEntry)
	var errorOrder, warningOrder []string

	totalNew := 0
	for _, entry := range newEntries {
		level, _ := entry["level"].(string)
		msg, _ := entry["msg"].(string)
		if msg == "" {
			msg, _ = entry["message"].(string)
		}
		source, _ := entry["source"].(string)

		if level == "error" {
			totalNew++
			fp := FingerprintMessage(msg)
			if existing, ok := errorMap[fp]; ok {
				existing.count++
			} else {
				truncMsg := truncateMessage(msg)
				errorMap[fp] = &fingerprintEntry{message: truncMsg, source: source, count: 1}
				errorOrder = append(errorOrder, fp)
			}
		} else if level == "warn" || level == "warning" {
			totalNew++
			if severity == "errors_only" {
				continue
			}
			fp := FingerprintMessage(msg)
			if existing, ok := warningMap[fp]; ok {
				existing.count++
			} else {
				truncMsg := truncateMessage(msg)
				warningMap[fp] = &fingerprintEntry{message: truncMsg, source: source, count: 1}
				warningOrder = append(warningOrder, fp)
			}
		} else {
			totalNew++
		}
	}

	diff := &ConsoleDiff{TotalNew: totalNew}

	// Build error entries (capped at max)
	for i, fp := range errorOrder {
		if i >= maxDiffEntriesPerCat {
			break
		}
		e := errorMap[fp]
		diff.Errors = append(diff.Errors, ConsoleEntry{
			Message: e.message,
			Source:  e.source,
			Count:   e.count,
		})
	}

	// Build warning entries (capped at max)
	for i, fp := range warningOrder {
		if i >= maxDiffEntriesPerCat {
			break
		}
		w := warningMap[fp]
		diff.Warnings = append(diff.Warnings, ConsoleEntry{
			Message: w.message,
			Source:  w.source,
			Count:   w.count,
		})
	}

	return diff
}

func (cm *CheckpointManager) computeNetworkDiff(cp *Checkpoint) *NetworkDiff {
	currentTotal := cm.capture.GetNetworkTotalAdded()
	newCount := int(currentTotal - cp.NetworkTotal)
	if newCount <= 0 {
		return &NetworkDiff{}
	}

	// Get all network bodies (thread-safe)
	allBodies := cm.capture.GetNetworkBodies()
	available := len(allBodies)
	toRead := newCount
	if toRead > available {
		toRead = available
	}
	// Slice the already-copied bodies to get the most recent newCount entries
	newBodies := allBodies[available-toRead:]

	diff := &NetworkDiff{TotalNew: len(newBodies)}

	// Track endpoints seen in new entries
	for _, body := range newBodies {
		path := capture.ExtractURLPath(body.URL)

		// Check for failures (4xx/5xx where previously success)
		if body.Status >= 400 {
			if prev, known := cp.KnownEndpoints[path]; known && prev.Status < 400 {
				diff.Failures = append(diff.Failures, NetworkFailure{
					Path:           path,
					Status:         body.Status,
					PreviousStatus: prev.Status,
				})
			} else if !known {
				// New endpoint that immediately fails — count as new endpoint
				if !containsString(diff.NewEndpoints, path) {
					diff.NewEndpoints = append(diff.NewEndpoints, path)
				}
			}
		} else {
			// Check for new endpoints
			if _, known := cp.KnownEndpoints[path]; !known {
				if !containsString(diff.NewEndpoints, path) {
					diff.NewEndpoints = append(diff.NewEndpoints, path)
				}
			}

			// Check for degraded latency
			if body.Duration > 0 {
				if prev, known := cp.KnownEndpoints[path]; known && prev.Duration > 0 {
					if body.Duration > prev.Duration*degradedLatencyFactor {
						diff.Degraded = append(diff.Degraded, NetworkDegraded{
							Path:     path,
							Duration: body.Duration,
							Baseline: prev.Duration,
						})
					}
				}
			}
		}
	}

	// Cap entries
	if len(diff.Failures) > maxDiffEntriesPerCat {
		diff.Failures = diff.Failures[:maxDiffEntriesPerCat]
	}
	if len(diff.NewEndpoints) > maxDiffEntriesPerCat {
		diff.NewEndpoints = diff.NewEndpoints[:maxDiffEntriesPerCat]
	}
	if len(diff.Degraded) > maxDiffEntriesPerCat {
		diff.Degraded = diff.Degraded[:maxDiffEntriesPerCat]
	}

	return diff
}

func (cm *CheckpointManager) computeWebSocketDiff(cp *Checkpoint, severity string) *WebSocketDiff {
	currentTotal := cm.capture.GetWebSocketTotalAdded()
	newCount := int(currentTotal - cp.WSTotal)
	if newCount <= 0 {
		return &WebSocketDiff{}
	}

	// Get all WebSocket events (thread-safe)
	allEvents := cm.capture.GetAllWebSocketEvents()
	available := len(allEvents)
	toRead := newCount
	if toRead > available {
		toRead = available
	}
	// Slice the already-copied events to get the most recent newCount entries
	newEvents := allEvents[available-toRead:]

	diff := &WebSocketDiff{TotalNew: len(newEvents)}

	for i := range newEvents {
		switch newEvents[i].Event {
		case "close":
			if severity != "errors_only" {
				diff.Disconnections = append(diff.Disconnections, WSDisco{
					URL:         newEvents[i].URL,
					CloseCode:   newEvents[i].CloseCode,
					CloseReason: newEvents[i].CloseReason,
				})
			}
		case "open":
			diff.Connections = append(diff.Connections, WSConn{
				URL: newEvents[i].URL,
				ID:  newEvents[i].ID,
			})
		case "error":
			diff.Errors = append(diff.Errors, WSError{
				URL:     newEvents[i].URL,
				Message: newEvents[i].Data,
			})
		}
	}

	// Cap entries
	if len(diff.Disconnections) > maxDiffEntriesPerCat {
		diff.Disconnections = diff.Disconnections[:maxDiffEntriesPerCat]
	}
	if len(diff.Connections) > maxDiffEntriesPerCat {
		diff.Connections = diff.Connections[:maxDiffEntriesPerCat]
	}
	if len(diff.Errors) > maxDiffEntriesPerCat {
		diff.Errors = diff.Errors[:maxDiffEntriesPerCat]
	}

	return diff
}

func (cm *CheckpointManager) computeActionsDiff(cp *Checkpoint) *ActionsDiff {
	currentTotal := cm.capture.GetActionTotalAdded()
	newCount := int(currentTotal - cp.ActionTotal)
	if newCount <= 0 {
		return &ActionsDiff{}
	}

	// Get all enhanced actions (thread-safe)
	allActions := cm.capture.GetAllEnhancedActions()
	available := len(allActions)
	toRead := newCount
	if toRead > available {
		toRead = available
	}
	// Slice the already-copied actions to get the most recent newCount entries
	newActions := allActions[available-toRead:]

	diff := &ActionsDiff{TotalNew: len(newActions)}

	for i := range newActions {
		if i >= maxDiffEntriesPerCat {
			break
		}
		diff.Actions = append(diff.Actions, ActionEntry{
			Type:      newActions[i].Type,
			URL:       newActions[i].URL,
			Timestamp: newActions[i].Timestamp,
		})
	}

	return diff
}

// ============================================
// Severity and summary
// ============================================

func (cm *CheckpointManager) determineSeverity(resp DiffResponse) string {
	// Error: console errors or network failures
	if resp.Console != nil && len(resp.Console.Errors) > 0 {
		return "error"
	}
	if resp.Network != nil && len(resp.Network.Failures) > 0 {
		return "error"
	}

	// Warning: console warnings or WebSocket disconnections
	if resp.Console != nil && len(resp.Console.Warnings) > 0 {
		return "warning"
	}
	if resp.WebSocket != nil && len(resp.WebSocket.Disconnections) > 0 {
		return "warning"
	}

	return "clean"
}

func (cm *CheckpointManager) buildSummary(resp DiffResponse) string {
	if resp.Severity == "clean" {
		return "No significant changes."
	}

	var parts []string

	if resp.Console != nil && len(resp.Console.Errors) > 0 {
		count := 0
		for _, e := range resp.Console.Errors {
			count += e.Count
		}
		parts = append(parts, fmt.Sprintf("%d new console error(s)", count))
	}

	if resp.Network != nil && len(resp.Network.Failures) > 0 {
		parts = append(parts, fmt.Sprintf("%d network failure(s)", len(resp.Network.Failures)))
	}

	if resp.Console != nil && len(resp.Console.Warnings) > 0 {
		count := 0
		for _, w := range resp.Console.Warnings {
			count += w.Count
		}
		parts = append(parts, fmt.Sprintf("%d new console warning(s)", count))
	}

	if resp.WebSocket != nil && len(resp.WebSocket.Disconnections) > 0 {
		parts = append(parts, fmt.Sprintf("%d websocket disconnection(s)", len(resp.WebSocket.Disconnections)))
	}

	if len(parts) == 0 {
		return "No significant changes."
	}

	return strings.Join(parts, ", ")
}

// ============================================
// Known endpoints
// ============================================

func (cm *CheckpointManager) buildKnownEndpoints(existing map[string]endpointState) map[string]endpointState {
	result := make(map[string]endpointState)

	// Copy existing
	for k, v := range existing {
		result[k] = v
	}

	// Update with current network bodies (thread-safe)
	for _, body := range cm.capture.GetNetworkBodies() {
		path := capture.ExtractURLPath(body.URL)
		result[path] = endpointState{
			Status:   body.Status,
			Duration: body.Duration,
		}
	}

	return result
}

// ============================================
// Utility functions
// ============================================

var (
	uuidRegex      = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	largeNumberRe  = regexp.MustCompile(`\b\d{4,}\b`)
	isoTimestampRe = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z?`)
)

// FingerprintMessage normalizes dynamic content in a message for deduplication
func FingerprintMessage(msg string) string {
	// Replace UUIDs
	result := uuidRegex.ReplaceAllString(msg, "{uuid}")
	// Replace ISO timestamps (before numbers, since timestamps contain numbers)
	result = isoTimestampRe.ReplaceAllString(result, "{ts}")
	// Replace large numbers (4+ digits)
	result = largeNumberRe.ReplaceAllString(result, "{n}")
	return result
}

func truncateMessage(msg string) string {
	if len(msg) <= maxMessageLen {
		return msg
	}
	// Truncate at a valid UTF-8 boundary to avoid splitting multi-byte characters
	truncated := msg[:maxMessageLen]
	for len(truncated) > 0 && !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}
