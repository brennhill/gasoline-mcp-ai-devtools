// summary_builders_test.go — Tests for compact summary builders.
package observe

import (
	"testing"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/pagination"
)

func TestBuildErrorsSummary_CountsBySource(t *testing.T) {
	t.Parallel()
	errors := []map[string]any{
		{"message": "err1", "source": "console", "timestamp": "2024-01-01T00:00:00Z"},
		{"message": "err2", "source": "console", "timestamp": "2024-01-01T00:00:01Z"},
		{"message": "err3", "source": "network", "timestamp": "2024-01-01T00:00:02Z"},
	}
	meta := ResponseMetadata{RetrievedAt: "2024-01-01T00:00:03Z", DataAge: "1.0s"}
	result := buildErrorsSummary(errors, 0, meta)

	total, _ := result["total"].(int)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	bySource, ok := result["by_source"].(map[string]int)
	if !ok {
		t.Fatal("by_source not a map[string]int")
	}
	if bySource["console"] != 2 {
		t.Errorf("console count = %d, want 2", bySource["console"])
	}
	if bySource["network"] != 1 {
		t.Errorf("network count = %d, want 1", bySource["network"])
	}
}

func TestBuildErrorsSummary_TopMessages(t *testing.T) {
	t.Parallel()
	errors := make([]map[string]any, 0)
	for i := 0; i < 10; i++ {
		errors = append(errors, map[string]any{"message": "repeated error", "source": "js"})
	}
	for i := 0; i < 3; i++ {
		errors = append(errors, map[string]any{"message": "less common", "source": "js"})
	}
	errors = append(errors, map[string]any{"message": "rare error", "source": "js"})

	result := buildErrorsSummary(errors, 0, ResponseMetadata{})
	topMessages, ok := result["top_messages"].([]map[string]any)
	if !ok {
		t.Fatal("top_messages not a []map[string]any")
	}
	if len(topMessages) == 0 {
		t.Fatal("top_messages is empty")
	}
	// First should be the most frequent
	if topMessages[0]["message"] != "repeated error" {
		t.Errorf("first top message = %v, want 'repeated error'", topMessages[0]["message"])
	}
	if topMessages[0]["count"] != 10 {
		t.Errorf("first count = %v, want 10", topMessages[0]["count"])
	}
	// Should be capped at 5
	if len(topMessages) > 5 {
		t.Errorf("top_messages len = %d, want <= 5", len(topMessages))
	}
}

func TestBuildErrorsSummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildErrorsSummary(nil, 0, ResponseMetadata{})
	total, _ := result["total"].(int)
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

func TestBuildLogsSummary_CountsByLevel(t *testing.T) {
	t.Parallel()
	logs := []map[string]any{
		{"level": "info", "message": "a"},
		{"level": "info", "message": "b"},
		{"level": "warn", "message": "c"},
		{"level": "error", "message": "d"},
	}
	result := buildLogsSummary(logs, map[string]any{})

	total, _ := result["total"].(int)
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}

	byLevel, ok := result["by_level"].(map[string]int)
	if !ok {
		t.Fatal("by_level not a map[string]int")
	}
	if byLevel["info"] != 2 {
		t.Errorf("info count = %d, want 2", byLevel["info"])
	}
	if byLevel["warn"] != 1 {
		t.Errorf("warn count = %d, want 1", byLevel["warn"])
	}
	if byLevel["error"] != 1 {
		t.Errorf("error count = %d, want 1", byLevel["error"])
	}
}

func TestBuildLogsSummary_CountsBySource(t *testing.T) {
	t.Parallel()
	logs := []map[string]any{
		{"level": "info", "source": "console"},
		{"level": "info", "source": "console"},
		{"level": "warn", "source": "network"},
	}
	result := buildLogsSummary(logs, map[string]any{})

	bySource, ok := result["by_source"].(map[string]int)
	if !ok {
		t.Fatal("by_source not a map[string]int")
	}
	if bySource["console"] != 2 {
		t.Errorf("console count = %d, want 2", bySource["console"])
	}
}

func TestBuildNetworkBodiesSummary_StatusGrouping(t *testing.T) {
	t.Parallel()
	bodies := []capture.NetworkBody{
		{URL: "http://a.com/api", Method: "GET", Status: 200},
		{URL: "http://a.com/api2", Method: "GET", Status: 201},
		{URL: "http://a.com/api3", Method: "POST", Status: 404},
		{URL: "http://a.com/api4", Method: "GET", Status: 500},
	}
	result := buildNetworkBodiesSummary(bodies, ResponseMetadata{})

	byStatus, ok := result["by_status_group"].(map[string]int)
	if !ok {
		t.Fatal("by_status_group not a map[string]int")
	}
	if byStatus["2xx"] != 2 {
		t.Errorf("2xx count = %d, want 2", byStatus["2xx"])
	}
	if byStatus["4xx"] != 1 {
		t.Errorf("4xx count = %d, want 1", byStatus["4xx"])
	}
	if byStatus["5xx"] != 1 {
		t.Errorf("5xx count = %d, want 1", byStatus["5xx"])
	}
}

func TestBuildNetworkBodiesSummary_RecentURLs(t *testing.T) {
	t.Parallel()
	longURL := "http://example.com/" + string(make([]byte, 100))
	bodies := []capture.NetworkBody{
		{URL: longURL, Method: "GET", Status: 200},
		{URL: "http://short.com", Method: "GET", Status: 200},
	}
	result := buildNetworkBodiesSummary(bodies, ResponseMetadata{})

	recentURLs, ok := result["recent_urls"].([]string)
	if !ok {
		t.Fatal("recent_urls not a []string")
	}
	if len(recentURLs) != 2 {
		t.Fatalf("recent_urls len = %d, want 2", len(recentURLs))
	}
	// Long URL should be truncated to 80 runes + "..."
	if len([]rune(recentURLs[0])) > 84 {
		t.Errorf("long URL not truncated: rune len=%d", len([]rune(recentURLs[0])))
	}
}

func TestBuildErrorBundlesSummary_Counts(t *testing.T) {
	t.Parallel()
	bundles := []map[string]any{
		{"error": map[string]any{"message": "err1"}},
		{"error": map[string]any{"message": "err2"}},
		{"error": map[string]any{"message": "err1"}},
	}
	meta := ResponseMetadata{RetrievedAt: "2024-01-01T00:00:00Z"}
	result := buildErrorBundlesSummary(bundles, time.Now(), meta)

	total, _ := result["total_bundles"].(int)
	if total != 3 {
		t.Errorf("total_bundles = %d, want 3", total)
	}

	messages, ok := result["unique_error_messages"].([]string)
	if !ok {
		t.Fatal("unique_error_messages not a []string")
	}
	if len(messages) != 2 {
		t.Errorf("unique_error_messages len = %d, want 2", len(messages))
	}

	// Verify metadata is included
	if _, ok := result["metadata"]; !ok {
		t.Error("expected metadata key in error bundles summary")
	}
}

func TestBuildWSEventsSummary_ByDirection(t *testing.T) {
	t.Parallel()
	events := []capture.WebSocketEvent{
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "outgoing", ID: "conn1", Event: "message"},
	}
	result := buildWSEventsSummary(events, ResponseMetadata{})

	byDir, ok := result["by_direction"].(map[string]int)
	if !ok {
		t.Fatal("by_direction not a map[string]int")
	}
	if byDir["incoming"] != 2 {
		t.Errorf("incoming = %d, want 2", byDir["incoming"])
	}
	if byDir["outgoing"] != 1 {
		t.Errorf("outgoing = %d, want 1", byDir["outgoing"])
	}
}

func TestBuildWSEventsSummary_UniqueConnections(t *testing.T) {
	t.Parallel()
	events := []capture.WebSocketEvent{
		{Direction: "incoming", ID: "conn1", Event: "message"},
		{Direction: "incoming", ID: "conn2", Event: "message"},
		{Direction: "incoming", ID: "conn1", Event: "message"},
	}
	result := buildWSEventsSummary(events, ResponseMetadata{})

	connCount, _ := result["connection_count"].(int)
	if connCount != 2 {
		t.Errorf("connection_count = %d, want 2", connCount)
	}
}

func TestBuildActionsSummary_ByType(t *testing.T) {
	t.Parallel()
	now := time.Now().UnixMilli()
	actions := []capture.EnhancedAction{
		{Type: "click", Timestamp: now},
		{Type: "click", Timestamp: now + 1000},
		{Type: "type", Timestamp: now + 2000},
		{Type: "navigate", Timestamp: now + 3000},
	}
	result := buildActionsSummary(actions, ResponseMetadata{})

	total, _ := result["total"].(int)
	if total != 4 {
		t.Errorf("total = %d, want 4", total)
	}

	byType, ok := result["by_type"].(map[string]int)
	if !ok {
		t.Fatal("by_type not a map[string]int")
	}
	if byType["click"] != 2 {
		t.Errorf("click = %d, want 2", byType["click"])
	}
	if byType["type"] != 1 {
		t.Errorf("type = %d, want 1", byType["type"])
	}
}

func TestBuildActionsSummary_TimeRange(t *testing.T) {
	t.Parallel()
	t1 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC).UnixMilli()
	t2 := time.Date(2024, 1, 1, 10, 5, 0, 0, time.UTC).UnixMilli()
	actions := []capture.EnhancedAction{
		{Type: "click", Timestamp: t1},
		{Type: "click", Timestamp: t2},
	}
	result := buildActionsSummary(actions, ResponseMetadata{})

	timeRange, ok := result["time_range"].(map[string]string)
	if !ok {
		t.Fatal("time_range not a map[string]string")
	}
	if timeRange["first"] == "" || timeRange["last"] == "" {
		t.Error("expected first and last timestamps in time_range")
	}
}

func TestQuickLogsSummary_ByLevel(t *testing.T) {
	t.Parallel()
	logs := []map[string]any{
		{"level": "info"},
		{"level": "info"},
		{"level": "error"},
	}
	result := quickLogsSummary(logs)

	total, _ := result["total"].(int)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}

	byLevel, ok := result["by_level"].(map[string]int)
	if !ok {
		t.Fatal("by_level not a map[string]int")
	}
	if byLevel["info"] != 2 {
		t.Errorf("info = %d, want 2", byLevel["info"])
	}
	if byLevel["error"] != 1 {
		t.Errorf("error = %d, want 1", byLevel["error"])
	}
}

func TestBuildLogsSummary_EmptySourceBucketed(t *testing.T) {
	t.Parallel()
	logs := []map[string]any{
		{"level": "info", "source": ""},
		{"level": "info", "source": "console"},
	}
	result := buildLogsSummary(logs, map[string]any{})

	bySource, ok := result["by_source"].(map[string]int)
	if !ok {
		t.Fatal("by_source not a map[string]int")
	}
	if bySource["unknown"] != 1 {
		t.Errorf("unknown count = %d, want 1", bySource["unknown"])
	}
	if bySource["console"] != 1 {
		t.Errorf("console count = %d, want 1", bySource["console"])
	}
}

func TestTruncateRunes_UTF8Safe(t *testing.T) {
	t.Parallel()
	// 4-byte emoji chars: each is 1 rune but 4 bytes
	input := "Hello \U0001F600\U0001F600\U0001F600 world"
	result := truncateRunes(input, 9)
	runes := []rune(result)
	if len(runes) != 9 {
		t.Errorf("rune len = %d, want 9", len(runes))
	}
	// Should not have corrupted byte sequences
	for i, r := range runes {
		if r == '\uFFFD' {
			t.Errorf("replacement char at rune %d — truncation corrupted UTF-8", i)
		}
	}
}

func TestBuildActionsSummary_EpochTimestamp(t *testing.T) {
	t.Parallel()
	// Timestamp 0 = Unix epoch. Should still produce time_range.
	actions := []capture.EnhancedAction{
		{Type: "click", Timestamp: 0},
		{Type: "click", Timestamp: 1000},
	}
	result := buildActionsSummary(actions, ResponseMetadata{})
	if _, ok := result["time_range"]; !ok {
		t.Error("expected time_range even with epoch timestamp 0")
	}
}

func TestBuildHistorySummary_Counts(t *testing.T) {
	t.Parallel()
	entries := []historyEntry{
		{Timestamp: "2024-01-01T10:00:00Z", ToURL: "http://a.com", Type: "navigate"},
		{Timestamp: "2024-01-01T10:01:00Z", ToURL: "http://b.com", Type: "navigate"},
		{Timestamp: "2024-01-01T10:02:00Z", ToURL: "http://b.com/page", Type: "page_visit"},
	}
	result := buildHistorySummary(entries, ResponseMetadata{RetrievedAt: "2024-01-01T10:03:00Z"})

	total, _ := result["total"].(int)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	byType, ok := result["by_type"].(map[string]int)
	if !ok {
		t.Fatal("by_type not a map[string]int")
	}
	if byType["navigate"] != 2 {
		t.Errorf("navigate = %d, want 2", byType["navigate"])
	}
	if byType["page_visit"] != 1 {
		t.Errorf("page_visit = %d, want 1", byType["page_visit"])
	}
	uniqueURLs, ok := result["unique_urls"].(int)
	if !ok {
		t.Fatal("unique_urls not an int")
	}
	if uniqueURLs != 3 {
		t.Errorf("unique_urls = %d, want 3", uniqueURLs)
	}
}

func TestBuildHistorySummary_Empty(t *testing.T) {
	t.Parallel()
	result := buildHistorySummary(nil, ResponseMetadata{})
	total, _ := result["total"].(int)
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}

func TestBuildPaginatedMetadataWithSummary_FirstPage(t *testing.T) {
	t.Parallel()
	pMeta := &pagination.CursorPaginationMetadata{
		Total:   50,
		HasMore: true,
		Cursor:  "cursor123",
	}
	summaryFn := func() map[string]any {
		return map[string]any{"total": 50, "by_level": map[string]int{"info": 30, "error": 20}}
	}

	// isFirstPage=true
	result := BuildPaginatedMetadataWithSummary(nil, time.Time{}, pMeta, true, summaryFn)

	summary, ok := result["summary"]
	if !ok {
		t.Fatal("expected summary key on first page")
	}
	summaryMap, ok := summary.(map[string]any)
	if !ok {
		t.Fatal("summary not a map[string]any")
	}
	if summaryMap["total"] != 50 {
		t.Errorf("summary total = %v, want 50", summaryMap["total"])
	}
}

func TestBuildPaginatedMetadataWithSummary_SubsequentPage(t *testing.T) {
	t.Parallel()
	pMeta := &pagination.CursorPaginationMetadata{
		Total:   50,
		HasMore: true,
		Cursor:  "cursor456",
	}
	called := false
	summaryFn := func() map[string]any {
		called = true
		return map[string]any{"total": 50}
	}

	// isFirstPage=false
	result := BuildPaginatedMetadataWithSummary(nil, time.Time{}, pMeta, false, summaryFn)

	if _, ok := result["summary"]; ok {
		t.Error("expected no summary key on subsequent page")
	}
	if called {
		t.Error("summaryFn should not be called for subsequent pages")
	}
}
