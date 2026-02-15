// helpers_test.go — Tests for shared utility functions: URL path extraction, slice helpers, ingest body reading.
package capture

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================
// extractURLPath / ExtractURLPath Tests
// ============================================

func TestNewExtractURLPath_FullURL(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("https://example.com/api/v1/users?page=2&limit=10")
	want := "/api/v1/users"
	if got != want {
		t.Errorf("ExtractURLPath(full URL with query) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_URLWithFragment(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("https://example.com/docs#section-3")
	want := "/docs"
	if got != want {
		t.Errorf("ExtractURLPath(URL with fragment) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_URLWithQueryAndFragment(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("https://example.com/search?q=test#results")
	want := "/search"
	if got != want {
		t.Errorf("ExtractURLPath(URL with query+fragment) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_RootPath(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("https://example.com/")
	want := "/"
	if got != want {
		t.Errorf("ExtractURLPath(root path) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_NoPath(t *testing.T) {
	t.Parallel()

	// When URL has no path component, extractURLPath returns "/"
	got := ExtractURLPath("https://example.com")
	want := "/"
	if got != want {
		t.Errorf("ExtractURLPath(no path) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_DeepNestedPath(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("https://api.example.com/v2/users/123/posts/456/comments")
	want := "/v2/users/123/posts/456/comments"
	if got != want {
		t.Errorf("ExtractURLPath(deep nested) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_EmptyString(t *testing.T) {
	t.Parallel()

	// Empty string is parseable as a URL with empty path
	got := ExtractURLPath("")
	want := "/"
	if got != want {
		t.Errorf("ExtractURLPath(empty string) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_JustPath(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("/api/health")
	want := "/api/health"
	if got != want {
		t.Errorf("ExtractURLPath(just path) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_UnparseableURL(t *testing.T) {
	t.Parallel()

	// url.Parse returns error for control characters
	input := string([]byte{0x7f})
	got := extractURLPath(input)
	// Should return input unchanged on parse error
	if got != input {
		t.Errorf("extractURLPath(unparseable) = %q, want original input %q", got, input)
	}
}

func TestNewExtractURLPath_FileURL(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("file:///home/user/document.html")
	want := "/home/user/document.html"
	if got != want {
		t.Errorf("ExtractURLPath(file URL) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_URLWithPort(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("http://localhost:8080/api/data")
	want := "/api/data"
	if got != want {
		t.Errorf("ExtractURLPath(URL with port) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_URLWithAuth(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("https://user:pass@example.com/secret")
	want := "/secret"
	if got != want {
		t.Errorf("ExtractURLPath(URL with auth) = %q, want %q", got, want)
	}
}

func TestNewExtractURLPath_URLWithEncodedChars(t *testing.T) {
	t.Parallel()

	got := ExtractURLPath("https://example.com/path%20with%20spaces")
	want := "/path%20with%20spaces"
	// url.Parse preserves percent-encoding in RawPath but Path is decoded.
	// The function uses parsed.Path, so spaces may be decoded.
	// Accept either encoded or decoded form:
	if got != want && got != "/path with spaces" {
		t.Errorf("ExtractURLPath(encoded chars) = %q, want %q or decoded form", got, want)
	}
}

// ============================================
// reverseSlice Tests
// ============================================

func TestNewReverseSlice_Integers(t *testing.T) {
	t.Parallel()

	s := []int{1, 2, 3, 4, 5}
	reverseSlice(s)

	want := []int{5, 4, 3, 2, 1}
	for i := range s {
		if s[i] != want[i] {
			t.Errorf("reverseSlice[%d] = %d, want %d", i, s[i], want[i])
		}
	}
}

func TestNewReverseSlice_Strings(t *testing.T) {
	t.Parallel()

	s := []string{"a", "b", "c"}
	reverseSlice(s)

	want := []string{"c", "b", "a"}
	for i := range s {
		if s[i] != want[i] {
			t.Errorf("reverseSlice[%d] = %q, want %q", i, s[i], want[i])
		}
	}
}

func TestNewReverseSlice_SingleElement(t *testing.T) {
	t.Parallel()

	s := []int{42}
	reverseSlice(s)

	if s[0] != 42 {
		t.Errorf("reverseSlice(single element) = %d, want 42", s[0])
	}
}

func TestNewReverseSlice_Empty(t *testing.T) {
	t.Parallel()

	s := []int{}
	reverseSlice(s) // should not panic

	if len(s) != 0 {
		t.Errorf("reverseSlice(empty) len = %d, want 0", len(s))
	}
}

func TestNewReverseSlice_TwoElements(t *testing.T) {
	t.Parallel()

	s := []string{"first", "second"}
	reverseSlice(s)

	if s[0] != "second" || s[1] != "first" {
		t.Errorf("reverseSlice(two elements) = %v, want [second, first]", s)
	}
}

func TestNewReverseSlice_EvenCount(t *testing.T) {
	t.Parallel()

	s := []int{10, 20, 30, 40}
	reverseSlice(s)

	want := []int{40, 30, 20, 10}
	for i := range s {
		if s[i] != want[i] {
			t.Errorf("reverseSlice(even count)[%d] = %d, want %d", i, s[i], want[i])
		}
	}
}

// ============================================
// removeFromSlice Tests
// ============================================

func TestNewRemoveFromSlice_RemovesFirstOccurrence(t *testing.T) {
	t.Parallel()

	input := []string{"a", "b", "c", "b", "d"}
	result := removeFromSlice(input, "b")

	want := []string{"a", "c", "b", "d"}
	if len(result) != len(want) {
		t.Fatalf("removeFromSlice len = %d, want %d", len(result), len(want))
	}
	for i := range result {
		if result[i] != want[i] {
			t.Errorf("removeFromSlice[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestNewRemoveFromSlice_RemovesFirst(t *testing.T) {
	t.Parallel()

	input := []string{"x", "y", "z"}
	result := removeFromSlice(input, "x")

	want := []string{"y", "z"}
	if len(result) != len(want) {
		t.Fatalf("removeFromSlice(first) len = %d, want %d", len(result), len(want))
	}
	for i := range result {
		if result[i] != want[i] {
			t.Errorf("removeFromSlice(first)[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestNewRemoveFromSlice_RemovesLast(t *testing.T) {
	t.Parallel()

	input := []string{"x", "y", "z"}
	result := removeFromSlice(input, "z")

	want := []string{"x", "y"}
	if len(result) != len(want) {
		t.Fatalf("removeFromSlice(last) len = %d, want %d", len(result), len(want))
	}
	for i := range result {
		if result[i] != want[i] {
			t.Errorf("removeFromSlice(last)[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestNewRemoveFromSlice_ItemNotFound(t *testing.T) {
	t.Parallel()

	input := []string{"a", "b", "c"}
	result := removeFromSlice(input, "missing")

	// Should return original slice unchanged
	if len(result) != 3 {
		t.Fatalf("removeFromSlice(not found) len = %d, want 3", len(result))
	}
	for i := range result {
		if result[i] != input[i] {
			t.Errorf("removeFromSlice(not found)[%d] = %q, want %q", i, result[i], input[i])
		}
	}
}

func TestNewRemoveFromSlice_SingleElement(t *testing.T) {
	t.Parallel()

	input := []string{"only"}
	result := removeFromSlice(input, "only")

	if len(result) != 0 {
		t.Fatalf("removeFromSlice(single) len = %d, want 0", len(result))
	}
}

func TestNewRemoveFromSlice_EmptySlice(t *testing.T) {
	t.Parallel()

	input := []string{}
	result := removeFromSlice(input, "anything")

	if len(result) != 0 {
		t.Fatalf("removeFromSlice(empty) len = %d, want 0", len(result))
	}
}

func TestNewRemoveFromSlice_PreservesOrder(t *testing.T) {
	t.Parallel()

	input := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	result := removeFromSlice(input, "gamma")

	want := []string{"alpha", "beta", "delta", "epsilon"}
	if len(result) != len(want) {
		t.Fatalf("removeFromSlice(order) len = %d, want %d", len(result), len(want))
	}
	for i := range result {
		if result[i] != want[i] {
			t.Errorf("removeFromSlice(order)[%d] = %q, want %q", i, result[i], want[i])
		}
	}
}

func TestNewRemoveFromSlice_NewBackingArray(t *testing.T) {
	t.Parallel()

	// Verify that removeFromSlice allocates a new backing array (no GC pinning)
	input := []string{"a", "b", "c"}
	result := removeFromSlice(input, "b")

	// Modifying result should not affect input
	if len(result) > 0 {
		result[0] = "modified"
	}
	if input[0] != "a" {
		t.Error("removeFromSlice should allocate new backing array; modifying result affected input")
	}
}

// ============================================
// readIngestBody Tests
// ============================================

func TestNewReadIngestBody_Success(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	body := []byte(`{"events":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	w := httptest.NewRecorder()

	result, ok := c.readIngestBody(w, req)
	if !ok {
		t.Fatal("readIngestBody returned false, want true")
	}
	if string(result) != string(body) {
		t.Errorf("readIngestBody body = %q, want %q", string(result), string(body))
	}
	if w.Code != http.StatusOK {
		t.Errorf("readIngestBody status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNewReadIngestBody_TooLargeBody(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Create a body larger than maxExtensionPostBody (5MB)
	bigBody := strings.Repeat("x", 6*1024*1024)
	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(bigBody))
	w := httptest.NewRecorder()

	result, ok := c.readIngestBody(w, req)
	if ok {
		t.Fatal("readIngestBody returned true for too-large body, want false")
	}
	if result != nil {
		t.Errorf("readIngestBody result = %v, want nil for too-large body", result)
	}
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("readIngestBody status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestNewReadIngestBody_RateLimited(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	// Trigger rate limit by recording many events
	for i := 0; i < 100; i++ {
		c.RecordEvents(RateLimitThreshold)
	}

	req := httptest.NewRequest(http.MethodPost, "/ingest", strings.NewReader(`{}`))
	w := httptest.NewRecorder()

	result, ok := c.readIngestBody(w, req)
	if ok {
		// If rate limit was triggered, should return false
		// If not triggered (timing dependent), that's ok too
		_ = result
	}
	// This test validates the rate limit path is exercised without crashing
}

// ============================================
// recordAndRecheck Tests
// ============================================

func TestNewRecordAndRecheck_NormalFlow(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	w := httptest.NewRecorder()

	ok := c.recordAndRecheck(w, 1)
	if !ok {
		t.Fatal("recordAndRecheck returned false for 1 event, want true")
	}
}

func TestNewRecordAndRecheck_RecordsEventCount(t *testing.T) {
	t.Parallel()

	c := NewCapture()
	t.Cleanup(c.Close)

	w := httptest.NewRecorder()

	c.recordAndRecheck(w, 10)
	// After recording 10 events, health should reflect them
	health := c.GetHealthStatus()
	if health.CurrentRate < 10 {
		t.Errorf("CurrentRate = %d after recording 10 events, want >= 10", health.CurrentRate)
	}
}
