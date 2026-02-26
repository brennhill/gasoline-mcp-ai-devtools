// invariants_test.go — Schema invariants that must hold for Claude API compatibility.
package schema

import (
	"encoding/json"
	"testing"
)

// TestAllToolSchemas_NoTopLevelCombiners ensures no tool input_schema uses
// oneOf, allOf, or anyOf at the top level. The Claude API rejects such schemas
// (error: "input_schema does not support oneOf, allOf, or anyOf at the top level").
func TestAllToolSchemas_NoTopLevelCombiners(t *testing.T) {
	t.Parallel()

	forbidden := []string{"oneOf", "allOf", "anyOf"}

	for _, tool := range AllTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			for _, key := range forbidden {
				if _, found := tool.InputSchema[key]; found {
					t.Errorf("tool %q: input_schema has top-level %q — Claude API does not support oneOf/allOf/anyOf at the top level", tool.Name, key)
				}
			}
		})
	}
}

// TestAllToolSchemas_NoNestedCombiners checks that property-level schemas also
// avoid combiners. Nested oneOf/anyOf/allOf in property definitions can cause
// Claude API validation errors in some contexts.
func TestAllToolSchemas_NoNestedCombiners(t *testing.T) {
	t.Parallel()

	forbidden := []string{"oneOf", "allOf", "anyOf", "not"}

	for _, tool := range AllTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			props, ok := tool.InputSchema["properties"].(map[string]any)
			if !ok {
				return
			}
			for propName, propRaw := range props {
				prop, ok := propRaw.(map[string]any)
				if !ok {
					continue
				}
				for _, key := range forbidden {
					if _, found := prop[key]; found {
						t.Errorf("tool %q property %q: has nested %q — avoid combiners in property schemas", tool.Name, propName, key)
					}
				}
			}
		})
	}
}

// TestAllToolSchemas_HavePropertiesAndObjectType ensures every tool schema has
// type:object and a properties field, catching accidentally empty or malformed schemas.
func TestAllToolSchemas_HavePropertiesAndObjectType(t *testing.T) {
	t.Parallel()

	for _, tool := range AllTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			if _, ok := tool.InputSchema["properties"]; !ok {
				t.Errorf("tool %q: input_schema missing top-level \"properties\"", tool.Name)
			}
			typeVal, ok := tool.InputSchema["type"].(string)
			if !ok || typeVal != "object" {
				t.Errorf("tool %q: input_schema type = %T(%v), want string \"object\"", tool.Name, tool.InputSchema["type"], tool.InputSchema["type"])
			}
		})
	}
}

// TestAllToolSchemas_ValidJSON ensures every tool schema serializes to valid JSON.
// This catches type mismatches (e.g. []int instead of []string in required) that
// compile fine but break at the MCP serialization boundary.
func TestAllToolSchemas_ValidJSON(t *testing.T) {
	t.Parallel()

	for _, tool := range AllTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			data, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatalf("tool %q: input_schema failed JSON marshal: %v", tool.Name, err)
			}
			var round map[string]any
			if err := json.Unmarshal(data, &round); err != nil {
				t.Fatalf("tool %q: marshaled schema is not valid JSON: %v", tool.Name, err)
			}
		})
	}
}
