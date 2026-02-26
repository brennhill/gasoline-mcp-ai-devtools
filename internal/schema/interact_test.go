package schema

import "testing"

func TestInteractToolSchema_DispatchAcceptsWhatOrAction(t *testing.T) {
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

	if _, hasRequired := tool.InputSchema["required"]; hasRequired {
		t.Fatal("interact schema should not hard-require 'what' when 'action' alias is supported")
	}

	anyOfRaw, ok := tool.InputSchema["anyOf"]
	if !ok {
		t.Fatal("interact schema missing anyOf dispatch requirement")
	}
	anyOf, ok := anyOfRaw.([]map[string]any)
	if !ok {
		t.Fatalf("interact anyOf type = %T, want []map[string]any", anyOfRaw)
	}
	if len(anyOf) != 2 {
		t.Fatalf("interact anyOf length = %d, want 2", len(anyOf))
	}

	firstRequired := toSchemaStringSlice(t, anyOf[0]["required"])
	secondRequired := toSchemaStringSlice(t, anyOf[1]["required"])
	if len(firstRequired) != 1 || firstRequired[0] != "what" {
		t.Fatalf("first anyOf required = %v, want [what]", firstRequired)
	}
	if len(secondRequired) != 1 || secondRequired[0] != "action" {
		t.Fatalf("second anyOf required = %v, want [action]", secondRequired)
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
