// Purpose: Adapts generic cursor pagination to console/network log entries.
// Why: Keeps log field extraction/serialization separate from generic pagination behavior.
// Docs: docs/features/feature/pagination/index.md

package pagination

import (
	"fmt"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/types"
)

// LogEntry is a type alias for the canonical definition in internal/types.
type LogEntry = types.LogEntry

// entryStr extracts a string field from a LogEntry map.
// Returns empty string if field doesn't exist or isn't a string.
func entryStr(entry LogEntry, key string) string {
	if v, ok := entry[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// entryDisplay extracts a field from a LogEntry and returns it as a display string.
// Handles numeric types by converting to string representation.
func entryDisplay(entry LogEntry, key string) string {
	if v, ok := entry[key]; ok {
		switch val := v.(type) {
		case string:
			return val
		case int:
			return fmt.Sprintf("%d", val)
		case int64:
			return fmt.Sprintf("%d", val)
		case float64:
			return fmt.Sprintf("%.0f", val)
		}
	}
	return ""
}

// ============================================
// Log Pagination
// ============================================

// LogEntryWithSequence pairs a log entry with its sequence number and timestamp for pagination.
type LogEntryWithSequence struct {
	Entry     LogEntry
	Sequence  int64
	Timestamp string
}

// GetSequence implements Sequenced.
func (e LogEntryWithSequence) GetSequence() int64 { return e.Sequence }

// GetTimestamp implements Sequenced.
func (e LogEntryWithSequence) GetTimestamp() string { return e.Timestamp }

// EnrichLogEntries adds sequence numbers and timestamps to entries for pagination.
// Must be called with the UNFILTERED entry list to get correct sequence numbers.
func EnrichLogEntries(entries []LogEntry, logTotalAdded int64) []LogEntryWithSequence {
	enriched := make([]LogEntryWithSequence, len(entries))
	baseSeq := logTotalAdded - int64(len(entries)) + 1

	for i, entry := range entries {
		enriched[i] = LogEntryWithSequence{
			Entry:     entry,
			Sequence:  baseSeq + int64(i),
			Timestamp: entryStr(entry, "ts"),
		}
	}

	return enriched
}

// ApplyLogCursorPagination applies cursor-based pagination to log entries with sequence metadata.
// Returns filtered entries, cursor metadata, and any error.
func ApplyLogCursorPagination(
	enrichedEntries []LogEntryWithSequence,
	afterCursor, beforeCursor, sinceCursor string,
	limit int,
	restartOnEviction bool,
) ([]LogEntryWithSequence, *CursorPaginationMetadata, error) {
	return ApplyCursorPagination(enrichedEntries, CursorParams{
		AfterCursor:       afterCursor,
		BeforeCursor:      beforeCursor,
		SinceCursor:       sinceCursor,
		Limit:             limit,
		RestartOnEviction: restartOnEviction,
	})
}

// SerializeLogEntryWithSequence converts a LogEntryWithSequence to a JSON-serializable map.
func SerializeLogEntryWithSequence(enriched LogEntryWithSequence) map[string]any {
	result := map[string]any{
		"level":     entryStr(enriched.Entry, "level"),
		"message":   entryStr(enriched.Entry, "message"),
		"source":    entryStr(enriched.Entry, "source"),
		"timestamp": enriched.Timestamp,
		"sequence":  enriched.Sequence,
	}

	if tabID := entryDisplay(enriched.Entry, "tabId"); tabID != "" {
		result["tab_id"] = tabID
	}

	return result
}
