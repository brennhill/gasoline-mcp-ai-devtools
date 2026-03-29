// Purpose: Snapshot filtering/stat helpers for CI state exports.
// Why: Keeps data-shaping logic separate from HTTP transport handlers.

package main

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

// filterLogsSince returns only log entries with timestamps after the given time.
// Uses the server's addedAt timestamps for ordering, falling back to the "ts" field.
func filterLogsSince(logs []LogEntry, since time.Time) []LogEntry {
	result := make([]LogEntry, 0, len(logs))
	for _, entry := range logs {
		ts, ok := entry["ts"].(string)
		if !ok {
			continue
		}
		entryTime, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			continue
		}
		if entryTime.After(since) {
			result = append(result, entry)
		}
	}
	return result
}

// computeSnapshotStats computes summary statistics for a snapshot.
func computeSnapshotStats(logs []LogEntry, wsEvents []capture.WebSocketEvent, networkBodies []capture.NetworkBody) SnapshotStats {
	stats := SnapshotStats{
		TotalLogs: len(logs),
	}

	for _, entry := range logs {
		level, _ := entry["level"].(string)
		switch level {
		case "error":
			stats.ErrorCount++
		case "warn", "warning":
			stats.WarningCount++
		}
	}

	for _, body := range networkBodies {
		if body.Status >= 400 {
			stats.NetworkFailures++
		}
	}

	// Count unique WS connections.
	connSeen := make(map[string]bool)
	for _, evt := range wsEvents {
		if evt.URL != "" {
			connSeen[evt.URL] = true
		}
	}
	stats.WSConnections = len(connSeen)

	return stats
}
