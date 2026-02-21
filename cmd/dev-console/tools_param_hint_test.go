// tools_param_hint_test.go — Coverage for inline valid-param hints in structured errors.
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolErrors_IncludeInlineValidParamsHint(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	tests := []struct {
		name       string
		tool       string
		args       string
		wantCode   string
		wantParams []string
	}{
		{
			name:       "analyze dom missing selector",
			tool:       "analyze",
			args:       `{"what":"dom"}`,
			wantCode:   ErrMissingParam,
			wantParams: []string{"what", "selector"},
		},
		{
			name:       "observe logs invalid scope",
			tool:       "observe",
			args:       `{"what":"logs","scope":"bogus"}`,
			wantCode:   ErrInvalidParam,
			wantParams: []string{"what", "scope"},
		},
		{
			name:       "configure noise_rule remove missing rule_id",
			tool:       "configure",
			args:       `{"what":"noise_rule","noise_action":"remove"}`,
			wantCode:   ErrMissingParam,
			wantParams: []string{"what", "noise_action", "rule_id"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			resp, handled := h.HandleToolCall(req, tc.tool, json.RawMessage(tc.args))
			if !handled {
				t.Fatalf("%s: expected handled=true", tc.tool)
			}

			errData := extractStructuredErrorJSON(t, resp.Result)
			if got, _ := errData["error"].(string); got != tc.wantCode {
				t.Fatalf("error code = %q, want %q", got, tc.wantCode)
			}

			hint, _ := errData["hint"].(string)
			if !strings.Contains(hint, "Valid params") {
				t.Fatalf("hint missing inline valid params guidance: %q", hint)
			}
			for _, p := range tc.wantParams {
				if !strings.Contains(hint, p) {
					t.Fatalf("hint should include param %q, got: %q", p, hint)
				}
			}
		})
	}
}

func TestToolErrors_PreservesGenerateModeSpecificHint(t *testing.T) {
	t.Parallel()

	h, _, _ := makeToolHandler(t)
	req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}

	resp, handled := h.HandleToolCall(req, "generate", json.RawMessage(`{"what":"har","limit":5}`))
	if !handled {
		t.Fatal("generate: expected handled=true")
	}

	errData := extractStructuredErrorJSON(t, resp.Result)
	hint, _ := errData["hint"].(string)
	if !strings.Contains(hint, "Valid params for 'har':") {
		t.Fatalf("expected mode-specific generate hint, got: %q", hint)
	}
	if strings.Count(hint, "Valid params for 'har':") != 1 {
		t.Fatalf("expected single generate hint occurrence, got: %q", hint)
	}
}
