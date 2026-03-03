// Purpose: Validate observe analysis summary and compact response functions.
// Why: Prevents silent regressions in summary/compact response features.
// Docs: docs/features/feature/observe/index.md

// analysis_test.go — Tests for waterfall summary, timeline summary, history limit, and a11y compact.
package observe

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/mcp"
)

// ============================================
// Mock Deps for RunA11yAudit tests
// ============================================

type mockA11yDeps struct {
	cap           *capture.Store
	a11yResult    json.RawMessage
	a11yErr       error
	diagnosticStr string
}

func (m *mockA11yDeps) DiagnosticHintString() string { return m.diagnosticStr }
func (m *mockA11yDeps) GetCapture() *capture.Store   { return m.cap }
func (m *mockA11yDeps) GetLogEntries() ([]mcp.LogEntry, []time.Time) {
	return nil, nil
}
func (m *mockA11yDeps) GetLogTotalAdded() int64 { return 0 }
func (m *mockA11yDeps) ExecuteA11yQuery(_ string, _ []string, _ any, _ bool) (json.RawMessage, error) {
	return m.a11yResult, m.a11yErr
}
func (m *mockA11yDeps) IsConsoleNoise(_ mcp.LogEntry) bool { return false }

// ============================================
// Waterfall Summary Tests
// ============================================

func TestWaterfallSummaryEntry_CompactFields(t *testing.T) {
	t.Parallel()
	entry := capture.NetworkWaterfallEntry{
		URL:             "https://example.com/api/data",
		InitiatorType:   "fetch",
		Duration:        123.45,
		StartTime:       100.0,
		TransferSize:    5000,
		DecodedBodySize: 10000,
		EncodedBodySize: 5000,
		Timestamp:       time.Now(),
		PageURL:         "https://example.com",
	}

	result := waterfallSummaryEntry(entry)

	// Should have exactly 3 fields: url, ms, type
	if len(result) != 3 {
		t.Errorf("expected 3 fields, got %d: %v", len(result), result)
	}
	if result["url"] != "https://example.com/api/data" {
		t.Errorf("url = %v, want https://example.com/api/data", result["url"])
	}
	if result["ms"] != 123.45 {
		t.Errorf("ms = %v, want 123.45", result["ms"])
	}
	if result["type"] != "fetch" {
		t.Errorf("type = %v, want fetch", result["type"])
	}
}

func TestWaterfallSummaryEntry_URLTruncation(t *testing.T) {
	t.Parallel()
	longURL := "https://example.com/" + string(make([]byte, 100)) // > 80 chars
	for i := range longURL {
		if i >= 20 && longURL[i] == 0 {
			// Fill with 'a' chars after the prefix
		}
	}
	// Build a URL that's definitely > 80 chars
	longURL = "https://example.com/api/v1/very/long/path/that/exceeds/eighty/characters/limit/and/keeps/going/further"

	entry := capture.NetworkWaterfallEntry{
		URL:           longURL,
		InitiatorType: "xmlhttprequest",
		Duration:      50.0,
	}

	result := waterfallSummaryEntry(entry)

	url := result["url"].(string)
	if len(url) > 83 { // 80 + "..."
		t.Errorf("URL should be truncated, len=%d: %s", len(url), url)
	}
	if url[len(url)-3:] != "..." {
		t.Errorf("truncated URL should end with ..., got: %s", url)
	}
}

func TestWaterfallSummaryEntry_ShortURL(t *testing.T) {
	t.Parallel()
	entry := capture.NetworkWaterfallEntry{
		URL:           "https://a.co/x",
		InitiatorType: "script",
		Duration:      10.0,
	}

	result := waterfallSummaryEntry(entry)
	if result["url"] != "https://a.co/x" {
		t.Errorf("short URL should not be truncated: %v", result["url"])
	}
}

func TestFilterWaterfallSummaryEntries(t *testing.T) {
	t.Parallel()
	entries := []capture.NetworkWaterfallEntry{
		{URL: "https://example.com/a", InitiatorType: "fetch", Duration: 10.0},
		{URL: "https://example.com/b", InitiatorType: "script", Duration: 20.0},
		{URL: "https://other.com/c", InitiatorType: "img", Duration: 30.0},
	}

	result := filterWaterfallSummaryEntries(entries, "", 10)
	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}
	// Each entry should have exactly 3 fields
	for i, entry := range result {
		if len(entry) != 3 {
			t.Errorf("entry %d: expected 3 fields, got %d", i, len(entry))
		}
	}
}

func TestFilterWaterfallSummaryEntries_WithFilter(t *testing.T) {
	t.Parallel()
	entries := []capture.NetworkWaterfallEntry{
		{URL: "https://example.com/a", InitiatorType: "fetch", Duration: 10.0},
		{URL: "https://other.com/b", InitiatorType: "script", Duration: 20.0},
	}

	result := filterWaterfallSummaryEntries(entries, "example", 10)
	if len(result) != 1 {
		t.Errorf("expected 1 filtered entry, got %d", len(result))
	}
}

func TestFilterWaterfallSummaryEntries_WithLimit(t *testing.T) {
	t.Parallel()
	entries := []capture.NetworkWaterfallEntry{
		{URL: "https://example.com/a", InitiatorType: "fetch", Duration: 10.0},
		{URL: "https://example.com/b", InitiatorType: "script", Duration: 20.0},
		{URL: "https://example.com/c", InitiatorType: "img", Duration: 30.0},
	}

	result := filterWaterfallSummaryEntries(entries, "", 2)
	if len(result) != 2 {
		t.Errorf("expected 2 entries with limit, got %d", len(result))
	}
}

// ============================================
// Timeline Summary Tests
// ============================================

func TestBuildTimelineSummary_CountsByType(t *testing.T) {
	t.Parallel()
	entries := []timelineEntry{
		{Timestamp: "2024-01-01T00:00:01Z", Type: "action", Summary: "click"},
		{Timestamp: "2024-01-01T00:00:02Z", Type: "action", Summary: "type"},
		{Timestamp: "2024-01-01T00:00:03Z", Type: "error", Summary: "ReferenceError"},
		{Timestamp: "2024-01-01T00:00:04Z", Type: "network", Summary: "GET /api"},
		{Timestamp: "2024-01-01T00:00:05Z", Type: "network", Summary: "POST /api"},
		{Timestamp: "2024-01-01T00:00:06Z", Type: "network", Summary: "GET /img"},
		{Timestamp: "2024-01-01T00:00:07Z", Type: "websocket", Summary: "message"},
	}

	result := buildTimelineSummary(entries)

	counts, ok := result["counts_by_type"].(map[string]int)
	if !ok {
		t.Fatalf("counts_by_type wrong type: %T", result["counts_by_type"])
	}
	if counts["action"] != 2 {
		t.Errorf("action count = %d, want 2", counts["action"])
	}
	if counts["error"] != 1 {
		t.Errorf("error count = %d, want 1", counts["error"])
	}
	if counts["network"] != 3 {
		t.Errorf("network count = %d, want 3", counts["network"])
	}
	if counts["websocket"] != 1 {
		t.Errorf("websocket count = %d, want 1", counts["websocket"])
	}

	if result["total"] != 7 {
		t.Errorf("total = %v, want 7", result["total"])
	}

	timeRange, ok := result["time_range"].(map[string]string)
	if !ok {
		t.Fatalf("time_range wrong type: %T", result["time_range"])
	}
	if timeRange["first"] != "2024-01-01T00:00:01Z" {
		t.Errorf("first = %v, want 2024-01-01T00:00:01Z", timeRange["first"])
	}
	if timeRange["last"] != "2024-01-01T00:00:07Z" {
		t.Errorf("last = %v, want 2024-01-01T00:00:07Z", timeRange["last"])
	}
}

func TestBuildTimelineSummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildTimelineSummary(nil)
	if result["total"] != 0 {
		t.Errorf("total = %v, want 0", result["total"])
	}
}

// ============================================
// History Limit Tests
// ============================================

func TestBuildHistoryEntries_Limit(t *testing.T) {
	t.Parallel()
	now := time.Now().UnixMilli()
	actions := []capture.EnhancedAction{
		{Type: "navigate", Timestamp: now - 3000, ToURL: "https://a.com"},
		{Type: "navigate", Timestamp: now - 2000, ToURL: "https://b.com"},
		{Type: "navigate", Timestamp: now - 1000, ToURL: "https://c.com"},
	}

	entries := buildHistoryEntries(actions)
	if len(entries) != 3 {
		t.Fatalf("expected 3 history entries, got %d", len(entries))
	}

	// Now test with limit
	limited := limitHistoryEntries(entries, 2)
	if len(limited) != 2 {
		t.Errorf("expected 2 entries with limit, got %d", len(limited))
	}
	// Should keep the most recent (last) entries
	if limited[0].ToURL != "https://b.com" {
		t.Errorf("first limited entry = %s, want https://b.com", limited[0].ToURL)
	}
	if limited[1].ToURL != "https://c.com" {
		t.Errorf("second limited entry = %s, want https://c.com", limited[1].ToURL)
	}
}

func TestLimitHistoryEntries_NoTruncation(t *testing.T) {
	t.Parallel()
	entries := []historyEntry{
		{ToURL: "https://a.com"},
		{ToURL: "https://b.com"},
	}
	limited := limitHistoryEntries(entries, 10)
	if len(limited) != 2 {
		t.Errorf("expected 2 entries (no truncation), got %d", len(limited))
	}
}

func TestLimitHistoryEntries_ZeroLimit(t *testing.T) {
	t.Parallel()
	entries := []historyEntry{
		{ToURL: "https://a.com"},
	}
	// Zero limit means no limit applied
	limited := limitHistoryEntries(entries, 0)
	if len(limited) != 1 {
		t.Errorf("expected 1 entry with zero limit, got %d", len(limited))
	}
}

// ============================================
// A11y Summary Tests
// ============================================

func TestBuildA11ySummary_Compact(t *testing.T) {
	t.Parallel()
	auditResult := map[string]any{
		"passes": []any{
			map[string]any{"id": "rule1"},
			map[string]any{"id": "rule2"},
		},
		"violations": []any{
			map[string]any{
				"id":     "color-contrast",
				"impact": "serious",
				"nodes":  []any{map[string]any{}, map[string]any{}, map[string]any{}},
			},
			map[string]any{
				"id":     "image-alt",
				"impact": "critical",
				"nodes":  []any{map[string]any{}},
			},
		},
		"incomplete": []any{
			map[string]any{"id": "aria-label"},
		},
	}

	result := buildA11ySummary(auditResult)

	if result["pass"] != 2 {
		t.Errorf("pass = %v, want 2", result["pass"])
	}
	if result["violations"] != 2 {
		t.Errorf("violations = %v, want 2", result["violations"])
	}
	if result["incomplete"] != 1 {
		t.Errorf("incomplete = %v, want 1", result["incomplete"])
	}

	topIssues, ok := result["top_issues"].([]map[string]any)
	if !ok {
		t.Fatalf("top_issues wrong type: %T", result["top_issues"])
	}
	if len(topIssues) != 2 {
		t.Fatalf("expected 2 top issues, got %d", len(topIssues))
	}
	// Should be sorted by node count descending
	if topIssues[0]["rule"] != "color-contrast" {
		t.Errorf("first issue = %v, want color-contrast", topIssues[0]["rule"])
	}
	if topIssues[0]["count"] != 3 {
		t.Errorf("first issue count = %v, want 3", topIssues[0]["count"])
	}
	if topIssues[0]["severity"] != "serious" {
		t.Errorf("first issue severity = %v, want serious", topIssues[0]["severity"])
	}
}

func TestBuildA11ySummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildA11ySummary(map[string]any{})
	if result["pass"] != 0 {
		t.Errorf("pass = %v, want 0", result["pass"])
	}
	if result["violations"] != 0 {
		t.Errorf("violations = %v, want 0", result["violations"])
	}
}

func TestBuildA11ySummary_TopIssuesLimitedTo5(t *testing.T) {
	t.Parallel()
	violations := make([]any, 7)
	for i := range violations {
		violations[i] = map[string]any{
			"id":     "rule-" + string(rune('a'+i)),
			"impact": "minor",
			"nodes":  []any{map[string]any{}},
		}
	}
	auditResult := map[string]any{"violations": violations}

	result := buildA11ySummary(auditResult)
	topIssues := result["top_issues"].([]map[string]any)
	if len(topIssues) != 5 {
		t.Errorf("expected 5 top issues (capped), got %d", len(topIssues))
	}
}

// ============================================
// Issue #276: A11y Audit Partial Results Tests
// ============================================

func TestRunA11yAudit_TimeoutReturnsPartialResults(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.SetTrackingStatusForTest(1, "https://example.com")

	deps := &mockA11yDeps{
		cap:     cap,
		a11yErr: errors.New("context deadline exceeded"),
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps, req, json.RawMessage(`{}`))

	// Should NOT be an error response — should return partial results gracefully
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result.IsError {
		t.Fatal("timeout should return partial results, not isError:true")
	}

	// Should contain an error field in the data
	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("partial result should contain 'error' field, got: %s", text)
	}
	if !strings.Contains(text, "timeout") && !strings.Contains(text, "deadline") {
		t.Errorf("partial result should mention timeout or deadline, got: %s", text)
	}

	// Should have empty violations/passes arrays and partial flag (partial result structure)
	var data map[string]any
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("partial result text should contain JSON object")
	}
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("failed to parse partial result JSON: %v", err)
	}
	if _, ok := data["violations"]; !ok {
		t.Error("partial result should include 'violations' field")
	}
	if _, ok := data["summary"]; !ok {
		t.Error("partial result should include 'summary' field")
	}
	summary, ok := data["summary"].(map[string]any)
	if !ok {
		t.Fatalf("partial result summary should be object, got %T", data["summary"])
	}
	if summary["violations"] != float64(0) || summary["violation_count"] != float64(0) {
		t.Errorf("expected zero violations summary aliases, got %+v", summary)
	}
	if summary["passes"] != float64(0) || summary["pass_count"] != float64(0) {
		t.Errorf("expected zero passes summary aliases, got %+v", summary)
	}
	if data["partial"] != true {
		t.Errorf("partial result should have partial=true, got: %v", data["partial"])
	}
}

func TestRunA11yAudit_AlreadyRunningReturnsPartialResults(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.SetTrackingStatusForTest(1, "https://example.com")

	deps := &mockA11yDeps{
		cap:     cap,
		a11yErr: errors.New("Axe is already running"),
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps, req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if result.IsError {
		t.Fatal("already-running should return partial results, not isError:true")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("partial result should contain error field, got: %s", text)
	}
	if !strings.Contains(text, "already running") {
		t.Errorf("partial result should mention 'already running', got: %s", text)
	}

	// Verify partial flag is set
	var data map[string]any
	idx := strings.Index(text, "{")
	if idx < 0 {
		t.Fatal("partial result text should contain JSON object")
	}
	if err := json.Unmarshal([]byte(text[idx:]), &data); err != nil {
		t.Fatalf("failed to parse partial result JSON: %v", err)
	}
	if data["partial"] != true {
		t.Errorf("partial result should have partial=true, got: %v", data["partial"])
	}
}

// ============================================
// parseDataURL Tests
// ============================================

func TestParseDataURL_ValidJPEG(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/jpeg;base64,/9j/4AAQSkZJRg==")
	if data != "/9j/4AAQSkZJRg==" {
		t.Errorf("base64Data = %q, want %q", data, "/9j/4AAQSkZJRg==")
	}
	if mime != "image/jpeg" {
		t.Errorf("mimeType = %q, want %q", mime, "image/jpeg")
	}
}

func TestParseDataURL_ValidPNG(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/png;base64,iVBORw0KGgo=")
	if data != "iVBORw0KGgo=" {
		t.Errorf("base64Data = %q, want %q", data, "iVBORw0KGgo=")
	}
	if mime != "image/png" {
		t.Errorf("mimeType = %q, want %q", mime, "image/png")
	}
}

func TestParseDataURL_MalformedNoDataPrefix(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("image/jpeg;base64,/9j/4AAQ")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for missing data: prefix, got data=%q mime=%q", data, mime)
	}
}

func TestParseDataURL_MalformedNoBase64Marker(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("data:image/jpeg;charset=utf-8,sometext")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for missing base64 marker, got data=%q mime=%q", data, mime)
	}
}

func TestParseDataURL_EmptyString(t *testing.T) {
	t.Parallel()
	data, mime := parseDataURL("")
	if data != "" || mime != "" {
		t.Errorf("expected empty strings for empty input, got data=%q mime=%q", data, mime)
	}
}

func TestRunA11yAudit_ResultWithErrorFieldReturnsGracefully(t *testing.T) {
	t.Parallel()
	cap := capture.NewCapture()
	cap.SetTrackingStatusForTest(1, "https://example.com")

	// Simulate extension returning partial results with an error field
	partialResult := map[string]any{
		"violations":   []any{},
		"passes":       []any{},
		"incomplete":   []any{},
		"inapplicable": []any{},
		"summary": map[string]any{
			"violations":   0,
			"passes":       0,
			"incomplete":   0,
			"inapplicable": 0,
		},
		"error": "Accessibility audit timeout",
	}
	resultJSON, _ := json.Marshal(partialResult)

	deps := &mockA11yDeps{
		cap:        cap,
		a11yResult: resultJSON,
	}

	req := mcp.JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
	resp := RunA11yAudit(deps, req, json.RawMessage(`{}`))

	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	// Should NOT be isError — partial results are still useful
	if result.IsError {
		t.Fatal("result with error field should not be isError:true")
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "error") {
		t.Errorf("response should preserve error field, got: %s", text)
	}
}
