// tools_core_unit_test.go — Unit tests for ToolHandler getters.
package main

import (
	"testing"

	"github.com/dev-console/dev-console/internal/capture"
)

func TestGetCapture(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	server, err := NewServer("/tmp/test-getters.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.toolHandler.(*ToolHandler)

	if h.GetCapture() != cap {
		t.Fatal("GetCapture should return the injected capture")
	}
}

func TestGetToolCallLimiter(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	server, err := NewServer("/tmp/test-limiter.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.toolHandler.(*ToolHandler)

	limiter := h.GetToolCallLimiter()
	if limiter == nil {
		t.Fatal("GetToolCallLimiter should return non-nil limiter")
	}
	// Limiter should allow calls
	if !limiter.Allow() {
		t.Fatal("fresh limiter should allow first call")
	}
}

func TestGetRedactionEngine_Configured(t *testing.T) {
	t.Parallel()

	cap := capture.NewCapture()
	server, err := NewServer("/tmp/test-redaction.jsonl", 100)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	mcpHandler := NewToolHandler(server, cap)
	h := mcpHandler.toolHandler.(*ToolHandler)

	if h.GetRedactionEngine() == nil {
		t.Fatal("GetRedactionEngine should return a configured engine")
	}
}
