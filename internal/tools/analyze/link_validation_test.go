// link_validation_test.go — Tests for link validation pure functions.
package analyze

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClampInt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		v, def, min, max, want int
	}{
		{0, 10, 1, 100, 10},    // zero uses default
		{5, 10, 1, 100, 5},     // in range
		{-1, 10, 1, 100, 1},    // below min
		{200, 10, 1, 100, 100}, // above max
		{50, 10, 1, 100, 50},   // in range
	}

	for _, tc := range tests {
		got := ClampInt(tc.v, tc.def, tc.min, tc.max)
		if got != tc.want {
			t.Errorf("ClampInt(%d, %d, %d, %d) = %d, want %d",
				tc.v, tc.def, tc.min, tc.max, got, tc.want)
		}
	}
}

func TestFilterHTTPURLs(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com",
		"http://example.com",
		"ftp://example.com",
		"mailto:test@example.com",
		"javascript:void(0)",
		"https://other.com/path",
	}

	filtered := FilterHTTPURLs(urls)
	if len(filtered) != 3 {
		t.Errorf("FilterHTTPURLs len = %d, want 3", len(filtered))
	}
	for _, u := range filtered {
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			t.Errorf("filtered URL %q should start with http:// or https://", u)
		}
	}
}

func TestClassifyHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		want   string
	}{
		{200, "ok"},
		{201, "ok"},
		{299, "ok"},
		{301, "redirect"},
		{302, "redirect"},
		{399, "redirect"},
		{401, "requires_auth"},
		{403, "requires_auth"},
		{404, "broken"},
		{500, "broken"},
		{100, "broken"},
	}

	for _, tc := range tests {
		got := ClassifyHTTPStatus(tc.status)
		if got != tc.want {
			t.Errorf("ClassifyHTTPStatus(%d) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestFilterHTTPURLs_Empty(t *testing.T) {
	t.Parallel()

	filtered := FilterHTTPURLs(nil)
	if len(filtered) != 0 {
		t.Errorf("FilterHTTPURLs(nil) len = %d, want 0", len(filtered))
	}

	filtered = FilterHTTPURLs([]string{})
	if len(filtered) != 0 {
		t.Errorf("FilterHTTPURLs([]) len = %d, want 0", len(filtered))
	}
}

func TestFilterHTTPURLs_AllValid(t *testing.T) {
	t.Parallel()

	urls := []string{"http://a.com", "https://b.com"}
	filtered := FilterHTTPURLs(urls)
	if len(filtered) != 2 {
		t.Errorf("FilterHTTPURLs len = %d, want 2", len(filtered))
	}
}

func TestFilterHTTPURLs_NoneValid(t *testing.T) {
	t.Parallel()

	urls := []string{"ftp://a.com", "mailto:x@y.com", "data:text/html,hello"}
	filtered := FilterHTTPURLs(urls)
	if len(filtered) != 0 {
		t.Errorf("FilterHTTPURLs len = %d, want 0", len(filtered))
	}
}

func TestClampInt_BoundaryValues(t *testing.T) {
	t.Parallel()

	// Exactly at min
	if got := ClampInt(1, 10, 1, 100); got != 1 {
		t.Errorf("ClampInt(1, 10, 1, 100) = %d, want 1", got)
	}
	// Exactly at max
	if got := ClampInt(100, 10, 1, 100); got != 100 {
		t.Errorf("ClampInt(100, 10, 1, 100) = %d, want 100", got)
	}
}

// ============================================
// ClassifyHTTPStatus — extended from pure_functions_test.go
// ============================================

func TestClassifyHTTPStatus_Extended(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		want   string
	}{
		{204, "ok"},
		{304, "redirect"},
		{503, "broken"},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			got := ClassifyHTTPStatus(tt.status)
			if got != tt.want {
				t.Errorf("ClassifyHTTPStatus(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ============================================
// BuildLinkResult
// ============================================

func TestBuildLinkResult(t *testing.T) {
	t.Run("200 OK no redirect", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("hello")),
		}
		result := BuildLinkResult(resp, "https://example.com", 50)
		if result.URL != "https://example.com" {
			t.Errorf("URL = %q", result.URL)
		}
		if result.Status != 200 {
			t.Errorf("Status = %d", result.Status)
		}
		if result.Code != "ok" {
			t.Errorf("Code = %q, want ok", result.Code)
		}
		if result.TimeMS != 50 {
			t.Errorf("TimeMS = %d", result.TimeMS)
		}
		if result.RedirectTo != "" {
			t.Errorf("RedirectTo = %q, want empty", result.RedirectTo)
		}
	})

	t.Run("301 redirect with Location header", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 301,
			Header:     http.Header{"Location": []string{"https://new.example.com"}},
			Body:       io.NopCloser(strings.NewReader("")),
		}
		result := BuildLinkResult(resp, "https://old.example.com", 30)
		if result.Code != "redirect" {
			t.Errorf("Code = %q, want redirect", result.Code)
		}
		if result.RedirectTo != "https://new.example.com" {
			t.Errorf("RedirectTo = %q", result.RedirectTo)
		}
	})

	t.Run("body drain error", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       io.NopCloser(&errorReader{}),
		}
		result := BuildLinkResult(resp, "https://example.com", 10)
		if result.Code != "broken" {
			t.Errorf("Code = %q, want broken", result.Code)
		}
		if result.Error == "" {
			t.Error("expected non-empty error for body drain failure")
		}
	})
}

// errorReader always returns an error on Read.
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}
