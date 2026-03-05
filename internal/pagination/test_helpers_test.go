// Purpose: Shared test assertions/helpers for pagination test suites.
// Docs: docs/features/feature/pagination/index.md

package pagination

import "testing"

func assertPaginationCountAndTotal(t *testing.T, gotCount, wantCount int, metadata *CursorPaginationMetadata, expectedTotal int) {
	t.Helper()
	if metadata == nil {
		t.Fatal("metadata is nil")
	}
	if gotCount != wantCount {
		t.Errorf("Result count = %d, want %d", gotCount, wantCount)
	}
	if metadata.Count != wantCount {
		t.Errorf("Metadata count = %d, want %d", metadata.Count, wantCount)
	}
	if metadata.Total != expectedTotal {
		t.Errorf("Metadata total = %d, want %d", metadata.Total, expectedTotal)
	}
}

func assertPaginationCursorFields(
	t *testing.T,
	metadata *CursorPaginationMetadata,
	oldestTimestamp string,
	newestTimestamp string,
	newestSequence int64,
) {
	t.Helper()
	if metadata == nil {
		t.Fatal("metadata is nil")
	}
	expectedCursor := BuildCursor(newestTimestamp, newestSequence)
	if metadata.Cursor != expectedCursor {
		t.Errorf("Metadata cursor = %v, want %v", metadata.Cursor, expectedCursor)
	}
	if metadata.OldestTimestamp != oldestTimestamp {
		t.Errorf("OldestTimestamp = %v, want %v", metadata.OldestTimestamp, oldestTimestamp)
	}
	if metadata.NewestTimestamp != newestTimestamp {
		t.Errorf("NewestTimestamp = %v, want %v", metadata.NewestTimestamp, newestTimestamp)
	}
}

func assertPaginationEmptyCursor(t *testing.T, metadata *CursorPaginationMetadata) {
	t.Helper()
	if metadata == nil {
		t.Fatal("metadata is nil")
	}
	if metadata.Cursor != "" {
		t.Errorf("Metadata cursor should be empty for empty results, got %v", metadata.Cursor)
	}
}
