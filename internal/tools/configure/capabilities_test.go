// capabilities_test.go — Tests for capability map building.
package configure

import (
	"sort"
	"testing"

	"github.com/dev-console/dev-console/internal/mcp"
	"github.com/dev-console/dev-console/internal/schema"
)

func TestBuildCapabilitiesMap_Empty(t *testing.T) {
	t.Parallel()

	result := BuildCapabilitiesMap(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestBuildCapabilitiesMap_SingleTool(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name:        "observe",
			Description: "Observe browser state",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"errors", "logs"},
					},
					"limit": map[string]any{
						"type": "number",
					},
				},
				"required": []string{"what"},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	observeRaw, ok := result["observe"]
	if !ok {
		t.Fatal("expected 'observe' in result")
	}
	observe := observeRaw.(map[string]any)

	if observe["dispatch_param"] != "what" {
		t.Errorf("dispatch_param = %v, want 'what'", observe["dispatch_param"])
	}

	modes, ok := observe["modes"].([]string)
	if !ok {
		t.Fatal("modes should be []string")
	}
	if len(modes) != 2 || modes[0] != "errors" || modes[1] != "logs" {
		t.Errorf("modes = %v, want [errors, logs]", modes)
	}

	params, ok := observe["params"].([]string)
	if !ok {
		t.Fatal("params should be []string")
	}
	if len(params) != 1 || params[0] != "limit" {
		t.Errorf("params = %v, want [limit]", params)
	}

	if observe["description"] != "Observe browser state" {
		t.Errorf("description = %v, want 'Observe browser state'", observe["description"])
	}
}

func TestBuildCapabilitiesMap_ParamsSorted(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name: "configure",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"action": map[string]any{"type": "string"},
					"zebra":  map[string]any{"type": "string"},
					"alpha":  map[string]any{"type": "string"},
					"middle": map[string]any{"type": "string"},
				},
				"required": []string{"action"},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	configRaw := result["configure"].(map[string]any)
	params := configRaw["params"].([]string)

	sorted := make([]string, len(params))
	copy(sorted, params)
	sort.Strings(sorted)

	for i := range params {
		if params[i] != sorted[i] {
			t.Errorf("params not sorted: %v", params)
			break
		}
	}
}

func TestBuildCapabilitiesMap_NoRequired(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name: "simple",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"foo": map[string]any{"type": "string"},
				},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	simple := result["simple"].(map[string]any)
	if simple["dispatch_param"] != "" {
		t.Errorf("dispatch_param = %v, want empty", simple["dispatch_param"])
	}
}

func TestBuildCapabilitiesMap_IncludesModeParamsAndTypeMetadata(t *testing.T) {
	t.Parallel()

	tools := []mcp.MCPTool{
		{
			Name:        "observe",
			Description: "Observe browser state",
			InputSchema: map[string]any{
				"properties": map[string]any{
					"what": map[string]any{
						"type": "string",
						"enum": []string{"errors", "logs"},
					},
					"timeout_ms": map[string]any{
						"type":        "number",
						"description": "Timeout ms (default 5000)",
					},
					"url": map[string]any{
						"type": "string",
					},
				},
				"required": []string{"what"},
			},
		},
	}

	result := BuildCapabilitiesMap(tools)
	observeRaw, ok := result["observe"]
	if !ok {
		t.Fatal("expected observe tool in result")
	}
	observe, ok := observeRaw.(map[string]any)
	if !ok {
		t.Fatalf("observe type = %T, want map[string]any", observeRaw)
	}

	modeParamsRaw, ok := observe["mode_params"]
	if !ok {
		t.Fatal("expected mode_params in capabilities")
	}
	modeParams, ok := modeParamsRaw.(map[string]any)
	if !ok {
		t.Fatalf("mode_params type = %T, want map[string]any", modeParamsRaw)
	}
	errorsMode, ok := modeParams["errors"].(map[string]any)
	if !ok {
		t.Fatalf("errors mode metadata missing or invalid: %T", modeParams["errors"])
	}

	required, ok := errorsMode["required"].([]string)
	if !ok {
		t.Fatalf("required type = %T, want []string", errorsMode["required"])
	}
	if len(required) != 1 || required[0] != "what" {
		t.Fatalf("required = %v, want [what]", required)
	}

	paramsRaw, ok := errorsMode["params"]
	if !ok {
		t.Fatal("expected params map in mode metadata")
	}
	params, ok := paramsRaw.(map[string]any)
	if !ok {
		t.Fatalf("params type = %T, want map[string]any", paramsRaw)
	}

	timeoutMetaRaw, ok := params["timeout_ms"]
	if !ok {
		t.Fatal("expected timeout_ms metadata in params")
	}
	timeoutMeta, ok := timeoutMetaRaw.(map[string]any)
	if !ok {
		t.Fatalf("timeout_ms metadata type = %T, want map[string]any", timeoutMetaRaw)
	}
	if timeoutMeta["type"] != "number" {
		t.Fatalf("timeout_ms type = %v, want number", timeoutMeta["type"])
	}
	if timeoutMeta["default"] != "5000" {
		t.Fatalf("timeout_ms default = %v, want 5000", timeoutMeta["default"])
	}
}

func TestBuildCapabilitiesMap_ModeSpecificRequiredParams(t *testing.T) {
	t.Parallel()

	caps := BuildCapabilitiesMap(schema.AllTools())

	assertModeRequiredContains(t, caps, "observe", "command_result", []string{"what", "correlation_id"})
	assertModeRequiredContains(t, caps, "analyze", "dom", []string{"what", "selector"})
	assertModeRequiredContains(t, caps, "interact", "execute_js", []string{"what", "script"})
	assertModeRequiredContains(t, caps, "configure", "get_sequence", []string{"what", "name"})
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

	required := toStringSlice(modeMap["required"])
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
