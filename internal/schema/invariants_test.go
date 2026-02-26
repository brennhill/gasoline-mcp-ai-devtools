// invariants_test.go — Schema invariants that must hold for Claude API compatibility.
package schema

import (
	"fmt"
	"testing"
)

// TestAllToolSchemas_NoTopLevelCombiners ensures no tool input_schema uses
// oneOf, allOf, or anyOf at the top level. The Claude API rejects such schemas.
// See: https://github.com/anthropics/anthropic-sdk-go/issues (input_schema constraint)
func TestAllToolSchemas_NoTopLevelCombiners(t *testing.T) {
	t.Parallel()

	forbidden := []string{"oneOf", "allOf", "anyOf"}

	for _, tool := range AllTools() {
		tool := tool
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

// TestAllToolSchemas_HaveRequiredOrProperties ensures every tool schema has at
// least a "properties" field, catching accidentally empty schemas.
func TestAllToolSchemas_HaveRequiredOrProperties(t *testing.T) {
	t.Parallel()

	for _, tool := range AllTools() {
		tool := tool
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()
			if _, ok := tool.InputSchema["properties"]; !ok {
				t.Errorf("tool %q: input_schema missing top-level %q", tool.Name, "properties")
			}
			if fmt.Sprintf("%s", tool.InputSchema["type"]) != "object" {
				t.Errorf("tool %q: input_schema type = %v, want \"object\"", tool.Name, tool.InputSchema["type"])
			}
		})
	}
}
