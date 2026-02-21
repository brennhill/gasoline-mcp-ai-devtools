// tools_action_schema_contract_test.go — Per-mode request schema/runtime contract checks.
package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/streaming"
	cfg "github.com/dev-console/dev-console/internal/tools/configure"
)

type requiredParamContractCase struct {
	tool      string
	mode      string
	wantParam string
	args      json.RawMessage
}

func TestContractPerModeRequiredParams_RuntimeMatchesCapabilities(t *testing.T) {
	cases := []requiredParamContractCase{
		{tool: "configure", mode: "test_boundary_start", wantParam: "test_id"},
		{tool: "configure", mode: "get_sequence", wantParam: "name"},
		{tool: "observe", mode: "command_result", wantParam: "correlation_id"},
		{tool: "observe", mode: "recording_actions", wantParam: "recording_id"},
		{tool: "analyze", mode: "dom", wantParam: "selector"},
		{tool: "analyze", mode: "annotation_detail", wantParam: "correlation_id"},
		{tool: "analyze", mode: "draw_session", wantParam: "file"},
		{tool: "analyze", mode: "link_validation", wantParam: "urls"},
		{tool: "interact", mode: "execute_js", wantParam: "script"},
		{tool: "interact", mode: "navigate", wantParam: "url"},
		{tool: "interact", mode: "save_state", wantParam: "snapshot_name"},
		{tool: "interact", mode: "set_storage", wantParam: "key", args: json.RawMessage(`{"what":"set_storage","storage_type":"localStorage"}`)},
		{tool: "interact", mode: "subtitle", wantParam: "text"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.tool+"_"+tc.mode, func(t *testing.T) {
			h := newTestToolHandler()
			h.alertBuffer = streaming.NewAlertBuffer()
			caps := cfg.BuildCapabilitiesMap(h.ToolsList())

			assertModeRequiredContains(t, caps, tc.tool, tc.mode, []string{"what", tc.wantParam})

			req := JSONRPCRequest{JSONRPC: "2.0", ID: json.RawMessage(`1`)}
			args := tc.args
			if len(args) == 0 {
				args = json.RawMessage(`{"what":"` + tc.mode + `"}`)
			}
			resp, handled := h.HandleToolCall(req, tc.tool, args)
			if !handled {
				t.Fatalf("%s(%s): tool call not handled", tc.tool, tc.mode)
			}

			var result MCPToolResult
			if err := json.Unmarshal(resp.Result, &result); err != nil {
				t.Fatalf("%s(%s): unmarshal MCPToolResult: %v", tc.tool, tc.mode, err)
			}
			if !result.IsError {
				t.Fatalf("%s(%s): expected error for missing %q", tc.tool, tc.mode, tc.wantParam)
			}

			errData := parseStructuredErrorData(t, result)
			if got, _ := errData["error"].(string); got != ErrMissingParam {
				t.Fatalf("%s(%s): error code = %q, want %q", tc.tool, tc.mode, got, ErrMissingParam)
			}
			if got, _ := errData["param"].(string); got != tc.wantParam {
				msg, _ := errData["message"].(string)
				if !strings.Contains(msg, "'"+tc.wantParam+"'") {
					t.Fatalf("%s(%s): param/message mismatch. param=%q message=%q want param %q", tc.tool, tc.mode, got, msg, tc.wantParam)
				}
			}
		})
	}
}

func parseStructuredErrorData(t *testing.T, result MCPToolResult) map[string]any {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("structured error missing content block")
	}
	text := result.Content[0].Text
	jsonText := extractJSONFromText(text)
	if strings.TrimSpace(jsonText) == "" {
		idx := strings.LastIndex(text, "{")
		if idx >= 0 {
			jsonText = text[idx:]
		}
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonText), &data); err != nil {
		idx := strings.LastIndex(text, "{")
		if idx >= 0 {
			if retryErr := json.Unmarshal([]byte(text[idx:]), &data); retryErr == nil {
				return data
			}
		}
		t.Fatalf("structured error payload is not valid JSON: %v", err)
	}
	return data
}

func assertModeRequiredContains(
	t *testing.T,
	caps map[string]any,
	toolName string,
	mode string,
	requiredParams []string,
) {
	t.Helper()

	toolRaw, ok := caps[toolName]
	if !ok {
		t.Fatalf("tool %q missing from capabilities", toolName)
	}
	toolMap, ok := toolRaw.(map[string]any)
	if !ok {
		t.Fatalf("tool %q type = %T, want map[string]any", toolName, toolRaw)
	}

	modeParamsRaw, ok := toolMap["mode_params"]
	if !ok {
		t.Fatalf("tool %q missing mode_params", toolName)
	}
	modeParams, ok := modeParamsRaw.(map[string]any)
	if !ok {
		t.Fatalf("tool %q mode_params type = %T, want map[string]any", toolName, modeParamsRaw)
	}

	modeRaw, ok := modeParams[mode]
	if !ok {
		t.Fatalf("tool %q mode %q missing from mode_params", toolName, mode)
	}
	modeMap, ok := modeRaw.(map[string]any)
	if !ok {
		t.Fatalf("tool %q mode %q type = %T, want map[string]any", toolName, mode, modeRaw)
	}

	required := toStringSliceLoose(modeMap["required"])
	requiredSet := make(map[string]bool, len(required))
	for _, p := range required {
		requiredSet[p] = true
	}
	for _, want := range requiredParams {
		if !requiredSet[want] {
			t.Fatalf("tool %q mode %q required missing %q (got %v)", toolName, mode, want, required)
		}
	}
}

func toStringSliceLoose(raw any) []string {
	switch typed := raw.(type) {
	case []string:
		out := make([]string, 0, len(typed))
		for _, v := range typed {
			if v != "" {
				out = append(out, v)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
