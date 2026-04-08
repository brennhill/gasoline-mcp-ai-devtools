// invariants_test.go -- Cross-reference invariants between schema enum values and runtime dispatch handlers.
// Why: Catches drift where a schema enum value has no handler or a handler has no schema enum entry.
// These tests complement tools_schema_parity_test.go by validating directly from schema.AllTools()
// and checking additional properties (uniqueness, no empty values).

package main

import (
	"sort"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/schema"
)

// TestSchemaHandlerCrossRef_AllTools verifies that for each of the 5 tools,
// every 'what' enum value in the schema has a corresponding handler key,
// and every handler key appears in the schema enum (accounting for known
// silent/alias exclusions).
func TestSchemaHandlerCrossRef_AllTools(t *testing.T) {
	t.Parallel()

	// Build handler maps for each tool.
	handlerMaps := map[string]map[string]bool{
		"observe":   toKeySet(observeHandlers),
		"analyze":   toKeySet(analyzeHandlers),
		"generate":  toKeySet(generateHandlers),
		"configure": toKeySet(configureHandlers),
		"interact":  toKeySet(getInteractHandlers()),
	}

	// Known handler keys intentionally omitted from schema enum.
	// These are runtime-only aliases or silently proxied modes.
	silentHandlers := map[string]map[string]bool{
		"observe": observeSilentModes,
		"interact": interactAliasActionSet(),
	}

	for _, tool := range schema.AllTools() {
		tool := tool
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()

			props, ok := tool.InputSchema["properties"].(map[string]any)
			if !ok {
				t.Fatalf("tool %q: schema missing properties", tool.Name)
			}
			whatProp, ok := props["what"].(map[string]any)
			if !ok {
				t.Fatalf("tool %q: schema missing 'what' property", tool.Name)
			}
			enumRaw, ok := whatProp["enum"]
			if !ok {
				t.Fatalf("tool %q: 'what' property missing enum", tool.Name)
			}
			enumValues, err := toStringSlice(enumRaw)
			if err != nil {
				t.Fatalf("tool %q: enum parse error: %v", tool.Name, err)
			}

			handlers := handlerMaps[tool.Name]
			if handlers == nil {
				t.Fatalf("tool %q: no handler map found", tool.Name)
			}

			silent := silentHandlers[tool.Name]

			// Check: every enum value has a handler.
			enumSet := make(map[string]bool, len(enumValues))
			for _, v := range enumValues {
				if v == "" {
					t.Errorf("tool %q: enum contains empty string", tool.Name)
					continue
				}
				if enumSet[v] {
					t.Errorf("tool %q: duplicate enum value %q", tool.Name, v)
					continue
				}
				enumSet[v] = true

				if !handlers[v] {
					t.Errorf("tool %q: schema enum value %q has no handler", tool.Name, v)
				}
			}

			// Check: every handler key is in the enum (or is a known silent mode).
			for key := range handlers {
				if silent[key] {
					continue
				}
				if !enumSet[key] {
					t.Errorf("tool %q: handler key %q not in schema enum", tool.Name, key)
				}
			}
		})
	}
}

// TestSchemaEnumValues_NoDuplicatesNoEmpty ensures enum values are unique and non-empty
// for all tools' 'what' parameter.
func TestSchemaEnumValues_NoDuplicatesNoEmpty(t *testing.T) {
	t.Parallel()

	for _, tool := range schema.AllTools() {
		tool := tool
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()

			props, ok := tool.InputSchema["properties"].(map[string]any)
			if !ok {
				return
			}
			whatProp, ok := props["what"].(map[string]any)
			if !ok {
				return
			}
			enumRaw, ok := whatProp["enum"]
			if !ok {
				return
			}
			enumValues, err := toStringSlice(enumRaw)
			if err != nil {
				t.Fatalf("enum parse error: %v", err)
			}

			seen := make(map[string]bool, len(enumValues))
			for _, v := range enumValues {
				if v == "" {
					t.Error("enum contains empty string value")
				}
				if seen[v] {
					t.Errorf("duplicate enum value: %q", v)
				}
				seen[v] = true
			}
		})
	}
}

// interactAliasActionSet returns the set of interact action names marked as aliases.
func interactAliasActionSet() map[string]bool {
	aliasSet := make(map[string]bool)
	for _, spec := range schema.InteractActionSpecs() {
		if spec.IsAlias {
			aliasSet[spec.Name] = true
		}
	}
	return aliasSet
}

// toKeySet converts a ModeHandler map to a set of keys.
func toKeySet(m map[string]ModeHandler) map[string]bool {
	s := make(map[string]bool, len(m))
	for k := range m {
		s[k] = true
	}
	return s
}

// sortedKeys returns sorted keys of a string-keyed map for deterministic output.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
