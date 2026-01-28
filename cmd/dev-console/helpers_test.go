package main

import (
	"testing"
)

func setupTestCapture(t *testing.T) *Capture {
	t.Helper()
	return NewCapture()
}

// setupToolHandler creates a NewToolHandler and registers cleanup to prevent goroutine leaks.
// The SessionStore's background goroutine is shut down when the test completes.
func setupToolHandler(t *testing.T, server *Server, capture *Capture) *MCPHandler {
	t.Helper()
	mcp := NewToolHandler(server, capture)
	t.Cleanup(func() {
		if mcp.toolHandler != nil && mcp.toolHandler.sessionStore != nil {
			mcp.toolHandler.sessionStore.Shutdown()
		}
	})
	return mcp
}

// ============================================
// Coverage Gap Tests: extractURLPath with query string
// ============================================

// ============================================
// Coverage: extractURLPath with unparseable URL (line 12)
// ============================================

func TestExtractURLPath_UnparseableURL(t *testing.T) {
	t.Parallel()
	// url.Parse returns error for URLs with invalid control characters
	rawURL := "http://example.com/path\x7f"
	result := extractURLPath(rawURL)
	// Should return the input unchanged since url.Parse fails
	if result != rawURL {
		t.Errorf("Expected unchanged input for unparseable URL, got %q", result)
	}
}

func TestExtractURLPath_WithQueryString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "path with query string",
			input:    "http://localhost:3000/api/users?page=1&limit=10",
			expected: "/api/users",
		},
		{
			name:     "path with fragment",
			input:    "http://localhost:3000/page#section",
			expected: "/page",
		},
		{
			name:     "path with query and fragment",
			input:    "http://localhost:3000/search?q=test#results",
			expected: "/search",
		},
		{
			name:     "root path with query string",
			input:    "http://localhost:3000/?debug=true",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractURLPath(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractURLPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
