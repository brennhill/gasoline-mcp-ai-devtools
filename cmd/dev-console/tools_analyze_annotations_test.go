// tools_analyze_annotations_test.go — Tests for analyze annotations handlers.
package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// unmarshalMCPText extracts the text from an MCP tool response.
func unmarshalMCPText(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var result MCPToolResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
	return result.Content[0].Text
}

func TestToolGetAnnotations_NoSession(t *testing.T) {
	h := createTestToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations"}`)

	resp := h.toolGetAnnotations(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "No annotation") {
		t.Errorf("expected no annotation message, got %q", text)
	}
}

func TestToolGetAnnotations_WithSession(t *testing.T) {
	h := createTestToolHandler(t)

	// Store a session
	session := &AnnotationSession{
		Annotations: []Annotation{
			{
				ID:             "ann_1",
				Text:           "make this darker",
				ElementSummary: "button.primary 'Submit'",
				CorrelationID:  "detail_1",
				Rect:           AnnotationRect{X: 100, Y: 200, Width: 150, Height: 50},
			},
		},
		ScreenshotPath: "/tmp/test.png",
		PageURL:        "https://example.com",
		TabID:          1,
		Timestamp:      time.Now().UnixMilli(),
	}
	h.annotationStore.StoreSession(1, session)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations"}`)

	resp := h.toolGetAnnotations(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "make this darker") {
		t.Errorf("expected annotation text in result, got %q", text)
	}
	if !strings.Contains(text, "/tmp/test.png") {
		t.Errorf("expected screenshot path in result, got %q", text)
	}
}

func TestToolGetAnnotationDetail_Missing(t *testing.T) {
	h := createTestToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "nonexistent"}`)

	resp := h.toolGetAnnotationDetail(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "not found") && !strings.Contains(text, "expired") {
		t.Errorf("expected not found error, got %q", text)
	}
}

func TestToolGetAnnotationDetail_Found(t *testing.T) {
	h := createTestToolHandler(t)

	detail := AnnotationDetail{
		CorrelationID:  "detail_1",
		Selector:       "button.primary",
		Tag:            "button",
		TextContent:    "Submit",
		Classes:        []string{"primary"},
		ComputedStyles: map[string]string{"color": "rgb(255,255,255)"},
	}
	h.annotationStore.StoreDetail("detail_1", detail)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_1"}`)

	resp := h.toolGetAnnotationDetail(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "button.primary") {
		t.Errorf("expected selector in result, got %q", text)
	}
}

func TestToolGetAnnotations_FullResponseShape(t *testing.T) {
	h := createTestToolHandler(t)

	session := &AnnotationSession{
		Annotations: []Annotation{
			{
				ID:             "ann_1",
				Text:           "make this darker",
				ElementSummary: "button.primary 'Submit'",
				CorrelationID:  "detail_1",
				Rect:           AnnotationRect{X: 100, Y: 200, Width: 150, Height: 50},
				PageURL:        "https://example.com",
				Timestamp:      1700000000000,
			},
			{
				ID:             "ann_2",
				Text:           "increase font",
				ElementSummary: "p.body 'Lorem'",
				CorrelationID:  "detail_2",
				Rect:           AnnotationRect{X: 300, Y: 400, Width: 200, Height: 30},
				PageURL:        "https://example.com",
				Timestamp:      1700000001000,
			},
		},
		ScreenshotPath: "/tmp/annotated.png",
		PageURL:        "https://example.com",
		TabID:          1,
		Timestamp:      1700000002000,
	}
	h.annotationStore.StoreSession(1, session)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations"}`)

	resp := h.toolGetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	// Verify top-level fields
	if data["count"] != float64(2) {
		t.Errorf("Expected count=2, got %v", data["count"])
	}
	if data["page_url"] != "https://example.com" {
		t.Errorf("Expected page_url, got %v", data["page_url"])
	}
	if data["screenshot"] != "/tmp/annotated.png" {
		t.Errorf("Expected screenshot path, got %v", data["screenshot"])
	}

	// Verify annotations array
	anns, ok := data["annotations"].([]any)
	if !ok || len(anns) != 2 {
		t.Fatalf("Expected annotations array with 2 items, got %v", data["annotations"])
	}

	first := anns[0].(map[string]any)
	for _, field := range []string{"id", "text", "element_summary", "correlation_id", "rect"} {
		if _, exists := first[field]; !exists {
			t.Errorf("Missing field %q in annotation", field)
		}
	}
	if first["text"] != "make this darker" {
		t.Errorf("Expected text 'make this darker', got %v", first["text"])
	}
	if first["correlation_id"] != "detail_1" {
		t.Errorf("Expected correlation_id 'detail_1', got %v", first["correlation_id"])
	}

	// Verify rect sub-object
	rect, ok := first["rect"].(map[string]any)
	if !ok {
		t.Fatal("Expected rect to be an object")
	}
	if rect["x"] != float64(100) || rect["width"] != float64(150) {
		t.Errorf("Unexpected rect values: %v", rect)
	}
}

func TestToolGetAnnotationDetail_FullResponseShape(t *testing.T) {
	h := createTestToolHandler(t)

	detail := AnnotationDetail{
		CorrelationID:  "detail_full",
		Selector:       "button#submit-btn",
		Tag:            "button",
		TextContent:    "Submit Order",
		Classes:        []string{"primary", "rounded"},
		ID:             "submit-btn",
		ComputedStyles: map[string]string{"background-color": "rgb(59, 130, 246)", "font-size": "14px"},
		ParentSelector: "form.checkout > div.actions > button#submit-btn",
		BoundingRect:   AnnotationRect{X: 100, Y: 200, Width: 150, Height: 50},
	}
	h.annotationStore.StoreDetail("detail_full", detail)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_full"}`)

	resp := h.toolGetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v\nraw text: %s", err, text)
	}

	checks := map[string]any{
		"correlation_id":  "detail_full",
		"selector":        "button#submit-btn",
		"tag":             "button",
		"text_content":    "Submit Order",
		"id":              "submit-btn",
		"parent_selector": "form.checkout > div.actions > button#submit-btn",
	}
	for field, expected := range checks {
		if data[field] != expected {
			t.Errorf("Field %q: expected %v, got %v", field, expected, data[field])
		}
	}

	// Verify classes array
	classes, ok := data["classes"].([]any)
	if !ok || len(classes) != 2 {
		t.Fatalf("Expected classes array with 2 items, got %v", data["classes"])
	}

	// Verify computed_styles
	styles, ok := data["computed_styles"].(map[string]any)
	if !ok {
		t.Fatal("Expected computed_styles to be an object")
	}
	if styles["background-color"] != "rgb(59, 130, 246)" {
		t.Errorf("Expected background-color, got %v", styles["background-color"])
	}

	// Verify bounding_rect
	rect, ok := data["bounding_rect"].(map[string]any)
	if !ok {
		t.Fatal("Expected bounding_rect to be an object")
	}
	if rect["x"] != float64(100) || rect["width"] != float64(150) {
		t.Errorf("Unexpected bounding_rect: %v", rect)
	}
}

func TestToolGetAnnotations_ZeroAnnotationsFlow(t *testing.T) {
	h := createTestToolHandler(t)

	// Use a fresh store to avoid cross-test contamination from globalAnnotationStore
	h.annotationStore = NewAnnotationStore(10 * time.Minute)
	defer h.annotationStore.Close()

	session := &AnnotationSession{
		Annotations:    []Annotation{},
		ScreenshotPath: "/tmp/empty.png",
		PageURL:        "https://example.com/empty",
		TabID:          5,
		Timestamp:      time.Now().UnixMilli(),
	}
	h.annotationStore.StoreSession(5, session)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations"}`)

	resp := h.toolGetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if data["count"] != float64(0) {
		t.Errorf("Expected count=0, got %v", data["count"])
	}
	anns, ok := data["annotations"].([]any)
	if !ok {
		t.Fatal("Expected annotations to be an array")
	}
	if len(anns) != 0 {
		t.Errorf("Expected empty annotations array, got %d items", len(anns))
	}
	if data["screenshot"] != "/tmp/empty.png" {
		t.Errorf("Expected screenshot path, got %v", data["screenshot"])
	}
}

func TestToolGetAnnotations_WaitTrue_ImmediateReturn(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = NewAnnotationStore(10 * time.Minute)
	defer h.annotationStore.Close()

	// Mark draw started, then store session
	h.annotationStore.MarkDrawStarted()
	time.Sleep(1 * time.Millisecond)
	h.annotationStore.StoreSession(1, &AnnotationSession{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "wait-immediate"}},
		PageURL:     "https://example.com",
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": true}`)

	resp := h.toolGetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "wait-immediate") {
		t.Errorf("expected annotation text, got %q", text)
	}
}

func TestToolGetAnnotations_WaitTrue_ReturnsCorrelationID(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = NewAnnotationStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": true}`)

	resp := h.toolGetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse response JSON: %v", err)
	}

	if data["status"] != "waiting_for_user" {
		t.Errorf("expected status 'waiting_for_user', got %v", data["status"])
	}
	corrID, ok := data["correlation_id"].(string)
	if !ok || corrID == "" {
		t.Error("expected non-empty correlation_id")
	}
	if !strings.HasPrefix(corrID, "ann_") {
		t.Errorf("expected correlation_id prefix 'ann_', got %q", corrID)
	}
	if !strings.Contains(text, "observe") {
		t.Error("expected polling instructions in message")
	}
}

func TestToolGetAnnotations_WaitTrue_ImmediateIfDataReady(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = NewAnnotationStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.MarkDrawStarted()
	time.Sleep(2 * time.Millisecond) // ensure session timestamp > draw start

	// Store session BEFORE calling wait — should return immediately
	h.annotationStore.StoreSession(1, &AnnotationSession{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "already-done"}},
		PageURL:     "https://example.com",
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": true}`)

	resp := h.toolGetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "already-done") {
		t.Errorf("expected annotation text, got %q", text)
	}
}

func TestToolGetAnnotations_WaitTrue_WaiterCompletedOnStore(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = NewAnnotationStore(10 * time.Minute)
	defer h.annotationStore.Close()

	// Track completed commands
	var completedID string
	var completedResult json.RawMessage
	h.annotationStore.SetCommandCompleter(func(corrID string, result json.RawMessage) {
		completedID = corrID
		completedResult = result
	})

	h.annotationStore.MarkDrawStarted()

	// Call wait=true — returns correlation_id immediately
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": true}`)
	resp := h.toolGetAnnotations(req, args)

	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)
	var data map[string]any
	_ = json.Unmarshal([]byte(jsonText), &data)
	corrID := data["correlation_id"].(string)

	// Now store annotations — should trigger the waiter completion
	h.annotationStore.StoreSession(1, &AnnotationSession{
		TabID:       1,
		Timestamp:   time.Now().UnixMilli(),
		Annotations: []Annotation{{Text: "async-result"}},
		PageURL:     "https://example.com",
	})

	if completedID != corrID {
		t.Errorf("expected completed correlation_id %q, got %q", corrID, completedID)
	}
	if !strings.Contains(string(completedResult), "async-result") {
		t.Errorf("expected result to contain annotation text, got %s", completedResult)
	}
}

func TestToolGetAnnotations_WaitFalse_DefaultBehavior(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = NewAnnotationStore(10 * time.Minute)
	defer h.annotationStore.Close()

	// No session exists, wait=false — should return immediately with no-data message
	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "wait": false}`)

	resp := h.toolGetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "No annotation") {
		t.Errorf("expected no annotation message, got %q", text)
	}
}

func TestToolGetAnnotations_NamedSession(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = NewAnnotationStore(10 * time.Minute)
	defer h.annotationStore.Close()

	h.annotationStore.AppendToNamedSession("qa", &AnnotationSession{
		TabID:       1,
		Timestamp:   100,
		PageURL:     "https://example.com/login",
		Annotations: []Annotation{{Text: "fix button"}},
	})
	h.annotationStore.AppendToNamedSession("qa", &AnnotationSession{
		TabID:          1,
		Timestamp:      200,
		PageURL:        "https://example.com/dashboard",
		ScreenshotPath: "/tmp/dash.png",
		Annotations:    []Annotation{{Text: "wrong color"}, {Text: "misaligned"}},
	})

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "annot_session": "qa"}`)

	resp := h.toolGetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if data["annot_session_name"] != "qa" {
		t.Errorf("expected annot_session_name 'qa', got %v", data["annot_session_name"])
	}
	if data["page_count"] != float64(2) {
		t.Errorf("expected page_count 2, got %v", data["page_count"])
	}
	if data["total_count"] != float64(3) {
		t.Errorf("expected total_count 3, got %v", data["total_count"])
	}

	pages, ok := data["pages"].([]any)
	if !ok || len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %v", data["pages"])
	}
	p1 := pages[0].(map[string]any)
	if p1["page_url"] != "https://example.com/login" {
		t.Errorf("expected first page URL, got %v", p1["page_url"])
	}
	p2 := pages[1].(map[string]any)
	if p2["screenshot"] != "/tmp/dash.png" {
		t.Errorf("expected screenshot on page 2, got %v", p2["screenshot"])
	}
}

func TestToolGetAnnotations_NamedSession_NotFound(t *testing.T) {
	h := createTestToolHandler(t)
	h.annotationStore = NewAnnotationStore(10 * time.Minute)
	defer h.annotationStore.Close()

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotations", "annot_session": "nonexistent"}`)

	resp := h.toolGetAnnotations(req, args)
	text := unmarshalMCPText(t, resp.Result)

	if !strings.Contains(text, "not found") {
		t.Errorf("expected not found message, got %q", text)
	}
}

func TestToolGetAnnotationDetail_WithA11yFlags(t *testing.T) {
	h := createTestToolHandler(t)

	detail := AnnotationDetail{
		CorrelationID: "detail_a11y",
		Selector:      "div.clickable",
		Tag:           "div",
		A11yFlags:     []string{"interactive_without_role", "low_contrast:2.1:1", "small_touch_target:32x28"},
	}
	h.annotationStore.StoreDetail("detail_a11y", detail)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_a11y"}`)

	resp := h.toolGetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	flags, ok := data["a11y_flags"].([]any)
	if !ok {
		t.Fatal("expected a11y_flags array")
	}
	if len(flags) != 3 {
		t.Errorf("expected 3 a11y flags, got %d", len(flags))
	}
	if flags[0] != "interactive_without_role" {
		t.Errorf("expected first flag, got %v", flags[0])
	}
}

func TestToolGetAnnotationDetail_NoA11yFlags(t *testing.T) {
	h := createTestToolHandler(t)

	detail := AnnotationDetail{
		CorrelationID: "detail_clean",
		Selector:      "button.primary",
		Tag:           "button",
		A11yFlags:     nil,
	}
	h.annotationStore.StoreDetail("detail_clean", detail)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail", "correlation_id": "detail_clean"}`)

	resp := h.toolGetAnnotationDetail(req, args)
	text := unmarshalMCPText(t, resp.Result)
	jsonText := extractJSONFromText(text)

	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// a11y_flags should be absent when empty
	if _, exists := data["a11y_flags"]; exists {
		t.Error("expected a11y_flags to be absent when empty")
	}
}

func TestToolGetAnnotationDetail_MissingCorrelationID(t *testing.T) {
	h := createTestToolHandler(t)

	req := JSONRPCRequest{JSONRPC: "2.0", ID: float64(1)}
	args := json.RawMessage(`{"what": "annotation_detail"}`)

	resp := h.toolGetAnnotationDetail(req, args)

	text := unmarshalMCPText(t, resp.Result)
	if !strings.Contains(text, "correlation_id") {
		t.Errorf("expected missing param error, got %q", text)
	}
}

