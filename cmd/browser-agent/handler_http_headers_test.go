// Purpose: Tests HTTP request-context header extraction for MCP-over-HTTP.
// Docs: docs/core/mcp-correctness.md

package main

import (
	"net/http/httptest"
	"testing"
)

func TestNewHTTPRequestContextReadsKaboomHeaders(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "http://localhost/mcp", nil)
	req.Header.Set("X-Kaboom-Ext-Session", "session-123")
	req.Header.Set("X-Kaboom-Client", "kaboom-extension/1.2.3")
	req.Header.Set("X-Kaboom-Extension-Version", "1.2.3")
	req.Header.Set("Authorization", "Bearer secret-token")

	ctx := newHTTPRequestContext(req, "9.9.9")

	if ctx.extSessionID != "session-123" {
		t.Fatalf("extSessionID = %q, want session-123", ctx.extSessionID)
	}
	if ctx.clientID != "kaboom-extension/1.2.3" {
		t.Fatalf("clientID = %q, want kaboom-extension/1.2.3", ctx.clientID)
	}
	if got := ctx.headers["Authorization"]; got != "[REDACTED]" {
		t.Fatalf("Authorization header = %q, want [REDACTED]", got)
	}
	if got := ctx.headers["X-Kaboom-Extension-Version"]; got != "1.2.3" {
		t.Fatalf("X-Kaboom-Extension-Version = %q, want 1.2.3", got)
	}
}
