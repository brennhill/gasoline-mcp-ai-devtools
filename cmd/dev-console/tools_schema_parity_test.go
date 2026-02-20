// tools_schema_parity_test.go — Enforces parity between tool schema enums and runtime dispatch handlers.
package main

import (
	"fmt"
	"sort"
	"testing"
)

func TestSchemaParity_AnalyzeWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	schemaModes := mustToolEnumValues(t, h.ToolsList(), "analyze", "what")
	runtimeModes := sortedKeysAnalyzeHandlers()

	assertSameStringSet(t, "analyze.what enum vs analyzeHandlers", schemaModes, runtimeModes)
}

func TestSchemaParity_GenerateWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	schemaFormats := mustToolEnumValues(t, h.ToolsList(), "generate", "what")
	runtimeFormats := sortedKeysGenerateHandlers()
	assertSameStringSet(t, "generate.what enum vs generateHandlers", schemaFormats, runtimeFormats)
}

func TestSchemaParity_ConfigureWhatEnumMatchesHandlers(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)
	schemaActions := mustToolEnumValues(t, h.ToolsList(), "configure", "what")
	runtimeActions := sortedKeysConfigureHandlers()
	assertSameStringSet(t, "configure.what enum vs configureHandlers", schemaActions, runtimeActions)
}

func TestSchemaParity_InteractWhatEnumMatchesDispatch(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	schemaActions := mustToolEnumValues(t, h.ToolsList(), "interact", "what")
	runtimeActions := sortedInteractRuntimeActions(h)

	assertSameStringSet(t, "interact.what enum vs interact runtime actions", schemaActions, runtimeActions)
}

func sortedKeysGenerateHandlers() []string {
	keys := make([]string, 0, len(generateHandlers))
	for format := range generateHandlers {
		keys = append(keys, format)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysConfigureHandlers() []string {
	keys := make([]string, 0, len(configureHandlers))
	for action := range configureHandlers {
		keys = append(keys, action)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysAnalyzeHandlers() []string {
	keys := make([]string, 0, len(analyzeHandlers))
	for mode := range analyzeHandlers {
		keys = append(keys, mode)
	}
	sort.Strings(keys)
	return keys
}

func sortedInteractRuntimeActions(h *ToolHandler) []string {
	actions := make(map[string]bool)
	for action := range h.interactDispatch() {
		actions[action] = true
	}
	for action := range domPrimitiveActions {
		actions[action] = true
	}
	keys := make([]string, 0, len(actions))
	for action := range actions {
		keys = append(keys, action)
	}
	sort.Strings(keys)
	return keys
}

func mustToolEnumValues(t *testing.T, tools []MCPTool, toolName, propertyName string) []string {
	t.Helper()

	var tool *MCPTool
	for i := range tools {
		if tools[i].Name == toolName {
			tool = &tools[i]
			break
		}
	}
	if tool == nil {
		t.Fatalf("tool %q not found in ToolsList", toolName)
	}

	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("tool %q schema missing properties", toolName)
	}

	prop, ok := props[propertyName].(map[string]any)
	if !ok {
		t.Fatalf("tool %q schema missing property %q", toolName, propertyName)
	}

	enumRaw, ok := prop["enum"]
	if !ok {
		t.Fatalf("tool %q property %q missing enum", toolName, propertyName)
	}

	enum, err := toStringSlice(enumRaw)
	if err != nil {
		t.Fatalf("tool %q property %q enum parse failed: %v", toolName, propertyName, err)
	}

	sort.Strings(enum)
	return enum
}

func toStringSlice(v any) ([]string, error) {
	switch vv := v.(type) {
	case []string:
		out := make([]string, len(vv))
		copy(out, vv)
		return out, nil
	case []any:
		out := make([]string, 0, len(vv))
		for i, item := range vv {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("enum[%d] is %T, want string", i, item)
			}
			out = append(out, s)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported enum type %T", v)
	}
}

func assertSameStringSet(t *testing.T, label string, got, want []string) {
	t.Helper()

	gotSet := make(map[string]bool, len(got))
	wantSet := make(map[string]bool, len(want))

	for _, v := range got {
		gotSet[v] = true
	}
	for _, v := range want {
		wantSet[v] = true
	}

	missingInSchema := make([]string, 0)
	missingInRuntime := make([]string, 0)

	for v := range wantSet {
		if !gotSet[v] {
			missingInSchema = append(missingInSchema, v)
		}
	}
	for v := range gotSet {
		if !wantSet[v] {
			missingInRuntime = append(missingInRuntime, v)
		}
	}

	sort.Strings(missingInSchema)
	sort.Strings(missingInRuntime)

	if len(missingInSchema) > 0 || len(missingInRuntime) > 0 {
		t.Fatalf(
			"%s mismatch\nmissing_in_schema=%v\nmissing_in_runtime=%v\ngot=%v\nwant=%v",
			label,
			missingInSchema,
			missingInRuntime,
			got,
			want,
		)
	}
}
