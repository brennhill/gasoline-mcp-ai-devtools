// pure_functions_test.go — Unit tests for pure functions with 0% coverage.
package main

import (
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
// classifyHTTPStatus, buildLinkResult tests moved to
// internal/tools/analyze/link_validation_test.go
// ============================================
