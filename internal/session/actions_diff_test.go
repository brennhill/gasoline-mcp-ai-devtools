// actions_diff_test.go — Tests for actions-diff.go and helpers.go.
// Covers: diffErrors, countPerfRegressions, hasStatusRegression, computeSummary,
// validateName, removeFromOrder, ExtractURLPath.
package session

import (
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/performance"
)

// ============================================
// diffErrors
// ============================================

func TestDiffErrors_NewErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	snapA := &NamedSnapshot{ConsoleErrors: []SnapshotError{}}
	snapB := &NamedSnapshot{ConsoleErrors: []SnapshotError{
		{Type: "error", Message: "TypeError: x is null", Count: 2},
		{Type: "error", Message: "RangeError: invalid index", Count: 1},
	}}

	diff := sm.diffErrors(snapA, snapB)

	if len(diff.New) != 2 {
		t.Fatalf("Expected 2 new errors, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Errorf("Expected 0 resolved errors, got %d", len(diff.Resolved))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged errors, got %d", len(diff.Unchanged))
	}
}

func TestDiffErrors_ResolvedErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	snapA := &NamedSnapshot{ConsoleErrors: []SnapshotError{
		{Type: "error", Message: "old error 1", Count: 1},
		{Type: "error", Message: "old error 2", Count: 3},
	}}
	snapB := &NamedSnapshot{ConsoleErrors: []SnapshotError{}}

	diff := sm.diffErrors(snapA, snapB)

	if len(diff.Resolved) != 2 {
		t.Fatalf("Expected 2 resolved errors, got %d", len(diff.Resolved))
	}
	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new errors, got %d", len(diff.New))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged errors, got %d", len(diff.Unchanged))
	}
}

func TestDiffErrors_UnchangedErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	errors := []SnapshotError{
		{Type: "error", Message: "persistent error", Count: 5},
	}
	snapA := &NamedSnapshot{ConsoleErrors: errors}
	snapB := &NamedSnapshot{ConsoleErrors: errors}

	diff := sm.diffErrors(snapA, snapB)

	if len(diff.Unchanged) != 1 {
		t.Fatalf("Expected 1 unchanged error, got %d", len(diff.Unchanged))
	}
	if diff.Unchanged[0].Message != "persistent error" {
		t.Errorf("Expected 'persistent error', got %q", diff.Unchanged[0].Message)
	}
	if diff.Unchanged[0].Count != 5 {
		t.Errorf("Expected count=5, got %d", diff.Unchanged[0].Count)
	}
	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new errors, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Errorf("Expected 0 resolved errors, got %d", len(diff.Resolved))
	}
}

func TestDiffErrors_MixedChanges(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	snapA := &NamedSnapshot{ConsoleErrors: []SnapshotError{
		{Type: "error", Message: "stays", Count: 1},
		{Type: "error", Message: "goes away", Count: 1},
	}}
	snapB := &NamedSnapshot{ConsoleErrors: []SnapshotError{
		{Type: "error", Message: "stays", Count: 1},
		{Type: "error", Message: "brand new", Count: 1},
	}}

	diff := sm.diffErrors(snapA, snapB)

	if len(diff.New) != 1 {
		t.Errorf("Expected 1 new error, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 1 {
		t.Errorf("Expected 1 resolved error, got %d", len(diff.Resolved))
	}
	if len(diff.Unchanged) != 1 {
		t.Errorf("Expected 1 unchanged error, got %d", len(diff.Unchanged))
	}

	// Verify exact messages
	if diff.New[0].Message != "brand new" {
		t.Errorf("Expected new error 'brand new', got %q", diff.New[0].Message)
	}
	if diff.Resolved[0].Message != "goes away" {
		t.Errorf("Expected resolved error 'goes away', got %q", diff.Resolved[0].Message)
	}
	if diff.Unchanged[0].Message != "stays" {
		t.Errorf("Expected unchanged error 'stays', got %q", diff.Unchanged[0].Message)
	}
}

func TestDiffErrors_BothEmpty(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	snapA := &NamedSnapshot{ConsoleErrors: []SnapshotError{}}
	snapB := &NamedSnapshot{ConsoleErrors: []SnapshotError{}}

	diff := sm.diffErrors(snapA, snapB)

	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Errorf("Expected 0 resolved, got %d", len(diff.Resolved))
	}
	if len(diff.Unchanged) != 0 {
		t.Errorf("Expected 0 unchanged, got %d", len(diff.Unchanged))
	}
}

func TestDiffErrors_NilConsoleErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	snapA := &NamedSnapshot{ConsoleErrors: nil}
	snapB := &NamedSnapshot{ConsoleErrors: nil}

	diff := sm.diffErrors(snapA, snapB)

	if len(diff.New) != 0 {
		t.Errorf("Expected 0 new for nil, got %d", len(diff.New))
	}
	if len(diff.Resolved) != 0 {
		t.Errorf("Expected 0 resolved for nil, got %d", len(diff.Resolved))
	}
}

func TestDiffErrors_DuplicateMessagesDeduped(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	// Two entries with same message in A - map deduplication means last wins
	snapA := &NamedSnapshot{ConsoleErrors: []SnapshotError{
		{Type: "error", Message: "duplicate", Count: 1},
		{Type: "error", Message: "duplicate", Count: 5},
	}}
	snapB := &NamedSnapshot{ConsoleErrors: []SnapshotError{}}

	diff := sm.diffErrors(snapA, snapB)

	// Should be 1 resolved (deduped by message)
	if len(diff.Resolved) != 1 {
		t.Errorf("Expected 1 resolved (deduped), got %d", len(diff.Resolved))
	}
}

// ============================================
// countPerfRegressions
// ============================================

func TestCountPerfRegressions_NoRegressions(t *testing.T) {
	t.Parallel()
	diff := PerformanceDiff{
		LoadTime:     &MetricChange{Regression: false},
		RequestCount: &MetricChange{Regression: false},
		TransferSize: &MetricChange{Regression: false},
	}
	if count := countPerfRegressions(diff); count != 0 {
		t.Errorf("Expected 0 regressions, got %d", count)
	}
}

func TestCountPerfRegressions_AllRegressions(t *testing.T) {
	t.Parallel()
	diff := PerformanceDiff{
		LoadTime:     &MetricChange{Regression: true},
		RequestCount: &MetricChange{Regression: true},
		TransferSize: &MetricChange{Regression: true},
	}
	if count := countPerfRegressions(diff); count != 3 {
		t.Errorf("Expected 3 regressions, got %d", count)
	}
}

func TestCountPerfRegressions_NilMetrics(t *testing.T) {
	t.Parallel()
	diff := PerformanceDiff{
		LoadTime:     nil,
		RequestCount: nil,
		TransferSize: nil,
	}
	if count := countPerfRegressions(diff); count != 0 {
		t.Errorf("Expected 0 regressions for nil metrics, got %d", count)
	}
}

func TestCountPerfRegressions_PartialNilMixed(t *testing.T) {
	t.Parallel()
	diff := PerformanceDiff{
		LoadTime:     &MetricChange{Regression: true},
		RequestCount: nil,
		TransferSize: &MetricChange{Regression: false},
	}
	if count := countPerfRegressions(diff); count != 1 {
		t.Errorf("Expected 1 regression, got %d", count)
	}
}

// ============================================
// hasStatusRegression
// ============================================

func TestHasStatusRegression_OKToError(t *testing.T) {
	t.Parallel()
	changes := []SessionNetworkChange{
		{BeforeStatus: 200, AfterStatus: 500},
	}
	if !hasStatusRegression(changes) {
		t.Error("Expected true for 200->500")
	}
}

func TestHasStatusRegression_ErrorToOK(t *testing.T) {
	t.Parallel()
	changes := []SessionNetworkChange{
		{BeforeStatus: 500, AfterStatus: 200},
	}
	if hasStatusRegression(changes) {
		t.Error("Expected false for 500->200 (improvement)")
	}
}

func TestHasStatusRegression_ErrorToError(t *testing.T) {
	t.Parallel()
	changes := []SessionNetworkChange{
		{BeforeStatus: 400, AfterStatus: 500},
	}
	if hasStatusRegression(changes) {
		t.Error("Expected false for 400->500 (was already erroring)")
	}
}

func TestHasStatusRegression_EmptyChanges(t *testing.T) {
	t.Parallel()
	if hasStatusRegression(nil) {
		t.Error("Expected false for nil changes")
	}
	if hasStatusRegression([]SessionNetworkChange{}) {
		t.Error("Expected false for empty changes")
	}
}

func TestHasStatusRegression_MultipleChanges(t *testing.T) {
	t.Parallel()
	changes := []SessionNetworkChange{
		{BeforeStatus: 500, AfterStatus: 200}, // improvement
		{BeforeStatus: 200, AfterStatus: 404}, // regression
	}
	if !hasStatusRegression(changes) {
		t.Error("Expected true when at least one change is regression")
	}
}

func TestHasStatusRegression_BoundaryValues(t *testing.T) {
	t.Parallel()
	// 399 -> 400: OK to error
	if !hasStatusRegression([]SessionNetworkChange{{BeforeStatus: 399, AfterStatus: 400}}) {
		t.Error("Expected true for 399->400")
	}
	// 400 -> 399: error to OK, not a regression in the "OK to error" sense
	if hasStatusRegression([]SessionNetworkChange{{BeforeStatus: 400, AfterStatus: 399}}) {
		t.Error("Expected false for 400->399 (was already erroring, now OK)")
	}
}

// ============================================
// computeSummary
// ============================================

func TestComputeSummary_Unchanged(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result := &SessionDiffResult{
		Errors:  ErrorDiff{New: []SnapshotError{}, Resolved: []SnapshotError{}, Unchanged: []SnapshotError{{Message: "still here"}}},
		Network: SessionNetworkDiff{NewErrors: []SnapshotNetworkRequest{}, StatusChanges: []SessionNetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := sm.computeSummary(result)

	if summary.Verdict != "unchanged" {
		t.Errorf("Expected 'unchanged', got %q", summary.Verdict)
	}
	if summary.NewErrors != 0 {
		t.Errorf("Expected new_errors=0, got %d", summary.NewErrors)
	}
	if summary.ResolvedErrors != 0 {
		t.Errorf("Expected resolved_errors=0, got %d", summary.ResolvedErrors)
	}
}

func TestComputeSummary_Improved(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result := &SessionDiffResult{
		Errors:  ErrorDiff{New: []SnapshotError{}, Resolved: []SnapshotError{{Message: "fixed"}}, Unchanged: []SnapshotError{}},
		Network: SessionNetworkDiff{NewErrors: []SnapshotNetworkRequest{}, StatusChanges: []SessionNetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := sm.computeSummary(result)
	if summary.Verdict != "improved" {
		t.Errorf("Expected 'improved', got %q", summary.Verdict)
	}
	if summary.ResolvedErrors != 1 {
		t.Errorf("Expected resolved_errors=1, got %d", summary.ResolvedErrors)
	}
}

func TestComputeSummary_Regressed_NewErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result := &SessionDiffResult{
		Errors:  ErrorDiff{New: []SnapshotError{{Message: "new err"}}, Resolved: []SnapshotError{}, Unchanged: []SnapshotError{}},
		Network: SessionNetworkDiff{NewErrors: []SnapshotNetworkRequest{}, StatusChanges: []SessionNetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := sm.computeSummary(result)
	if summary.Verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", summary.Verdict)
	}
}

func TestComputeSummary_Regressed_NetworkErrors(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result := &SessionDiffResult{
		Errors:  ErrorDiff{New: []SnapshotError{}, Resolved: []SnapshotError{}, Unchanged: []SnapshotError{}},
		Network: SessionNetworkDiff{NewErrors: []SnapshotNetworkRequest{{Status: 500}}, StatusChanges: []SessionNetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := sm.computeSummary(result)
	if summary.Verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", summary.Verdict)
	}
	if summary.NewNetworkErrors != 1 {
		t.Errorf("Expected new_network_errors=1, got %d", summary.NewNetworkErrors)
	}
}

func TestComputeSummary_Regressed_PerfRegressions(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result := &SessionDiffResult{
		Errors:  ErrorDiff{New: []SnapshotError{}, Resolved: []SnapshotError{}, Unchanged: []SnapshotError{}},
		Network: SessionNetworkDiff{NewErrors: []SnapshotNetworkRequest{}, StatusChanges: []SessionNetworkChange{}},
		Performance: PerformanceDiff{LoadTime: &MetricChange{Regression: true}},
	}

	summary := sm.computeSummary(result)
	if summary.Verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", summary.Verdict)
	}
	if summary.PerformanceRegressions != 1 {
		t.Errorf("Expected performance_regressions=1, got %d", summary.PerformanceRegressions)
	}
}

func TestComputeSummary_Regressed_StatusRegression(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result := &SessionDiffResult{
		Errors: ErrorDiff{New: []SnapshotError{}, Resolved: []SnapshotError{}, Unchanged: []SnapshotError{}},
		Network: SessionNetworkDiff{
			NewErrors:     []SnapshotNetworkRequest{},
			StatusChanges: []SessionNetworkChange{{BeforeStatus: 200, AfterStatus: 500}},
		},
		Performance: PerformanceDiff{},
	}

	summary := sm.computeSummary(result)
	if summary.Verdict != "regressed" {
		t.Errorf("Expected 'regressed', got %q", summary.Verdict)
	}
}

func TestComputeSummary_Mixed(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	result := &SessionDiffResult{
		Errors: ErrorDiff{
			New:      []SnapshotError{{Message: "new"}},
			Resolved: []SnapshotError{{Message: "fixed"}},
		},
		Network:     SessionNetworkDiff{NewErrors: []SnapshotNetworkRequest{}, StatusChanges: []SessionNetworkChange{}},
		Performance: PerformanceDiff{},
	}

	summary := sm.computeSummary(result)
	if summary.Verdict != "mixed" {
		t.Errorf("Expected 'mixed', got %q", summary.Verdict)
	}
}

// ============================================
// validateName
// ============================================

func TestValidateName_EmptyName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	err := sm.validateName("")
	if err == nil {
		t.Fatal("Expected error for empty name")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("Error should mention empty: %v", err)
	}
}

func TestValidateName_ReservedName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	err := sm.validateName("current")
	if err == nil {
		t.Fatal("Expected error for reserved name 'current'")
	}
	if !strings.Contains(err.Error(), "reserved") {
		t.Errorf("Error should mention reserved: %v", err)
	}
}

func TestValidateName_TooLong(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	longName := strings.Repeat("a", maxSnapshotNameLen+1)
	err := sm.validateName(longName)
	if err == nil {
		t.Fatal("Expected error for name exceeding max length")
	}
	if !strings.Contains(err.Error(), "50") {
		t.Errorf("Error should mention max length: %v", err)
	}
}

func TestValidateName_ExactMaxLength(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	exactName := strings.Repeat("x", maxSnapshotNameLen)
	err := sm.validateName(exactName)
	if err != nil {
		t.Errorf("Expected no error for name at exact max length, got: %v", err)
	}
}

func TestValidateName_ValidName(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	err := sm.validateName("my-snapshot-v2")
	if err != nil {
		t.Errorf("Expected no error for valid name, got: %v", err)
	}
}

// ============================================
// ExtractURLPath (helpers.go)
// ============================================

func TestExtractURLPath_NoQueryParams(t *testing.T) {
	t.Parallel()
	result := ExtractURLPath("/api/users")
	if result != "/api/users" {
		t.Errorf("Expected '/api/users', got %q", result)
	}
}

func TestExtractURLPath_WithQueryParams(t *testing.T) {
	t.Parallel()
	result := ExtractURLPath("/api/users?page=1&limit=10")
	if result != "/api/users" {
		t.Errorf("Expected '/api/users', got %q", result)
	}
}

func TestExtractURLPath_EmptyString(t *testing.T) {
	t.Parallel()
	result := ExtractURLPath("")
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestExtractURLPath_OnlyQueryParams(t *testing.T) {
	t.Parallel()
	result := ExtractURLPath("?foo=bar")
	if result != "" {
		t.Errorf("Expected empty string, got %q", result)
	}
}

func TestExtractURLPath_FullURL(t *testing.T) {
	t.Parallel()
	result := ExtractURLPath("https://example.com/api/v1/data?key=value")
	if result != "https://example.com/api/v1/data" {
		t.Errorf("Expected URL without query, got %q", result)
	}
}

// ============================================
// removeFromOrder (indirect test through Delete)
// ============================================

func TestRemoveFromOrder_FirstElement(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("a", "")
	sm.Capture("b", "")
	sm.Capture("c", "")

	sm.Delete("a")
	list := sm.List()

	if len(list) != 2 {
		t.Fatalf("Expected 2, got %d", len(list))
	}
	if list[0].Name != "b" || list[1].Name != "c" {
		t.Errorf("Expected [b, c], got [%s, %s]", list[0].Name, list[1].Name)
	}
}

func TestRemoveFromOrder_LastElement(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	sm.Capture("a", "")
	sm.Capture("b", "")
	sm.Capture("c", "")

	sm.Delete("c")
	list := sm.List()

	if len(list) != 2 {
		t.Fatalf("Expected 2, got %d", len(list))
	}
	if list[0].Name != "a" || list[1].Name != "b" {
		t.Errorf("Expected [a, b], got [%s, %s]", list[0].Name, list[1].Name)
	}
}

// ============================================
// Integration: Performance with snapshot manager
// ============================================

func TestDiffPerformance_IntegrationWithCompare(t *testing.T) {
	t.Parallel()
	mock := &mockCaptureState{pageURL: "http://localhost:3000"}
	sm := NewSessionManager(10, mock)

	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 500},
		Network: performance.NetworkSummary{RequestCount: 5, TransferSize: 25000},
	}
	sm.Capture("baseline", "")

	// Make it much worse
	mock.performance = &performance.PerformanceSnapshot{
		Timing:  performance.PerformanceTiming{Load: 5000},
		Network: performance.NetworkSummary{RequestCount: 50, TransferSize: 250000},
	}
	sm.Capture("degraded", "")

	diff, err := sm.Compare("baseline", "degraded")
	if err != nil {
		t.Fatalf("Compare failed: %v", err)
	}

	if diff.Performance.LoadTime == nil {
		t.Fatal("Expected non-nil LoadTime")
	}
	if diff.Performance.LoadTime.Before != 500 {
		t.Errorf("Expected before=500, got %v", diff.Performance.LoadTime.Before)
	}
	if diff.Performance.LoadTime.After != 5000 {
		t.Errorf("Expected after=5000, got %v", diff.Performance.LoadTime.After)
	}
	if diff.Performance.LoadTime.Change != "+900%" {
		t.Errorf("Expected '+900%%', got %q", diff.Performance.LoadTime.Change)
	}
	if !diff.Performance.LoadTime.Regression {
		t.Error("Expected regression=true")
	}

	// All 3 metrics should have regressed (10x increase)
	if diff.Summary.PerformanceRegressions != 3 {
		t.Errorf("Expected 3 performance regressions, got %d", diff.Summary.PerformanceRegressions)
	}
}
