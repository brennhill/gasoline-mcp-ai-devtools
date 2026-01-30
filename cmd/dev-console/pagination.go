// pagination.go — Cursor-based pagination helpers for observe() responses
package main

import (
	"fmt"
)

// LogEntryWithSequence pairs a log entry with its sequence number and timestamp for pagination.
type LogEntryWithSequence struct {
	Entry     LogEntry
	Sequence  int64
	Timestamp string
}

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
	metadata := &CursorPaginationMetadata{
		Total: len(enrichedEntries),
	}

	// No cursor specified - just apply limit
	if afterCursor == "" && beforeCursor == "" && sinceCursor == "" {
		if limit > 0 && limit < len(enrichedEntries) {
			enrichedEntries = enrichedEntries[len(enrichedEntries)-limit:]
		}
		metadata.Count = len(enrichedEntries)
		if len(enrichedEntries) > 0 {
			last := enrichedEntries[len(enrichedEntries)-1]
			metadata.Cursor = BuildCursor(last.Timestamp, last.Sequence)
			metadata.OldestTimestamp = enrichedEntries[0].Timestamp
			metadata.NewestTimestamp = last.Timestamp
		}
		return enrichedEntries, metadata, nil
	}

	// Determine which cursor to use
	cursorStr := afterCursor
	cursorType := "after"
	if cursorStr == "" {
		cursorStr = beforeCursor
		cursorType = "before"
	}
	if cursorStr == "" {
		cursorStr = sinceCursor
		cursorType = "since"
	}

	cursor, err := ParseCursor(cursorStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	// Check if cursor expired (buffer overflow detection)
	if len(enrichedEntries) > 0 && cursor.Sequence > 0 {
		oldestSeq := enrichedEntries[0].Sequence
		if cursor.Sequence < oldestSeq {
			// Cursor expired - data was evicted
			if restartOnEviction {
				// Auto-restart from oldest available
				metadata.CursorRestarted = true
				metadata.OriginalCursor = cursorStr
				metadata.Warning = fmt.Sprintf("Cursor expired (buffer overflow). Restarted from oldest available entry. Lost entries: %d to %d", cursor.Sequence, oldestSeq-1)
				// Don't filter - return from beginning
			} else {
				return nil, nil, fmt.Errorf("cursor expired (buffer overflow). Requested sequence %d, oldest available is %d. Lost %d entries", cursor.Sequence, oldestSeq, oldestSeq-cursor.Sequence)
			}
		}
	}

	// Filter based on cursor (if not restarted)
	var filtered []LogEntryWithSequence
	if !metadata.CursorRestarted {
		for _, enriched := range enrichedEntries {
			include := false
			switch cursorType {
			case "after":
				// Backward pagination: return entries older than cursor
				include = cursor.IsOlder(enriched.Timestamp, enriched.Sequence)
			case "before":
				// Forward pagination: return entries newer than cursor
				include = cursor.IsNewer(enriched.Timestamp, enriched.Sequence)
			case "since":
				// Convenience: all entries newer than or equal to cursor
				include = cursor.IsNewer(enriched.Timestamp, enriched.Sequence) ||
					(enriched.Timestamp == cursor.Timestamp && enriched.Sequence == cursor.Sequence)
			}

			if include {
				filtered = append(filtered, enriched)
			}
		}
		enrichedEntries = filtered
	}

	// Track count before limit for HasMore calculation
	countBeforeLimit := len(enrichedEntries)

	// Apply limit (after cursor filtering)
	if limit > 0 && limit < len(enrichedEntries) {
		// If cursor was restarted, start from beginning (oldest)
		// Otherwise use normal pagination direction
		if metadata.CursorRestarted || afterCursor == "" {
			// Forward pagination or restart: take oldest N entries (from beginning)
			enrichedEntries = enrichedEntries[:limit]
		} else {
			// Backward pagination: take newest N entries (from end)
			enrichedEntries = enrichedEntries[len(enrichedEntries)-limit:]
		}
	}

	// Build metadata
	metadata.Count = len(enrichedEntries)
	if len(enrichedEntries) > 0 {
		// Oldest timestamp
		metadata.OldestTimestamp = enrichedEntries[0].Timestamp

		// Newest timestamp and cursor
		last := enrichedEntries[len(enrichedEntries)-1]
		metadata.NewestTimestamp = last.Timestamp
		metadata.Cursor = BuildCursor(last.Timestamp, last.Sequence)

		// Check if there are more entries available (we trimmed due to limit)
		if afterCursor != "" && countBeforeLimit > len(enrichedEntries) {
			metadata.HasMore = true
		}
	}

	return enrichedEntries, metadata, nil
}

// SerializeLogEntryWithSequence converts a LogEntryWithSequence to a JSON-serializable map.
func SerializeLogEntryWithSequence(enriched LogEntryWithSequence) map[string]interface{} {
	result := map[string]interface{}{
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
// Action Pagination Helpers
// ============================================

// ActionEntryWithSequence pairs an action entry with its sequence number and timestamp for pagination.
type ActionEntryWithSequence struct {
	Entry     EnhancedAction
	Sequence  int64
	Timestamp string // RFC3339 normalized timestamp
}

// EnrichActionEntries adds sequence numbers and normalized timestamps to action entries for pagination.
// Must be called with the UNFILTERED entry list to get correct sequence numbers.
func EnrichActionEntries(actions []EnhancedAction, actionTotalAdded int64) []ActionEntryWithSequence {
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
	metadata := &CursorPaginationMetadata{
		Total: len(enrichedEntries),
	}

	// No cursor specified - just apply limit
	if afterCursor == "" && beforeCursor == "" && sinceCursor == "" {
		if limit > 0 && limit < len(enrichedEntries) {
			enrichedEntries = enrichedEntries[len(enrichedEntries)-limit:]
		}
		metadata.Count = len(enrichedEntries)
		if len(enrichedEntries) > 0 {
			last := enrichedEntries[len(enrichedEntries)-1]
			metadata.Cursor = BuildCursor(last.Timestamp, last.Sequence)
			metadata.OldestTimestamp = enrichedEntries[0].Timestamp
			metadata.NewestTimestamp = last.Timestamp
		}
		return enrichedEntries, metadata, nil
	}

	// Determine which cursor to use
	cursorStr := afterCursor
	cursorType := "after"
	if cursorStr == "" {
		cursorStr = beforeCursor
		cursorType = "before"
	}
	if cursorStr == "" {
		cursorStr = sinceCursor
		cursorType = "since"
	}

	cursor, err := ParseCursor(cursorStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	// Check if cursor expired (buffer overflow detection)
	if len(enrichedEntries) > 0 && cursor.Sequence > 0 {
		oldestSeq := enrichedEntries[0].Sequence
		if cursor.Sequence < oldestSeq {
			// Cursor expired - data was evicted
			if restartOnEviction {
				// Auto-restart from oldest available
				metadata.CursorRestarted = true
				metadata.OriginalCursor = cursorStr
				metadata.Warning = fmt.Sprintf("Cursor expired (buffer overflow). Restarted from oldest available entry. Lost entries: %d to %d", cursor.Sequence, oldestSeq-1)
				// Don't filter - return from beginning
			} else {
				return nil, nil, fmt.Errorf("cursor expired (buffer overflow). Requested sequence %d, oldest available is %d. Lost %d entries", cursor.Sequence, oldestSeq, oldestSeq-cursor.Sequence)
			}
		}
	}

	// Filter based on cursor (if not restarted)
	var filtered []ActionEntryWithSequence
	if !metadata.CursorRestarted {
		for _, enriched := range enrichedEntries {
			include := false
			switch cursorType {
			case "after":
				// Backward pagination: return entries older than cursor
				include = cursor.IsOlder(enriched.Timestamp, enriched.Sequence)
			case "before":
				// Forward pagination: return entries newer than cursor
				include = cursor.IsNewer(enriched.Timestamp, enriched.Sequence)
			case "since":
				// Convenience: all entries newer than or equal to cursor
				include = cursor.IsNewer(enriched.Timestamp, enriched.Sequence) ||
					(enriched.Timestamp == cursor.Timestamp && enriched.Sequence == cursor.Sequence)
			}

			if include {
				filtered = append(filtered, enriched)
			}
		}
		enrichedEntries = filtered
	}

	// Track count before limit for HasMore calculation
	countBeforeLimit := len(enrichedEntries)

	// Apply limit (after cursor filtering)
	if limit > 0 && limit < len(enrichedEntries) {
		// If cursor was restarted, start from beginning (oldest)
		// Otherwise use normal pagination direction
		if metadata.CursorRestarted || afterCursor == "" {
			// Forward pagination or restart: take oldest N entries (from beginning)
			enrichedEntries = enrichedEntries[:limit]
		} else {
			// Backward pagination: take newest N entries (from end)
			enrichedEntries = enrichedEntries[len(enrichedEntries)-limit:]
		}
	}

	// Build metadata
	metadata.Count = len(enrichedEntries)
	if len(enrichedEntries) > 0 {
		// Oldest timestamp
		metadata.OldestTimestamp = enrichedEntries[0].Timestamp

		// Newest timestamp and cursor
		last := enrichedEntries[len(enrichedEntries)-1]
		metadata.NewestTimestamp = last.Timestamp
		metadata.Cursor = BuildCursor(last.Timestamp, last.Sequence)

		// Check if there are more entries available (we trimmed due to limit)
		if afterCursor != "" && countBeforeLimit > len(enrichedEntries) {
			metadata.HasMore = true
		}
	}

	return enrichedEntries, metadata, nil
}

// SerializeActionEntryWithSequence converts an ActionEntryWithSequence to a JSON-serializable map.
func SerializeActionEntryWithSequence(enriched ActionEntryWithSequence) map[string]interface{} {
	result := map[string]interface{}{
		"type":      enriched.Entry.Type,
		"timestamp": enriched.Timestamp,
		"sequence":  enriched.Sequence,
	}

	// Add optional fields if present
	if enriched.Entry.URL != "" {
		result["url"] = enriched.Entry.URL
	}
	if enriched.Entry.Selectors != nil && len(enriched.Entry.Selectors) > 0 {
		result["selectors"] = enriched.Entry.Selectors
	}
	if enriched.Entry.Value != "" {
		result["value"] = enriched.Entry.Value
	}
	if enriched.Entry.InputType != "" {
		result["input_type"] = enriched.Entry.InputType
	}
	if enriched.Entry.Key != "" {
		result["key"] = enriched.Entry.Key
	}
	if enriched.Entry.FromURL != "" {
		result["from_url"] = enriched.Entry.FromURL
	}
	if enriched.Entry.ToURL != "" {
		result["to_url"] = enriched.Entry.ToURL
	}
	if enriched.Entry.SelectedValue != "" {
		result["selected_value"] = enriched.Entry.SelectedValue
	}
	if enriched.Entry.SelectedText != "" {
		result["selected_text"] = enriched.Entry.SelectedText
	}
	if enriched.Entry.ScrollY != 0 {
		result["scroll_y"] = enriched.Entry.ScrollY
	}

	// Add tabId if present
	if enriched.Entry.TabId > 0 {
		result["tab_id"] = enriched.Entry.TabId
	}

	return result
}

// ============================================
// WebSocket Pagination Helpers
// ============================================

// WebSocketEntryWithSequence pairs a websocket event with its sequence number and timestamp for pagination.
type WebSocketEntryWithSequence struct {
	Entry     WebSocketEvent
	Sequence  int64
	Timestamp string // RFC3339 timestamp (already string in WebSocketEvent)
}

// EnrichWebSocketEntries adds sequence numbers to websocket events for pagination.
// Must be called with the UNFILTERED entry list to get correct sequence numbers.
func EnrichWebSocketEntries(events []WebSocketEvent, wsTotalAdded int64) []WebSocketEntryWithSequence {
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
	metadata := &CursorPaginationMetadata{
		Total: len(enrichedEntries),
	}

	// No cursor specified - just apply limit
	if afterCursor == "" && beforeCursor == "" && sinceCursor == "" {
		if limit > 0 && limit < len(enrichedEntries) {
			enrichedEntries = enrichedEntries[len(enrichedEntries)-limit:]
		}
		metadata.Count = len(enrichedEntries)
		if len(enrichedEntries) > 0 {
			last := enrichedEntries[len(enrichedEntries)-1]
			metadata.Cursor = BuildCursor(last.Timestamp, last.Sequence)
			metadata.OldestTimestamp = enrichedEntries[0].Timestamp
			metadata.NewestTimestamp = last.Timestamp
		}
		return enrichedEntries, metadata, nil
	}

	// Determine which cursor to use
	cursorStr := afterCursor
	cursorType := "after"
	if cursorStr == "" {
		cursorStr = beforeCursor
		cursorType = "before"
	}
	if cursorStr == "" {
		cursorStr = sinceCursor
		cursorType = "since"
	}

	cursor, err := ParseCursor(cursorStr)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid cursor format: %w", err)
	}

	// Check if cursor expired (buffer overflow detection)
	if len(enrichedEntries) > 0 && cursor.Sequence > 0 {
		oldestSeq := enrichedEntries[0].Sequence
		if cursor.Sequence < oldestSeq {
			// Cursor expired - data was evicted
			if restartOnEviction {
				// Auto-restart from oldest available
				metadata.CursorRestarted = true
				metadata.OriginalCursor = cursorStr
				metadata.Warning = fmt.Sprintf("Cursor expired (buffer overflow). Restarted from oldest available entry. Lost entries: %d to %d", cursor.Sequence, oldestSeq-1)
				// Don't filter - return from beginning
			} else {
				return nil, nil, fmt.Errorf("cursor expired (buffer overflow). Requested sequence %d, oldest available is %d. Lost %d entries", cursor.Sequence, oldestSeq, oldestSeq-cursor.Sequence)
			}
		}
	}

	// Filter based on cursor (if not restarted)
	var filtered []WebSocketEntryWithSequence
	if !metadata.CursorRestarted {
		for _, enriched := range enrichedEntries {
			include := false
			switch cursorType {
			case "after":
				// Backward pagination: return entries older than cursor
				include = cursor.IsOlder(enriched.Timestamp, enriched.Sequence)
			case "before":
				// Forward pagination: return entries newer than cursor
				include = cursor.IsNewer(enriched.Timestamp, enriched.Sequence)
			case "since":
				// Convenience: all entries newer than or equal to cursor
				include = cursor.IsNewer(enriched.Timestamp, enriched.Sequence) ||
					(enriched.Timestamp == cursor.Timestamp && enriched.Sequence == cursor.Sequence)
			}

			if include {
				filtered = append(filtered, enriched)
			}
		}
		enrichedEntries = filtered
	}

	// Track count before limit for HasMore calculation
	countBeforeLimit := len(enrichedEntries)

	// Apply limit (after cursor filtering)
	if limit > 0 && limit < len(enrichedEntries) {
		// If cursor was restarted, start from beginning (oldest)
		// Otherwise use normal pagination direction
		if metadata.CursorRestarted || afterCursor == "" {
			// Forward pagination or restart: take oldest N entries (from beginning)
			enrichedEntries = enrichedEntries[:limit]
		} else {
			// Backward pagination: take newest N entries (from end)
			enrichedEntries = enrichedEntries[len(enrichedEntries)-limit:]
		}
	}

	// Build metadata
	metadata.Count = len(enrichedEntries)
	if len(enrichedEntries) > 0 {
		// Oldest timestamp
		metadata.OldestTimestamp = enrichedEntries[0].Timestamp

		// Newest timestamp and cursor
		last := enrichedEntries[len(enrichedEntries)-1]
		metadata.NewestTimestamp = last.Timestamp
		metadata.Cursor = BuildCursor(last.Timestamp, last.Sequence)

		// Check if there are more entries available (we trimmed due to limit)
		if afterCursor != "" && countBeforeLimit > len(enrichedEntries) {
			metadata.HasMore = true
		}
	}

	return enrichedEntries, metadata, nil
}

// SerializeWebSocketEntryWithSequence converts a WebSocketEntryWithSequence to a JSON-serializable map.
func SerializeWebSocketEntryWithSequence(enriched WebSocketEntryWithSequence) map[string]interface{} {
	result := map[string]interface{}{
		"event":     enriched.Entry.Event,
		"id":        enriched.Entry.ID,
		"timestamp": enriched.Timestamp,
		"sequence":  enriched.Sequence,
	}

	// Add optional fields if present
	if enriched.Entry.Type != "" {
		result["type"] = enriched.Entry.Type
	}
	if enriched.Entry.URL != "" {
		result["url"] = enriched.Entry.URL
	}
	if enriched.Entry.Direction != "" {
		result["direction"] = enriched.Entry.Direction
	}
	if enriched.Entry.Data != "" {
		result["data"] = enriched.Entry.Data
	}
	if enriched.Entry.Size > 0 {
		result["size"] = enriched.Entry.Size
	}
	if enriched.Entry.CloseCode > 0 {
		result["code"] = enriched.Entry.CloseCode
	}
	if enriched.Entry.CloseReason != "" {
		result["reason"] = enriched.Entry.CloseReason
	}
	if enriched.Entry.BinaryFormat != "" {
		result["binary_format"] = enriched.Entry.BinaryFormat
	}
	if enriched.Entry.FormatConfidence > 0 {
		result["format_confidence"] = enriched.Entry.FormatConfidence
	}
	if enriched.Entry.Sampled != nil {
		result["sampled"] = enriched.Entry.Sampled
	}

	// Add tabId if present
	if enriched.Entry.TabId > 0 {
		result["tab_id"] = enriched.Entry.TabId
	}

	return result
}
