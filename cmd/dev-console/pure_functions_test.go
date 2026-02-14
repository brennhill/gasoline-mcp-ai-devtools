// pure_functions_test.go — Unit tests for pure functions with 0% coverage.
package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// ============================================
// extractRequestID
// ============================================

func TestExtractRequestID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  any
	}{
		{"valid JSON-RPC", `{"jsonrpc":"2.0","id":42,"method":"test"}`, float64(42)},
		{"string id", `{"jsonrpc":"2.0","id":"req-1","method":"test"}`, "req-1"},
		{"null id", `{"jsonrpc":"2.0","id":null,"method":"test"}`, nil},
		{"no id field", `{"jsonrpc":"2.0","method":"test"}`, nil},
		{"invalid JSON", `not json at all`, nil},
		{"empty string", ``, nil},
		{"partial JSON", `{"jsonrpc":"2.0"`, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRequestID(tt.input)
			if got != tt.want {
				t.Errorf("extractRequestID(%q) = %v (%T), want %v (%T)", tt.input, got, got, tt.want, tt.want)
			}
		})
	}
}

// ============================================
// classifyHTTPStatus
// ============================================

func TestClassifyHTTPStatus(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{200, "ok"},
		{201, "ok"},
		{204, "ok"},
		{299, "ok"},
		{301, "redirect"},
		{302, "redirect"},
		{304, "redirect"},
		{401, "requires_auth"},
		{403, "requires_auth"},
		{404, "broken"},
		{500, "broken"},
		{503, "broken"},
		{100, "broken"},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			got := classifyHTTPStatus(tt.status)
			if got != tt.want {
				t.Errorf("classifyHTTPStatus(%d) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

// ============================================
// buildLinkResult
// ============================================

func TestBuildLinkResult(t *testing.T) {
	t.Run("200 OK no redirect", func(t *testing.T) {
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       io.NopCloser(strings.NewReader("hello")),
		}
		result := buildLinkResult(resp, "https://example.com", 50)
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
		result := buildLinkResult(resp, "https://old.example.com", 30)
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
		result := buildLinkResult(resp, "https://example.com", 10)
		if result.Code != "broken" {
			t.Errorf("Code = %q, want broken", result.Code)
		}
		if result.Error == "" {
			t.Error("expected non-empty error for body drain failure")
		}
	})
}

// errorReader always returns an error on Read
type errorReader struct{}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}
