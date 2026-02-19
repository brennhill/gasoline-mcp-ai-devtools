// annotation_store_test.go — Tests for annotation HTTP route helpers.
// Pure store tests live in internal/annotation/store_test.go.
package main

import (
	"encoding/json"
	"testing"
	"time"
)

// --- parseAnnotations edge case tests ---

func TestParseAnnotations_EmptyText(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"a1","text":"","rect":{"x":10,"y":20,"width":100,"height":50}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty text, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].Text != "" {
		t.Errorf("expected empty text, got %q", annotations[0].Text)
	}
	if annotations[0].ID != "a1" {
		t.Errorf("expected id 'a1', got %q", annotations[0].ID)
	}
}

func TestParseAnnotations_EmptyID(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"","text":"some text","rect":{"x":0,"y":0,"width":50,"height":50}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty ID, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].ID != "" {
		t.Errorf("expected empty ID, got %q", annotations[0].ID)
	}
}

func TestParseAnnotations_ZeroRect(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"a1","text":"zero rect","rect":{"x":0,"y":0,"width":0,"height":0}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for zero rect, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].Rect.Width != 0 || annotations[0].Rect.Height != 0 {
		t.Errorf("expected zero width/height, got %+v", annotations[0].Rect)
	}
}

func TestParseAnnotations_MixedValidAndInvalid(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"good","text":"valid annotation","rect":{"x":10,"y":20,"width":30,"height":40}}`),
		json.RawMessage(`not-valid-json`),
		json.RawMessage(`{"id":"also-good","text":"another valid","rect":{"x":0,"y":0,"width":10,"height":10}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(annotations) != 2 {
		t.Errorf("expected 2 valid annotations, got %d", len(annotations))
	}
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestParseAnnotations_AllInvalid(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`invalid-1`),
		json.RawMessage(`invalid-2`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(annotations))
	}
	if len(warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(warnings))
	}
}

func TestParseAnnotations_EmptyInput(t *testing.T) {
	annotations, warnings := parseAnnotations(nil)
	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations for nil input, got %d", len(annotations))
	}
	if warnings != nil {
		t.Errorf("expected nil warnings for nil input, got %v", warnings)
	}
}

func TestParseAnnotations_EmptySlice(t *testing.T) {
	annotations, warnings := parseAnnotations([]json.RawMessage{})
	if len(annotations) != 0 {
		t.Errorf("expected 0 annotations for empty slice, got %d", len(annotations))
	}
	if warnings != nil {
		t.Errorf("expected nil warnings for empty slice, got %v", warnings)
	}
}

func TestParseAnnotations_FullAnnotationFields(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{
			"id": "ann_full",
			"text": "make this darker",
			"element_summary": "button.primary 'Submit'",
			"correlation_id": "detail_full",
			"rect": {"x": 100, "y": 200, "width": 150, "height": 50},
			"page_url": "https://example.com",
			"timestamp": 1700000000000
		}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}

	ann := annotations[0]
	if ann.ID != "ann_full" {
		t.Errorf("expected ID 'ann_full', got %q", ann.ID)
	}
	if ann.Text != "make this darker" {
		t.Errorf("expected text, got %q", ann.Text)
	}
	if ann.ElementSummary != "button.primary 'Submit'" {
		t.Errorf("expected element summary, got %q", ann.ElementSummary)
	}
	if ann.CorrelationID != "detail_full" {
		t.Errorf("expected correlation ID, got %q", ann.CorrelationID)
	}
	if ann.Rect.X != 100 || ann.Rect.Y != 200 || ann.Rect.Width != 150 || ann.Rect.Height != 50 {
		t.Errorf("unexpected rect: %+v", ann.Rect)
	}
	if ann.PageURL != "https://example.com" {
		t.Errorf("expected page URL, got %q", ann.PageURL)
	}
	if ann.Timestamp != 1700000000000 {
		t.Errorf("expected timestamp, got %d", ann.Timestamp)
	}
}

func TestParseAnnotations_NegativeRectValues(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"neg","text":"negative","rect":{"x":-10,"y":-20,"width":-5,"height":-5}}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for negative rect, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].Rect.X != -10 {
		t.Errorf("expected x=-10, got %f", annotations[0].Rect.X)
	}
}

func TestParseAnnotations_MinimalValidJSON(t *testing.T) {
	// An empty JSON object should parse successfully (all fields zero-valued)
	raw := []json.RawMessage{
		json.RawMessage(`{}`),
	}
	annotations, warnings := parseAnnotations(raw)

	if len(warnings) != 0 {
		t.Errorf("expected no warnings for empty object, got %v", warnings)
	}
	if len(annotations) != 1 {
		t.Fatalf("expected 1 annotation, got %d", len(annotations))
	}
	if annotations[0].ID != "" || annotations[0].Text != "" {
		t.Errorf("expected empty fields, got ID=%q Text=%q", annotations[0].ID, annotations[0].Text)
	}
}

// --- persistDrawSession edge cases ---

func TestPersistDrawSession_WithSessionName(t *testing.T) {
	body := &drawModeRequest{
		PageURL:          "https://example.com",
		TabID:            42,
		AnnotSessionName: "qa-review",
	}
	annotations := []Annotation{
		{ID: "a1", Text: "fix this"},
	}

	// persistDrawSession writes to screenshotsDir; just ensure it does not panic
	persistDrawSession(body, "/tmp/test.png", annotations)
}

func TestPersistDrawSession_WithoutSessionName(t *testing.T) {
	body := &drawModeRequest{
		PageURL: "https://example.com",
		TabID:   43,
	}
	annotations := []Annotation{
		{ID: "a1", Text: "fix that"},
	}

	// persistDrawSession writes to screenshotsDir; just ensure it does not panic
	persistDrawSession(body, "/tmp/test2.png", annotations)
}

func TestPersistDrawSession_EmptyAnnotations(t *testing.T) {
	body := &drawModeRequest{
		PageURL: "https://example.com",
		TabID:   44,
	}

	// Should not panic with empty annotations
	persistDrawSession(body, "", []Annotation{})
}

// --- storeElementDetails edge cases ---

func TestStoreElementDetails_MultipleDetails(t *testing.T) {
	// Save and restore the global store
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	details := map[string]json.RawMessage{
		"d1": json.RawMessage(`{"selector":"div.a","tag":"div","text_content":"A"}`),
		"d2": json.RawMessage(`{"selector":"span.b","tag":"span","text_content":"B"}`),
	}
	storeElementDetails(details)

	got1, found1 := globalAnnotationStore.GetDetail("d1")
	if !found1 || got1.Selector != "div.a" {
		t.Errorf("expected detail d1 with selector 'div.a', found=%v got=%+v", found1, got1)
	}
	got2, found2 := globalAnnotationStore.GetDetail("d2")
	if !found2 || got2.Selector != "span.b" {
		t.Errorf("expected detail d2 with selector 'span.b', found=%v got=%+v", found2, got2)
	}
	// CorrelationID should be set from the map key
	if got1.CorrelationID != "d1" {
		t.Errorf("expected correlationID 'd1', got %q", got1.CorrelationID)
	}
}

func TestStoreElementDetails_InvalidJSON(t *testing.T) {
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	details := map[string]json.RawMessage{
		"bad": json.RawMessage(`not-valid-json`),
	}
	// Should not panic; invalid JSON is silently ignored
	storeElementDetails(details)

	_, found := globalAnnotationStore.GetDetail("bad")
	if found {
		t.Error("expected invalid JSON detail to not be stored")
	}
}

func TestStoreElementDetails_Empty(t *testing.T) {
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	// Should not panic with empty map
	storeElementDetails(map[string]json.RawMessage{})

	// Should not panic with nil map
	storeElementDetails(nil)
}

// --- storeAnnotationSession edge cases ---

func TestStoreAnnotationSession_WithSessionName(t *testing.T) {
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	body := &drawModeRequest{
		PageURL:          "https://example.com",
		TabID:            50,
		AnnotSessionName: "named-test",
	}
	annotations := []Annotation{{ID: "a1", Text: "test"}}

	storeAnnotationSession(body, "/tmp/ss.png", annotations)

	// Should be stored in both anonymous and named
	session := globalAnnotationStore.GetSession(50)
	if session == nil {
		t.Fatal("expected session in anonymous store")
	}
	if session.ScreenshotPath != "/tmp/ss.png" {
		t.Errorf("expected screenshot path '/tmp/ss.png', got %q", session.ScreenshotPath)
	}
	if session.PageURL != "https://example.com" {
		t.Errorf("expected page URL 'https://example.com', got %q", session.PageURL)
	}
	if session.TabID != 50 {
		t.Errorf("expected tab ID 50, got %d", session.TabID)
	}
	if len(session.Annotations) != 1 || session.Annotations[0].ID != "a1" || session.Annotations[0].Text != "test" {
		t.Errorf("expected annotation {ID:a1, Text:test}, got %+v", session.Annotations)
	}

	ns := globalAnnotationStore.GetNamedSession("named-test")
	if ns == nil {
		t.Fatal("expected named session")
	}
	if ns.Name != "named-test" {
		t.Errorf("expected named session name 'named-test', got %q", ns.Name)
	}
	if len(ns.Pages) != 1 {
		t.Fatalf("expected 1 page in named session, got %d", len(ns.Pages))
	}
	if ns.Pages[0].PageURL != "https://example.com" {
		t.Errorf("expected named session page URL 'https://example.com', got %q", ns.Pages[0].PageURL)
	}
}

func TestStoreAnnotationSession_WithoutSessionName(t *testing.T) {
	oldStore := globalAnnotationStore
	globalAnnotationStore = NewAnnotationStore(10 * time.Minute)
	defer func() {
		globalAnnotationStore.Close()
		globalAnnotationStore = oldStore
	}()

	body := &drawModeRequest{
		PageURL: "https://example.com",
		TabID:   51,
	}
	annotations := []Annotation{{ID: "a1", Text: "test"}}

	storeAnnotationSession(body, "", annotations)

	session := globalAnnotationStore.GetSession(51)
	if session == nil {
		t.Fatal("expected session in anonymous store")
	}
	if session.TabID != 51 {
		t.Errorf("expected tab ID 51, got %d", session.TabID)
	}
	if session.PageURL != "https://example.com" {
		t.Errorf("expected page URL 'https://example.com', got %q", session.PageURL)
	}
	if len(session.Annotations) != 1 || session.Annotations[0].Text != "test" {
		t.Errorf("expected annotation text 'test', got %+v", session.Annotations)
	}

	// With empty session name, no named session should be created
	names := globalAnnotationStore.ListNamedSessions()
	if len(names) != 0 {
		t.Errorf("expected no named sessions, got %v", names)
	}
}
