// capabilities_test.go — Tests for capability map building.
package configure

import (
	"sort"
	"testing"

	"github.com/dev-console/dev-console/internal/mcp"
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
