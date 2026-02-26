package schema

import "testing"

func TestInteractToolSchema_RequiresWhat_ActionIsRuntimeAlias(t *testing.T) {
	t.Parallel()

	tool := InteractToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing properties")
	}

	whatProp, ok := props["what"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing 'what' property")
	}
	actionProp, ok := props["action"].(map[string]any)
	if !ok {
		t.Fatal("interact schema missing 'action' property")
	}

	whatEnum := toSchemaStringSlice(t, whatProp["enum"])
	actionEnum := toSchemaStringSlice(t, actionProp["enum"])
	if len(whatEnum) == 0 || len(actionEnum) == 0 {
		t.Fatal("interact dispatch enums must be non-empty")
	}
	if len(whatEnum) != len(actionEnum) {
		t.Fatalf("enum length mismatch: what=%d action=%d", len(whatEnum), len(actionEnum))
	}
	for i := range whatEnum {
		if whatEnum[i] != actionEnum[i] {
			t.Fatalf("enum mismatch at %d: what=%q action=%q", i, whatEnum[i], actionEnum[i])
		}
	}

	// Claude API forbids oneOf/allOf/anyOf at the top level of input_schema.
	// 'what' is required; 'action' is a runtime alias handled by the server.
	if _, hasAnyOf := tool.InputSchema["anyOf"]; hasAnyOf {
		t.Fatal("interact schema must not use top-level anyOf (Claude API limitation)")
	}

	required := toSchemaStringSlice(t, tool.InputSchema["required"])
	if len(required) != 1 || required[0] != "what" {
		t.Fatalf("interact schema required = %v, want [what]", required)
	}
}

func toSchemaStringSlice(t *testing.T, raw any) []string {
	t.Helper()
	switch v := raw.(type) {
	case []string:
		out := make([]string, len(v))
		copy(out, v)
		return out
	case []any:
		out := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				t.Fatalf("string slice item[%d] type = %T, want string", i, item)
			}
			out = append(out, s)
		}
		return out
	default:
		t.Fatalf("string slice type = %T, want []string or []any", raw)
		return nil
	}
}
