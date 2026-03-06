// Purpose: Determines checkpoint diff severity and builds human-readable summary strings.
// Why: Separates severity determination and summary formatting from diff data computation.
package checkpoint

import (
	"fmt"
	"strings"
)

func (cm *CheckpointManager) determineSeverity(resp DiffResponse) string {
	if hasConsoleErrors(resp) || hasNetworkFailures(resp) {
		return "error"
	}
	if hasConsoleWarnings(resp) || hasWSDisconnections(resp) {
		return "warning"
	}
	return "clean"
}

func hasConsoleErrors(resp DiffResponse) bool {
	return resp.Console != nil && len(resp.Console.Errors) > 0
}

func hasNetworkFailures(resp DiffResponse) bool {
	return resp.Network != nil && len(resp.Network.Failures) > 0
}

func hasConsoleWarnings(resp DiffResponse) bool {
	return resp.Console != nil && len(resp.Console.Warnings) > 0
}

func hasWSDisconnections(resp DiffResponse) bool {
	return resp.WebSocket != nil && len(resp.WebSocket.Disconnections) > 0
}

func (cm *CheckpointManager) buildSummary(resp DiffResponse) string {
	if resp.Severity == "clean" {
		return "No significant changes."
	}
	parts := collectSummaryParts(resp)
	if len(parts) == 0 {
		return "No significant changes."
	}
	return strings.Join(parts, ", ")
}

func collectSummaryParts(resp DiffResponse) []string {
	var parts []string
	if hasConsoleErrors(resp) {
		parts = append(parts, fmt.Sprintf("%d new console error(s)", sumConsoleCounts(resp.Console.Errors)))
	}
	if hasNetworkFailures(resp) {
		parts = append(parts, fmt.Sprintf("%d network failure(s)", len(resp.Network.Failures)))
	}
	if hasConsoleWarnings(resp) {
		parts = append(parts, fmt.Sprintf("%d new console warning(s)", sumConsoleCounts(resp.Console.Warnings)))
	}
	if hasWSDisconnections(resp) {
		parts = append(parts, fmt.Sprintf("%d websocket disconnection(s)", len(resp.WebSocket.Disconnections)))
	}
	return parts
}

func sumConsoleCounts(entries []ConsoleEntry) int {
	total := 0
	for _, e := range entries {
		total += e.Count
	}
	return total
}
