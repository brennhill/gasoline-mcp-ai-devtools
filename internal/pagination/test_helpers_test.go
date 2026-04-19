// Purpose: Shared test assertions/helpers for pagination test suites.
// Docs: docs/features/feature/pagination/index.md

package pagination

import "testing"

type paginationAfterCursorCase struct {
	name             string
	afterCursor      string
	limit            int
	expectedCount    int
	expectedFirstSeq int64
	expectedLastSeq  int64
	expectedHasMore  bool
}

type paginationBeforeCursorCase struct {
	name             string
	beforeCursor     string
	limit            int
	expectedCount    int
	expectedFirstSeq int64
	expectedLastSeq  int64
}

func runAfterCursorPaginationCases[T any](
	t *testing.T,
	total int,
	cases []paginationAfterCursorCase,
	paginate func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]T, *CursorPaginationMetadata, error),
	sequenceFor func(T) int64,
	timestampFor func(T) string,
) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := paginate(tt.afterCursor, "", tt.limit, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			assertDirectionalCursorResult(
				t,
				total,
				tt.expectedCount,
				tt.expectedFirstSeq,
				tt.expectedLastSeq,
				result,
				metadata,
				sequenceFor,
				timestampFor,
			)
			if metadata.HasMore != tt.expectedHasMore {
				t.Errorf("HasMore = %v, want %v", metadata.HasMore, tt.expectedHasMore)
			}
		})
	}
}

func runBeforeCursorPaginationCases[T any](
	t *testing.T,
	total int,
	cases []paginationBeforeCursorCase,
	paginate func(afterCursor, beforeCursor string, limit int, restartOnEviction bool) ([]T, *CursorPaginationMetadata, error),
	sequenceFor func(T) int64,
	timestampFor func(T) string,
) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, metadata, err := paginate("", tt.beforeCursor, tt.limit, false)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			assertDirectionalCursorResult(
				t,
				total,
				tt.expectedCount,
				tt.expectedFirstSeq,
				tt.expectedLastSeq,
				result,
				metadata,
				sequenceFor,
				timestampFor,
			)
		})
	}
}

func assertDirectionalCursorResult[T any](
	t *testing.T,
	total int,
	expectedCount int,
	expectedFirstSeq int64,
	expectedLastSeq int64,
	result []T,
	metadata *CursorPaginationMetadata,
	sequenceFor func(T) int64,
	timestampFor func(T) string,
) {
	t.Helper()
	assertPaginationCountAndTotal(t, len(result), expectedCount, metadata, total)
	if expectedCount == 0 {
		assertPaginationEmptyCursor(t, metadata)
		return
	}

	if sequenceFor(result[0]) != expectedFirstSeq {
		t.Errorf("First sequence = %d, want %d", sequenceFor(result[0]), expectedFirstSeq)
	}
	if sequenceFor(result[len(result)-1]) != expectedLastSeq {
		t.Errorf("Last sequence = %d, want %d", sequenceFor(result[len(result)-1]), expectedLastSeq)
	}
	assertPaginationCursorFields(
		t,
		metadata,
		timestampFor(result[0]),
		timestampFor(result[len(result)-1]),
		sequenceFor(result[len(result)-1]),
	)
}

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
