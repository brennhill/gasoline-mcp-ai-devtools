// annotation_store_named_test.go — Tests for GetNamedSessionSinceDraw and buildNamedAnnotationResult.
package main

import (
	"encoding/json"
	"testing"
	"time"
)

// ============================================
// GetNamedSessionSinceDraw
// ============================================

func TestGetNamedSessionSinceDraw_ReturnsSessionUpdatedAfterDraw(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()
	time.Sleep(2 * time.Millisecond)

	store.AppendToNamedSession("review", &AnnotationSession{
		TabID:       1,
		PageURL:     "https://example.com",
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []Annotation{{ID: "a1", Text: "fix this"}},
	})

	ns := store.GetNamedSessionSinceDraw("review")
	if ns == nil {
		t.Fatal("expected named session, got nil")
	}
	if ns.Name != "review" {
		t.Errorf("expected name 'review', got %q", ns.Name)
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(ns.Pages))
	}
	if ns.Pages[0].PageURL != "https://example.com" {
		t.Errorf("expected page URL 'https://example.com', got %q", ns.Pages[0].PageURL)
	}
}

func TestGetNamedSessionSinceDraw_ReturnsNilWhenUpdatedBeforeDraw(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.AppendToNamedSession("old-review", &AnnotationSession{
		TabID:     1,
		Timestamp: time.Now().UnixMilli(),
	})

	time.Sleep(2 * time.Millisecond)
	store.MarkDrawStarted()

	ns := store.GetNamedSessionSinceDraw("old-review")
	if ns != nil {
		t.Errorf("expected nil for session updated before draw, got %+v", ns)
	}
}

func TestGetNamedSessionSinceDraw_ReturnsNilForNonexistentSession(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	ns := store.GetNamedSessionSinceDraw("nonexistent")
	if ns != nil {
		t.Errorf("expected nil for nonexistent session, got %+v", ns)
	}
}

func TestGetNamedSessionSinceDraw_ReturnsNilForExpiredSession(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()

	// Inject an expired named session with a future UpdatedAt
	store.mu.Lock()
	store.named["expired"] = &namedSessionEntry{
		Session: &NamedAnnotationSession{
			Name:      "expired",
			Pages:     []*AnnotationSession{{TabID: 1}},
			UpdatedAt: time.Now().UnixMilli() + 10000,
		},
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	store.mu.Unlock()

	ns := store.GetNamedSessionSinceDraw("expired")
	if ns != nil {
		t.Errorf("expected nil for expired session, got %+v", ns)
	}
}

func TestGetNamedSessionSinceDraw_NoDrawStarted_AnySessionQualifies(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	// Without MarkDrawStarted, lastDrawStartedAt is 0 so any session qualifies
	store.AppendToNamedSession("any", &AnnotationSession{
		TabID:     1,
		Timestamp: time.Now().UnixMilli(),
	})

	ns := store.GetNamedSessionSinceDraw("any")
	if ns == nil {
		t.Fatal("expected session when no draw started (lastDrawStartedAt=0)")
	}
	if ns.Name != "any" {
		t.Errorf("expected name 'any', got %q", ns.Name)
	}
}

func TestGetNamedSessionSinceDraw_MultiplePages(t *testing.T) {
	t.Parallel()
	store := NewAnnotationStore(10 * time.Minute)
	defer store.Close()

	store.MarkDrawStarted()
	time.Sleep(2 * time.Millisecond)

	store.AppendToNamedSession("multi", &AnnotationSession{
		TabID:   1,
		PageURL: "https://example.com/page1",
	})
	store.AppendToNamedSession("multi", &AnnotationSession{
		TabID:   2,
		PageURL: "https://example.com/page2",
	})

	ns := store.GetNamedSessionSinceDraw("multi")
	if ns == nil {
		t.Fatal("expected named session")
	}
	if len(ns.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(ns.Pages))
	}
	if ns.Pages[0].PageURL != "https://example.com/page1" {
		t.Errorf("expected first page URL, got %q", ns.Pages[0].PageURL)
	}
	if ns.Pages[1].PageURL != "https://example.com/page2" {
		t.Errorf("expected second page URL, got %q", ns.Pages[1].PageURL)
	}
}

// ============================================
// buildNamedAnnotationResult
// ============================================

func TestBuildNamedAnnotationResult_SinglePage(t *testing.T) {
	t.Parallel()
	ns := &NamedAnnotationSession{
		Name: "test-session",
		Pages: []*AnnotationSession{
			{
				Annotations: []Annotation{
					{ID: "a1", Text: "fix button"},
					{ID: "a2", Text: "wrong color"},
				},
				ScreenshotPath: "/tmp/ss1.png",
				PageURL:        "https://example.com/login",
				TabID:          42,
			},
		},
	}

	raw := buildNamedAnnotationResult(ns)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if result["status"] != "complete" {
		t.Errorf("expected status 'complete', got %v", result["status"])
	}
	if result["session_name"] != "test-session" {
		t.Errorf("expected session_name 'test-session', got %v", result["session_name"])
	}
	if result["page_count"] != float64(1) {
		t.Errorf("expected page_count 1, got %v", result["page_count"])
	}
	if result["total_count"] != float64(2) {
		t.Errorf("expected total_count 2, got %v", result["total_count"])
	}

	pages, ok := result["pages"].([]any)
	if !ok {
		t.Fatalf("expected pages to be array, got %T", result["pages"])
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	page := pages[0].(map[string]any)
	if page["page_url"] != "https://example.com/login" {
		t.Errorf("expected page_url, got %v", page["page_url"])
	}
	if page["count"] != float64(2) {
		t.Errorf("expected count 2, got %v", page["count"])
	}
	if page["tab_id"] != float64(42) {
		t.Errorf("expected tab_id 42, got %v", page["tab_id"])
	}
	if page["screenshot"] != "/tmp/ss1.png" {
		t.Errorf("expected screenshot path, got %v", page["screenshot"])
	}
}

func TestBuildNamedAnnotationResult_MultiplePages(t *testing.T) {
	t.Parallel()
	ns := &NamedAnnotationSession{
		Name: "multi-page",
		Pages: []*AnnotationSession{
			{
				Annotations: []Annotation{{ID: "a1", Text: "page1 ann"}},
				PageURL:     "https://example.com/page1",
				TabID:       1,
			},
			{
				Annotations: []Annotation{
					{ID: "a2", Text: "page2 ann1"},
					{ID: "a3", Text: "page2 ann2"},
					{ID: "a4", Text: "page2 ann3"},
				},
				ScreenshotPath: "/tmp/page2.png",
				PageURL:        "https://example.com/page2",
				TabID:          2,
			},
		},
	}

	raw := buildNamedAnnotationResult(ns)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["total_count"] != float64(4) {
		t.Errorf("expected total_count 4 (1+3), got %v", result["total_count"])
	}
	if result["page_count"] != float64(2) {
		t.Errorf("expected page_count 2, got %v", result["page_count"])
	}

	pages := result["pages"].([]any)
	page1 := pages[0].(map[string]any)
	page2 := pages[1].(map[string]any)

	// Page 1 has no screenshot, so the key should not be present
	if _, hasScreenshot := page1["screenshot"]; hasScreenshot {
		t.Errorf("page 1 should not have screenshot key, got %v", page1["screenshot"])
	}
	if page1["count"] != float64(1) {
		t.Errorf("page 1 count should be 1, got %v", page1["count"])
	}

	// Page 2 has screenshot
	if page2["screenshot"] != "/tmp/page2.png" {
		t.Errorf("page 2 screenshot should be '/tmp/page2.png', got %v", page2["screenshot"])
	}
	if page2["count"] != float64(3) {
		t.Errorf("page 2 count should be 3, got %v", page2["count"])
	}
}

func TestBuildNamedAnnotationResult_EmptyPages(t *testing.T) {
	t.Parallel()
	ns := &NamedAnnotationSession{
		Name:  "empty-session",
		Pages: []*AnnotationSession{},
	}

	raw := buildNamedAnnotationResult(ns)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["status"] != "complete" {
		t.Errorf("expected status 'complete', got %v", result["status"])
	}
	if result["total_count"] != float64(0) {
		t.Errorf("expected total_count 0, got %v", result["total_count"])
	}
	if result["page_count"] != float64(0) {
		t.Errorf("expected page_count 0, got %v", result["page_count"])
	}
	pages := result["pages"].([]any)
	if len(pages) != 0 {
		t.Errorf("expected 0 pages, got %d", len(pages))
	}
}

func TestBuildNamedAnnotationResult_ZeroAnnotationsPage(t *testing.T) {
	t.Parallel()
	ns := &NamedAnnotationSession{
		Name: "zero-ann",
		Pages: []*AnnotationSession{
			{
				Annotations: []Annotation{},
				PageURL:     "https://example.com/empty",
				TabID:       10,
			},
		},
	}

	raw := buildNamedAnnotationResult(ns)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["total_count"] != float64(0) {
		t.Errorf("expected total_count 0, got %v", result["total_count"])
	}
	if result["page_count"] != float64(1) {
		t.Errorf("expected page_count 1, got %v", result["page_count"])
	}

	pages := result["pages"].([]any)
	page := pages[0].(map[string]any)
	if page["count"] != float64(0) {
		t.Errorf("expected page count 0, got %v", page["count"])
	}
}

func TestBuildNamedAnnotationResult_SnakeCaseFields(t *testing.T) {
	t.Parallel()
	ns := &NamedAnnotationSession{
		Name: "snake-case-check",
		Pages: []*AnnotationSession{
			{
				Annotations:    []Annotation{{ID: "a1", Text: "test"}},
				ScreenshotPath: "/tmp/test.png",
				PageURL:        "https://example.com",
				TabID:          1,
			},
		},
	}

	raw := buildNamedAnnotationResult(ns)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Verify all top-level keys are snake_case
	expectedTopKeys := []string{"status", "session_name", "pages", "page_count", "total_count"}
	for _, key := range expectedTopKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("expected top-level key %q to be present", key)
		}
	}

	// Verify page-level keys are snake_case
	pages := result["pages"].([]any)
	page := pages[0].(map[string]any)
	expectedPageKeys := []string{"page_url", "annotations", "count", "tab_id", "screenshot"}
	for _, key := range expectedPageKeys {
		if _, ok := page[key]; !ok {
			t.Errorf("expected page key %q to be present", key)
		}
	}
}

// ============================================
// buildAnnotationResult (package-level, also 0% coverage)
// ============================================

func TestBuildAnnotationResult_WithScreenshot(t *testing.T) {
	t.Parallel()
	session := &AnnotationSession{
		Annotations: []Annotation{
			{ID: "a1", Text: "fix this", CorrelationID: "d1"},
		},
		ScreenshotPath: "/tmp/draw.png",
		PageURL:        "https://example.com/page",
	}

	raw := buildAnnotationResult(session)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["status"] != "complete" {
		t.Errorf("expected status 'complete', got %v", result["status"])
	}
	if result["count"] != float64(1) {
		t.Errorf("expected count 1, got %v", result["count"])
	}
	if result["page_url"] != "https://example.com/page" {
		t.Errorf("expected page_url, got %v", result["page_url"])
	}
	if result["screenshot"] != "/tmp/draw.png" {
		t.Errorf("expected screenshot path, got %v", result["screenshot"])
	}
}

func TestBuildAnnotationResult_WithoutScreenshot(t *testing.T) {
	t.Parallel()
	session := &AnnotationSession{
		Annotations: []Annotation{
			{ID: "a1", Text: "ann1"},
			{ID: "a2", Text: "ann2"},
		},
		PageURL: "https://example.com",
	}

	raw := buildAnnotationResult(session)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["count"] != float64(2) {
		t.Errorf("expected count 2, got %v", result["count"])
	}
	// screenshot key should not be present when empty
	if _, hasScreenshot := result["screenshot"]; hasScreenshot {
		t.Error("expected no screenshot key when path is empty")
	}
}

func TestBuildAnnotationResult_EmptyAnnotations(t *testing.T) {
	t.Parallel()
	session := &AnnotationSession{
		Annotations: []Annotation{},
		PageURL:     "https://example.com",
	}

	raw := buildAnnotationResult(session)

	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result["count"] != float64(0) {
		t.Errorf("expected count 0, got %v", result["count"])
	}
}
