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

	// Spot-check that well-known actions are present in the canonical enum.
	// Since both 'what' and 'action' reference the same slice, we only check whatEnum.
	mustContain := []string{"navigate", "click", "screenshot", "type", "execute_js", "upload", "auto_dismiss_overlays", "wait_for_stable"}
	whatSet := make(map[string]bool, len(whatEnum))
	for _, v := range whatEnum {
		whatSet[v] = true
	}
	for _, name := range mustContain {
		if !whatSet[name] {
			t.Errorf("interact enum missing expected action %q", name)
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

func TestInteractToolSchema_AutoDismissParam(t *testing.T) {
	t.Parallel()
	tool := InteractToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	adProp, ok := props["auto_dismiss"].(map[string]any)
	if !ok {
		t.Fatal("auto_dismiss param should be in interact schema properties")
	}
	if adProp["type"] != "boolean" {
		t.Errorf("auto_dismiss type = %v, want boolean", adProp["type"])
	}
}

func TestInteractToolSchema_StabilityMsParam(t *testing.T) {
	t.Parallel()
	tool := InteractToolSchema()
	props, ok := tool.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema properties missing")
	}
	smProp, ok := props["stability_ms"].(map[string]any)
	if !ok {
		t.Fatal("stability_ms param should be in interact schema properties")
	}
	if smProp["type"] != "number" {
		t.Errorf("stability_ms type = %v, want number", smProp["type"])
	}
}

func TestInteractActionSpecs_EnumParity(t *testing.T) {
	t.Parallel()

	specs := InteractActionSpecs()
	if len(specs) == 0 {
		t.Fatal("InteractActionSpecs should be non-empty")
	}
	if len(interactActions) != len(specs) {
		t.Fatalf("interactActions/spec count mismatch: actions=%d specs=%d", len(interactActions), len(specs))
	}

	for i, spec := range specs {
		if interactActions[i] != spec.Name {
			t.Fatalf("interact action order mismatch at %d: enum=%q spec=%q", i, interactActions[i], spec.Name)
		}
	}
}

func TestInteractActionSpecs_NoDuplicateNamesOrEmptyHints(t *testing.T) {
	t.Parallel()

	seen := make(map[string]bool, len(interactActionSpecs))
	for i, spec := range interactActionSpecs {
		if spec.Name == "" {
			t.Fatalf("interactActionSpecs[%d] has empty Name", i)
		}
		if seen[spec.Name] {
			t.Fatalf("duplicate interact action name: %q", spec.Name)
		}
		seen[spec.Name] = true
		if spec.Hint == "" {
			t.Fatalf("interact action %q missing capability hint", spec.Name)
		}
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
