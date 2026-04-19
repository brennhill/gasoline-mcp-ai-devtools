// Purpose: Benchmark pagination and cursor throughput and latency.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"testing"
)

// BenchmarkParseCursor measures cursor parsing performance
func BenchmarkParseCursor(b *testing.B) {
	cursor := "2026-01-30T10:15:23.456789Z:1234"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseCursor(cursor)
	}
}

// BenchmarkBuildCursor measures cursor building performance
func BenchmarkBuildCursor(b *testing.B) {
	ts := "2026-01-30T10:15:23.456789Z"
	seq := int64(1234)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildCursor(ts, seq)
	}
}

// BenchmarkEnrichLogEntries measures log entry enrichment performance
func BenchmarkEnrichLogEntries(b *testing.B) {
	// Create 1000 log entries
	entries := make([]LogEntry, 1000)

	for i := 0; i < 1000; i++ {
		entries[i] = LogEntry{
			"message":   "test log entry",
			"level":     "info",
			"timestamp": "2026-01-30T10:15:23Z",
		}
	}

	totalAdded := int64(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EnrichLogEntries(entries, totalAdded)
	}
}

// BenchmarkApplyLogCursorPagination measures pagination performance on enriched datasets
func BenchmarkApplyLogCursorPagination(b *testing.B) {
	// Create and enrich 1000 log entries
	entries := make([]LogEntry, 1000)

	for i := 0; i < 1000; i++ {
		entries[i] = LogEntry{
			"message":   "test log entry",
			"level":     "info",
			"timestamp": "2026-01-30T10:15:23Z",
		}
	}

	totalAdded := int64(1000)
	enriched := EnrichLogEntries(entries, totalAdded)
	cursor := "2026-01-30T10:15:23Z:500"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ApplyLogCursorPagination(enriched, cursor, "", "", 100, false)
	}
}
