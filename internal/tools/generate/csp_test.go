// csp_test.go — Tests for CSP generation helpers.
package generate

import "testing"

func TestExtractOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/path", "https://example.com"},
		{"https://example.com:8080/path", "https://example.com:8080"},
		{"http://localhost:3000/api/v1", "http://localhost:3000"},
		{"https://cdn.example.com/js/app.js?v=1", "https://cdn.example.com"},
		{"http://127.0.0.1:9222", "http://127.0.0.1:9222"},
		{"", ""},
		{"ftp://files.example.com", ""},
		{"not-a-url", ""},
	}

	for _, tt := range tests {
		got := ExtractOrigin(tt.url)
		if got != tt.want {
			t.Errorf("ExtractOrigin(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestResourceTypeToCSPDirective(t *testing.T) {
	t.Parallel()

	tests := []struct {
		contentType string
		want        string
	}{
		{"application/javascript", "script-src"},
		{"text/javascript", "script-src"},
		{"text/css", "style-src"},
		{"font/woff2", "font-src"},
		{"image/png", "img-src"},
		{"image/svg+xml", "img-src"},
		{"video/mp4", "media-src"},
		{"audio/mpeg", "media-src"},
		{"application/json", "connect-src"},
		{"text/html", "connect-src"},
		{"", "connect-src"},
	}

	for _, tt := range tests {
		got := resourceTypeToCSPDirective(tt.contentType)
		if got != tt.want {
			t.Errorf("resourceTypeToCSPDirective(%q) = %q, want %q", tt.contentType, got, tt.want)
		}
	}
}
