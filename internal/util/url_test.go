// url_test.go — Tests for URL parsing utilities.
package util

import "testing"

// ============================================
// ExtractURLPath Tests
// ============================================

func TestExtractURLPath_FullURL(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("https://example.com/api/v1/users?page=2&limit=10")
	if got != "/api/v1/users" {
		t.Errorf("ExtractURLPath(full URL with query) = %q, want /api/v1/users", got)
	}
}

func TestExtractURLPath_URLWithFragment(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("https://example.com/docs#section-3")
	if got != "/docs" {
		t.Errorf("ExtractURLPath(URL with fragment) = %q, want /docs", got)
	}
}

func TestExtractURLPath_RootPath(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("https://example.com/")
	if got != "/" {
		t.Errorf("ExtractURLPath(root path) = %q, want /", got)
	}
}

func TestExtractURLPath_NoPath(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("https://example.com")
	if got != "/" {
		t.Errorf("ExtractURLPath(no path) = %q, want /", got)
	}
}

func TestExtractURLPath_EmptyString(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("")
	if got != "/" {
		t.Errorf("ExtractURLPath(empty) = %q, want /", got)
	}
}

func TestExtractURLPath_JustPath(t *testing.T) {
	t.Parallel()
	got := ExtractURLPath("/api/health")
	if got != "/api/health" {
		t.Errorf("ExtractURLPath(just path) = %q, want /api/health", got)
	}
}

func TestExtractURLPath_UnparseableURL(t *testing.T) {
	t.Parallel()
	input := string([]byte{0x7f})
	got := ExtractURLPath(input)
	if got != input {
		t.Errorf("ExtractURLPath(unparseable) = %q, want original input %q", got, input)
	}
}

// ============================================
// ExtractOrigin Tests
// ============================================

func TestExtractOrigin_StandardHTTPS(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("https://example.com/path?query=1")
	if got != "https://example.com" {
		t.Errorf("ExtractOrigin(standard HTTPS) = %q, want https://example.com", got)
	}
}

func TestExtractOrigin_WithPort(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("http://localhost:8080/api")
	if got != "http://localhost:8080" {
		t.Errorf("ExtractOrigin(with port) = %q, want http://localhost:8080", got)
	}
}

func TestExtractOrigin_DataURL(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("data:text/html,<h1>Hello</h1>")
	if got != "" {
		t.Errorf("ExtractOrigin(data:) = %q, want empty", got)
	}
}

func TestExtractOrigin_BlobURL(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("blob:https://example.com/uuid-here")
	if got != "https://example.com" {
		t.Errorf("ExtractOrigin(blob:) = %q, want https://example.com", got)
	}
}

func TestExtractOrigin_NoScheme(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("example.com/path")
	if got != "" {
		t.Errorf("ExtractOrigin(no scheme) = %q, want empty", got)
	}
}

func TestExtractOrigin_NoHost(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("file:///path/to/file")
	if got != "" {
		t.Errorf("ExtractOrigin(file:///) = %q, want empty", got)
	}
}

func TestExtractOrigin_EmptyString(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("")
	if got != "" {
		t.Errorf("ExtractOrigin(empty) = %q, want empty", got)
	}
}

func TestExtractOrigin_MalformedURL(t *testing.T) {
	t.Parallel()
	got := ExtractOrigin("://invalid")
	if got != "" {
		t.Errorf("ExtractOrigin(malformed) = %q, want empty", got)
	}
}
