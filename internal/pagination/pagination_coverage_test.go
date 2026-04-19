// Purpose: Coverage-expansion tests for pagination and cursor edge cases and branch paths.
// Docs: docs/features/feature/pagination/index.md

// pagination_coverage_test.go — Targeted tests for uncovered branches in pagination package.
package pagination

import (
	"testing"
)

// ============================================
// ParseCursor — no-colon format error path
// ============================================

func TestParseCursor_NoColon(t *testing.T) {
	t.Parallel()
	_, err := ParseCursor("nocolon")
	if err == nil {
		t.Fatal("ParseCursor(\"nocolon\") expected error, got nil")
	}
	wantSubstr := "invalid cursor format"
	if !contains(err.Error(), wantSubstr) {
		t.Errorf("error = %q, want substring %q", err.Error(), wantSubstr)
	}
}

// ============================================
// IsOlder / IsNewer — RFC3339 fallback paths
// ============================================

func TestIsOlder_RFC3339FallbackCursorTimestamp(t *testing.T) {
	t.Parallel()
	// Cursor timestamp is plain RFC3339 (no nanoseconds) so RFC3339Nano parse
	// fails and the code falls back to RFC3339.
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23Z",
		Sequence:  100,
	}
	// Entry uses RFC3339Nano — no fallback needed for the entry.
	// The cursor parse fails Nano, falls back to RFC3339.
	got := cursor.IsOlder("2026-01-30T10:15:22.000000Z", 99)
	if !got {
		t.Error("expected entry to be older than cursor")
	}
}

func TestIsOlder_RFC3339FallbackEntryTimestamp(t *testing.T) {
	t.Parallel()
	// Cursor timestamp is RFC3339Nano so first parse succeeds.
	// Entry timestamp is plain RFC3339 so Nano parse fails, falls back.
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23.000000Z",
		Sequence:  100,
	}
	got := cursor.IsOlder("2026-01-30T10:15:22Z", 99)
	if !got {
		t.Error("expected entry to be older than cursor (entry RFC3339 fallback)")
	}
}

func TestIsNewer_RFC3339FallbackCursorTimestamp(t *testing.T) {
	t.Parallel()
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23Z",
		Sequence:  100,
	}
	got := cursor.IsNewer("2026-01-30T10:15:24.000000Z", 101)
	if !got {
		t.Error("expected entry to be newer than cursor")
	}
}

func TestIsNewer_RFC3339FallbackEntryTimestamp(t *testing.T) {
	t.Parallel()
	cursor := Cursor{
		Timestamp: "2026-01-30T10:15:23.000000Z",
		Sequence:  100,
	}
	got := cursor.IsNewer("2026-01-30T10:15:24Z", 101)
	if !got {
		t.Error("expected entry to be newer than cursor (entry RFC3339 fallback)")
	}
}

// ============================================
// entryStr — missing key and non-string value
// ============================================

func TestEntryStr_MissingKey(t *testing.T) {
	t.Parallel()
	entry := LogEntry{"other": "value"}
	got := entryStr(entry, "missing")
	if got != "" {
		t.Errorf("entryStr(missing key) = %q, want empty", got)
	}
}

func TestEntryStr_NonStringValue(t *testing.T) {
	t.Parallel()
	entry := LogEntry{"count": 42}
	got := entryStr(entry, "count")
	if got != "" {
		t.Errorf("entryStr(int value) = %q, want empty", got)
	}
}
// ============================================
// resolveCursorType — "since" branch
// ============================================

func TestResolveCursorType_Since(t *testing.T) {
	t.Parallel()
	cursor, cursorType := resolveCursorType("", "", "2026-01-30T10:15:23Z:50")
	if cursor != "2026-01-30T10:15:23Z:50" {
		t.Errorf("cursor = %q, want since cursor string", cursor)
	}
	if cursorType != "since" {
		t.Errorf("cursorType = %q, want %q", cursorType, "since")
	}
}

func TestResolveCursorType_None(t *testing.T) {
	t.Parallel()
	cursor, cursorType := resolveCursorType("", "", "")
	if cursor != "" || cursorType != "" {
		t.Errorf("expected empty strings, got cursor=%q type=%q", cursor, cursorType)
	}
}

// ============================================
// matchesCursorType — "since" and "default" branches
// ============================================

func TestMatchesCursorType_Since_Exact(t *testing.T) {
	t.Parallel()
	cursor := Cursor{Timestamp: "2026-01-30T10:15:23Z", Sequence: 50}
	// "since" includes the exact match
	got := matchesCursorType(cursor, "since", "2026-01-30T10:15:23Z", 50)
	if !got {
		t.Error("since cursor should match exact timestamp+sequence")
	}
}

func TestMatchesCursorType_Since_Newer(t *testing.T) {
	t.Parallel()
	cursor := Cursor{Timestamp: "2026-01-30T10:15:23Z", Sequence: 50}
	got := matchesCursorType(cursor, "since", "2026-01-30T10:15:24Z", 51)
	if !got {
		t.Error("since cursor should match newer entries")
	}
}

func TestMatchesCursorType_Since_Older(t *testing.T) {
	t.Parallel()
	cursor := Cursor{Timestamp: "2026-01-30T10:15:23Z", Sequence: 50}
	got := matchesCursorType(cursor, "since", "2026-01-30T10:15:22Z", 49)
	if got {
		t.Error("since cursor should not match older entries")
	}
}

func TestMatchesCursorType_Default(t *testing.T) {
	t.Parallel()
	cursor := Cursor{Timestamp: "2026-01-30T10:15:23Z", Sequence: 50}
	got := matchesCursorType(cursor, "unknown", "2026-01-30T10:15:23Z", 50)
	if got {
		t.Error("unknown cursor type should return false")
	}
}

// ============================================
// checkCursorExpired — empty entries
// ============================================

func TestCheckCursorExpired_EmptyEntries(t *testing.T) {
	t.Parallel()
	metadata := &CursorPaginationMetadata{}
	err := checkCursorExpired([]LogEntryWithSequence{}, Cursor{Sequence: 10}, ":10", false, metadata)
	if err != nil {
		t.Errorf("checkCursorExpired(empty entries) should return nil, got %v", err)
	}
}

// ============================================
// ApplyCursorPagination — invalid cursor format
// ============================================

func TestApplyCursorPagination_InvalidCursorFormat(t *testing.T) {
	t.Parallel()
	entries := []LogEntryWithSequence{
		{Entry: LogEntry{}, Sequence: 1, Timestamp: "2026-01-30T10:15:23Z"},
	}
	_, _, err := ApplyLogCursorPagination(entries, "invalid-no-colon", "", "", 10, false)
	if err == nil {
		t.Fatal("expected error for invalid cursor format, got nil")
	}
	if !contains(err.Error(), "invalid cursor format") {
		t.Errorf("error = %q, want substring 'invalid cursor format'", err.Error())
	}
}

// ============================================
// ApplyCursorPagination — since cursor
// ============================================

func TestApplyLogCursorPagination_SinceCursor(t *testing.T) {
	t.Parallel()
	entries := []LogEntryWithSequence{
		{Entry: LogEntry{"ts": "2026-01-30T10:15:20Z"}, Sequence: 1, Timestamp: "2026-01-30T10:15:20Z"},
		{Entry: LogEntry{"ts": "2026-01-30T10:15:21Z"}, Sequence: 2, Timestamp: "2026-01-30T10:15:21Z"},
		{Entry: LogEntry{"ts": "2026-01-30T10:15:22Z"}, Sequence: 3, Timestamp: "2026-01-30T10:15:22Z"},
		{Entry: LogEntry{"ts": "2026-01-30T10:15:23Z"}, Sequence: 4, Timestamp: "2026-01-30T10:15:23Z"},
		{Entry: LogEntry{"ts": "2026-01-30T10:15:24Z"}, Sequence: 5, Timestamp: "2026-01-30T10:15:24Z"},
	}
	// since cursor at entry 2 should include entries 2, 3, 4, 5
	sinceCursor := BuildCursor("2026-01-30T10:15:21Z", 2)
	result, metadata, err := ApplyLogCursorPagination(entries, "", "", sinceCursor, 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 4 {
		t.Errorf("result count = %d, want 4 (since includes cursor entry)", len(result))
	}
	if result[0].Sequence != 2 {
		t.Errorf("first sequence = %d, want 2 (inclusive)", result[0].Sequence)
	}
	if result[len(result)-1].Sequence != 5 {
		t.Errorf("last sequence = %d, want 5", result[len(result)-1].Sequence)
	}
	if metadata.Count != 4 {
		t.Errorf("metadata count = %d, want 4", metadata.Count)
	}
	if metadata.Total != 5 {
		t.Errorf("metadata total = %d, want 5", metadata.Total)
	}
}

