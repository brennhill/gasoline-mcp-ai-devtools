// selector_test.go — Tests for selector parsing and DOM action utilities.
package interact

import (
	"testing"
)

func TestParseSelectorForReproduction_TextSelector(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("text=Submit")
	if result["text"] != "Submit" {
		t.Errorf("text=Submit should parse to text:Submit, got %v", result)
	}
}

func TestParseSelectorForReproduction_RoleSelector(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("role=button")
	roleMap, ok := result["role"].(map[string]any)
	if !ok || roleMap["role"] != "button" {
		t.Errorf("role=button should parse to role:{role:button}, got %v", result)
	}
}

func TestParseSelectorForReproduction_LabelSelector(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("label=Name")
	if result["ariaLabel"] != "Name" {
		t.Errorf("label=Name should parse to ariaLabel:Name, got %v", result)
	}
}

func TestParseSelectorForReproduction_AriaLabelSelector(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("aria-label=Close")
	if result["ariaLabel"] != "Close" {
		t.Errorf("aria-label=Close should parse to ariaLabel:Close, got %v", result)
	}
}

func TestParseSelectorForReproduction_PlaceholderSelector(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("placeholder=Email")
	if result["ariaLabel"] != "Email" {
		t.Errorf("placeholder=Email should parse to ariaLabel:Email, got %v", result)
	}
}

func TestParseSelectorForReproduction_UnknownPrefixToCSS(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("data-testid=main")
	if result["cssPath"] != "data-testid=main" {
		t.Errorf("unknown prefix should fall back to cssPath, got %v", result)
	}
}

func TestParseSelectorForReproduction_IDSelector(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("#main")
	if result["id"] != "main" {
		t.Errorf("#main should parse to id:main, got %v", result)
	}
}

func TestParseSelectorForReproduction_ComplexCSSSelector(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("#main .btn")
	if result["cssPath"] != "#main .btn" {
		t.Errorf("complex CSS should parse to cssPath, got %v", result)
	}
}

func TestParseSelectorForReproduction_PlainCSSSelector(t *testing.T) {
	t.Parallel()
	result := ParseSelectorForReproduction("div.container")
	if result["cssPath"] != "div.container" {
		t.Errorf("plain CSS should parse to cssPath, got %v", result)
	}
}

func TestTruncateToLen_Short(t *testing.T) {
	t.Parallel()
	if got := TruncateToLen("short", 10); got != "short" {
		t.Errorf("TruncateToLen('short', 10) = %q, want 'short'", got)
	}
}

func TestTruncateToLen_Long(t *testing.T) {
	t.Parallel()
	if got := TruncateToLen("longstring", 4); got != "long..." {
		t.Errorf("TruncateToLen('longstring', 4) = %q, want 'long...'", got)
	}
}

func TestTruncateToLen_Empty(t *testing.T) {
	t.Parallel()
	if got := TruncateToLen("", 10); got != "" {
		t.Errorf("TruncateToLen('', 10) = %q, want ''", got)
	}
}

func TestExtractElementList_Direct(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"elements": []any{
			map[string]any{"index": float64(0), "selector": "#btn"},
		},
	}
	elems := ExtractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestExtractElementList_NestedResult(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"result": map[string]any{
			"elements": []any{
				map[string]any{"index": float64(0), "selector": "input"},
			},
		},
	}
	elems := ExtractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element from nested result, got %d", len(elems))
	}
}

func TestExtractElementList_DoubleNestedResult(t *testing.T) {
	t.Parallel()
	data := map[string]any{
		"result": map[string]any{
			"result": map[string]any{
				"elements": []any{
					map[string]any{"index": float64(0), "selector": "a"},
				},
			},
		},
	}
	elems := ExtractElementList(data)
	if len(elems) != 1 {
		t.Fatalf("expected 1 element from double-nested result, got %d", len(elems))
	}
}

func TestExtractElementList_NoElements(t *testing.T) {
	t.Parallel()
	data := map[string]any{"status": "ok"}
	elems := ExtractElementList(data)
	if elems != nil {
		t.Errorf("expected nil for data without elements, got %v", elems)
	}
}

func TestDOMPrimitiveActions_Contains(t *testing.T) {
	t.Parallel()
	expected := []string{"click", "type", "select", "check", "get_text", "get_value",
		"get_attribute", "set_attribute", "focus", "scroll_to", "wait_for", "key_press", "paste"}
	for _, action := range expected {
		if !DOMPrimitiveActions[action] {
			t.Errorf("DOMPrimitiveActions missing %q", action)
		}
	}
}

func TestDOMActionToReproType_Mappings(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"click":     "click",
		"type":      "input",
		"select":    "select",
		"check":     "click",
		"key_press": "keypress",
		"scroll_to": "scroll_element",
		"focus":     "focus",
	}
	for action, expected := range cases {
		if got := DOMActionToReproType[action]; got != expected {
			t.Errorf("DOMActionToReproType[%q] = %q, want %q", action, got, expected)
		}
	}
}

func TestValidWorldValues(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"auto", "main", "isolated"} {
		if !ValidWorldValues[v] {
			t.Errorf("ValidWorldValues missing %q", v)
		}
	}
	if ValidWorldValues["invalid"] {
		t.Error("ValidWorldValues should not contain 'invalid'")
	}
}

func TestDOMActionRequiredParams_Structure(t *testing.T) {
	t.Parallel()
	if param, ok := DOMActionRequiredParams["type"]; !ok {
		t.Error("DOMActionRequiredParams missing 'type'")
	} else if param.Field != "text" {
		t.Errorf("type param field = %q, want 'text'", param.Field)
	}

	if param, ok := DOMActionRequiredParams["select"]; !ok {
		t.Error("DOMActionRequiredParams missing 'select'")
	} else if param.Field != "value" {
		t.Errorf("select param field = %q, want 'value'", param.Field)
	}

	if param, ok := DOMActionRequiredParams["get_attribute"]; !ok {
		t.Error("DOMActionRequiredParams missing 'get_attribute'")
	} else if param.Field != "name" {
		t.Errorf("get_attribute param field = %q, want 'name'", param.Field)
	}
}
