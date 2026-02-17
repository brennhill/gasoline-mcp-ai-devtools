// Purpose: Owns pagination.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// pagination.go — Cursor-based pagination helpers for observe() responses.
// Uses generic ApplyCursorPagination to eliminate duplication across log, action, and websocket types.
package pagination

import (
	"fmt"

	"github.com/dev-console/dev-console/internal/capture"
)

// Type aliases for imported types to avoid qualifying every use.
type (
	// LogEntry is a map-based log entry from the capture package.
	// any: JSON log entries have dynamic fields; map allows flexible access without schema.
	LogEntry = map[string]any
	// EnhancedAction is a user action from the capture package.
	EnhancedAction = capture.EnhancedAction
	// WebSocketEvent is a WebSocket event from the capture package.
	WebSocketEvent = capture.WebSocketEvent
)

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
// Sequenced is the interface for entries with pagination metadata.
// ============================================

// Sequenced provides access to sequence and timestamp for cursor pagination.
type Sequenced interface {
	GetSequence() int64
	GetTimestamp() string
}

// CursorParams bundles cursor pagination parameters.
type CursorParams struct {
	AfterCursor       string
	BeforeCursor      string
	SinceCursor       string
	Limit             int
	RestartOnEviction bool
}

// resolveCursorType determines which cursor string and type to use.
func resolveCursorType(after, before, since string) (string, string) {
	if after != "" {
		return after, "after"
	}
	if before != "" {
		return before, "before"
	}
	if since != "" {
		return since, "since"
	}
	return "", ""
}

// checkCursorExpired checks if the cursor has expired due to buffer overflow.
// Returns true if cursor expired and was handled (restart or error).
func checkCursorExpired[T Sequenced](
	entries []T, cursor Cursor, cursorStr string,
	restartOnEviction bool, metadata *CursorPaginationMetadata,
) error {
	if len(entries) == 0 || cursor.Sequence <= 0 {
		return nil
	}
	oldestSeq := entries[0].GetSequence()
	if cursor.Sequence >= oldestSeq {
		return nil
	}
	if restartOnEviction {
		metadata.CursorRestarted = true
		metadata.OriginalCursor = cursorStr
		metadata.Warning = fmt.Sprintf("Cursor expired (buffer overflow). Restarted from oldest available entry. Lost entries: %d to %d",
			cursor.Sequence, oldestSeq-1)
		return nil
	}
	return fmt.Errorf("cursor expired (buffer overflow). Requested sequence %d, oldest available is %d. Lost %d entries",
		cursor.Sequence, oldestSeq, oldestSeq-cursor.Sequence)
}

// filterByCursor filters entries using the cursor comparison for the given cursor type.
func filterByCursor[T Sequenced](entries []T, cursor Cursor, cursorType string) []T {
	var filtered []T
	for _, e := range entries {
		if matchesCursorType(cursor, cursorType, e.GetTimestamp(), e.GetSequence()) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// matchesCursorType returns true if an entry matches the cursor filter for the given type.
func matchesCursorType(cursor Cursor, cursorType, ts string, seq int64) bool {
	switch cursorType {
	case "after":
		return cursor.IsOlder(ts, seq)
	case "before":
		return cursor.IsNewer(ts, seq)
	case "since":
		return cursor.IsNewer(ts, seq) || (ts == cursor.Timestamp && seq == cursor.Sequence)
	default:
		return false
	}
}

// applyLimit trims entries to limit, respecting pagination direction.
func applyLimit[T Sequenced](entries []T, limit int, forwardPagination bool) []T {
	if limit <= 0 || limit >= len(entries) {
		return entries
	}
	if forwardPagination {
		return entries[:limit]
	}
	return entries[len(entries)-limit:]
}

// buildMetadata populates pagination metadata from the result set.
func buildMetadata[T Sequenced](entries []T, afterCursor string, countBeforeLimit int, metadata *CursorPaginationMetadata) {
	metadata.Count = len(entries)
	if len(entries) == 0 {
		return
	}
	metadata.OldestTimestamp = entries[0].GetTimestamp()
	last := entries[len(entries)-1]
	metadata.NewestTimestamp = last.GetTimestamp()
	metadata.Cursor = BuildCursor(last.GetTimestamp(), last.GetSequence())
	if countBeforeLimit > len(entries) {
		metadata.HasMore = true
	}
}

// ApplyCursorPagination is the generic cursor pagination implementation.
// Works for any Sequenced type (logs, actions, websocket events).
func ApplyCursorPagination[T Sequenced](entries []T, p CursorParams) ([]T, *CursorPaginationMetadata, error) {
	metadata := &CursorPaginationMetadata{Total: len(entries)}

	cursorStr, cursorType := resolveCursorType(p.AfterCursor, p.BeforeCursor, p.SinceCursor)

	// No cursor specified - just apply limit
	if cursorStr == "" {
		countBeforeLimit := len(entries)
		entries = applyLimit(entries, p.Limit, false)
		buildMetadata(entries, p.AfterCursor, countBeforeLimit, metadata)
		return entries, metadata, nil
	}

	cursor, err := ParseCursor(cursorStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	if err := checkCursorExpired(entries, cursor, cursorStr, p.RestartOnEviction, metadata); err != nil {
		return nil, nil, err
	}

	if !metadata.CursorRestarted {
		entries = filterByCursor(entries, cursor, cursorType)
	}

	countBeforeLimit := len(entries)
	forwardPagination := metadata.CursorRestarted || p.AfterCursor == ""
	entries = applyLimit(entries, p.Limit, forwardPagination)
	buildMetadata(entries, p.AfterCursor, countBeforeLimit, metadata)
	return entries, metadata, nil
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

	// Add tabId if present
	if tabId := entryDisplay(enriched.Entry, "tabId"); tabId != "" {
		result["tab_id"] = tabId
	}

	return result
}

// ============================================
// Action Pagination
// ============================================

// ActionEntryWithSequence pairs an action entry with its sequence number and timestamp for pagination.
type ActionEntryWithSequence struct {
	Entry     EnhancedAction
	Sequence  int64
	Timestamp string // RFC3339 normalized timestamp
}

// GetSequence implements Sequenced.
func (e ActionEntryWithSequence) GetSequence() int64 { return e.Sequence }

// GetTimestamp implements Sequenced.
func (e ActionEntryWithSequence) GetTimestamp() string { return e.Timestamp }

// EnrichActionEntries adds sequence numbers and normalized timestamps to action entries for pagination.
// Must be called with the UNFILTERED entry list to get correct sequence numbers.
func EnrichActionEntries(actions []capture.EnhancedAction, actionTotalAdded int64) []ActionEntryWithSequence {
	enriched := make([]ActionEntryWithSequence, len(actions))
	baseSeq := actionTotalAdded - int64(len(actions)) + 1

	for i, action := range actions {
		enriched[i] = ActionEntryWithSequence{
			Entry:     action,
			Sequence:  baseSeq + int64(i),
			Timestamp: NormalizeTimestamp(action.Timestamp), // Convert int64 to RFC3339
		}
	}

	return enriched
}

// ApplyActionCursorPagination applies cursor-based pagination to action entries with sequence metadata.
// Returns filtered entries, cursor metadata, and any error.
func ApplyActionCursorPagination(
	enrichedEntries []ActionEntryWithSequence,
	afterCursor, beforeCursor, sinceCursor string,
	limit int,
	restartOnEviction bool,
) ([]ActionEntryWithSequence, *CursorPaginationMetadata, error) {
	return ApplyCursorPagination(enrichedEntries, CursorParams{
		AfterCursor:       afterCursor,
		BeforeCursor:      beforeCursor,
		SinceCursor:       sinceCursor,
		Limit:             limit,
		RestartOnEviction: restartOnEviction,
	})
}

// SerializeActionEntryWithSequence converts an ActionEntryWithSequence to a JSON-serializable map.
func SerializeActionEntryWithSequence(enriched ActionEntryWithSequence) map[string]any {
	result := map[string]any{
		"type":      enriched.Entry.Type,
		"timestamp": enriched.Timestamp,
		"sequence":  enriched.Sequence,
	}

	addNonEmpty(result, "url", enriched.Entry.URL)
	if len(enriched.Entry.Selectors) > 0 {
		result["selectors"] = enriched.Entry.Selectors
	}
	addNonEmpty(result, "value", enriched.Entry.Value)
	addNonEmpty(result, "input_type", enriched.Entry.InputType)
	addNonEmpty(result, "key", enriched.Entry.Key)
	addNonEmpty(result, "from_url", enriched.Entry.FromURL)
	addNonEmpty(result, "to_url", enriched.Entry.ToURL)
	addNonEmpty(result, "selected_value", enriched.Entry.SelectedValue)
	addNonEmpty(result, "selected_text", enriched.Entry.SelectedText)

	if enriched.Entry.ScrollY != 0 {
		result["scroll_y"] = enriched.Entry.ScrollY
	}
	if enriched.Entry.TabId > 0 {
		result["tab_id"] = enriched.Entry.TabId
	}

	return result
}

// ============================================
// WebSocket Pagination
// ============================================

// WebSocketEntryWithSequence pairs a websocket event with its sequence number and timestamp for pagination.
type WebSocketEntryWithSequence struct {
	Entry     WebSocketEvent
	Sequence  int64
	Timestamp string // RFC3339 timestamp (already string in capture.WebSocketEvent)
}

// GetSequence implements Sequenced.
func (e WebSocketEntryWithSequence) GetSequence() int64 { return e.Sequence }

// GetTimestamp implements Sequenced.
func (e WebSocketEntryWithSequence) GetTimestamp() string { return e.Timestamp }

// EnrichWebSocketEntries adds sequence numbers to websocket events for pagination.
// Must be called with the UNFILTERED entry list to get correct sequence numbers.
func EnrichWebSocketEntries(events []capture.WebSocketEvent, wsTotalAdded int64) []WebSocketEntryWithSequence {
	enriched := make([]WebSocketEntryWithSequence, len(events))
	baseSeq := wsTotalAdded - int64(len(events)) + 1

	for i, event := range events {
		enriched[i] = WebSocketEntryWithSequence{
			Entry:     event,
			Sequence:  baseSeq + int64(i),
			Timestamp: event.Timestamp, // Already RFC3339 string
		}
	}

	return enriched
}

// ApplyWebSocketCursorPagination applies cursor-based pagination to websocket events with sequence metadata.
// Returns filtered entries, cursor metadata, and any error.
func ApplyWebSocketCursorPagination(
	enrichedEntries []WebSocketEntryWithSequence,
	afterCursor, beforeCursor, sinceCursor string,
	limit int,
	restartOnEviction bool,
) ([]WebSocketEntryWithSequence, *CursorPaginationMetadata, error) {
	return ApplyCursorPagination(enrichedEntries, CursorParams{
		AfterCursor:       afterCursor,
		BeforeCursor:      beforeCursor,
		SinceCursor:       sinceCursor,
		Limit:             limit,
		RestartOnEviction: restartOnEviction,
	})
}

// SerializeWebSocketEntryWithSequence converts a WebSocketEntryWithSequence to a JSON-serializable map.
func SerializeWebSocketEntryWithSequence(enriched WebSocketEntryWithSequence) map[string]any {
	result := map[string]any{
		"event":     enriched.Entry.Event,
		"id":        enriched.Entry.ID,
		"timestamp": enriched.Timestamp,
		"sequence":  enriched.Sequence,
	}

	addNonEmpty(result, "type", enriched.Entry.Type)
	addNonEmpty(result, "url", enriched.Entry.URL)
	addNonEmpty(result, "direction", enriched.Entry.Direction)
	addNonEmpty(result, "data", enriched.Entry.Data)
	addNonEmpty(result, "reason", enriched.Entry.CloseReason)
	addNonEmpty(result, "binary_format", enriched.Entry.BinaryFormat)

	if enriched.Entry.Size > 0 {
		result["size"] = enriched.Entry.Size
	}
	if enriched.Entry.CloseCode > 0 {
		result["code"] = enriched.Entry.CloseCode
	}
	if enriched.Entry.FormatConfidence > 0 {
		result["format_confidence"] = enriched.Entry.FormatConfidence
	}
	if enriched.Entry.Sampled != nil {
		result["sampled"] = enriched.Entry.Sampled
	}
	if enriched.Entry.TabId > 0 {
		result["tab_id"] = enriched.Entry.TabId
	}

	return result
}

// ============================================
// Shared Serialization Helpers
// ============================================

// addNonEmpty adds a key-value pair to the map only if the string value is non-empty.
func addNonEmpty(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}
