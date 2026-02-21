// tools_interact_dom_test.go — Tests for DOM interaction primitives.
//
// Tests verify parameter validation, pilot gating, and queuing behavior
// for the 13 DOM primitive actions.
//
// Run: go test ./cmd/dev-console -run "TestDOMPrimitive" -v
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================
// Parameter Validation: Missing Selector
// All 11 selector-requiring actions must error when selector is missing
// ============================================

func TestDOMPrimitive_MissingSelector(t *testing.T) {
	env := newInteractTestEnv(t)

	actions := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click"}`},
		{"type", `{"what":"type","text":"hello"}`},
		{"paste", `{"what":"paste","text":"hello"}`},
		{"select", `{"what":"select","value":"opt1"}`},
		{"check", `{"what":"check"}`},
		{"get_text", `{"what":"get_text"}`},
		{"get_value", `{"what":"get_value"}`},
		{"get_attribute", `{"what":"get_attribute","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","name":"href","value":"#"}`},
		{"focus", `{"what":"focus"}`},
		{"scroll_to", `{"what":"scroll_to"}`},
		{"wait_for", `{"what":"wait_for"}`},
		{"key_press", `{"what":"key_press","text":"Enter"}`},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			result, ok := env.callInteract(t, tc.args)
			if !ok {
				t.Fatalf("%s without selector should return result", tc.action)
			}

			if !result.IsError {
				t.Errorf("%s without selector MUST return isError:true", tc.action)
			}

			if len(result.Content) > 0 {
				text := strings.ToLower(result.Content[0].Text)
				if !strings.Contains(text, "selector") {
					t.Errorf("%s error should mention selector\nGot: %s", tc.action, result.Content[0].Text)
				}
			}
		})
	}
}

// ============================================
// Parameter Validation: Action-specific required params
// ============================================

func TestDOMPrimitive_Type_MissingText(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"type","selector":"#input"}`)
	if !ok {
		t.Fatal("type without text should return result")
	}

	if !result.IsError {
		t.Error("type without text MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "text") {
			t.Errorf("error should mention text parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_Select_MissingValue(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"select","selector":"#dropdown"}`)
	if !ok {
		t.Fatal("select without value should return result")
	}

	if !result.IsError {
		t.Error("select without value MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "value") {
			t.Errorf("error should mention value parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_GetAttribute_MissingName(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"get_attribute","selector":"#link"}`)
	if !ok {
		t.Fatal("get_attribute without name should return result")
	}

	if !result.IsError {
		t.Error("get_attribute without name MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "name") {
			t.Errorf("error should mention name parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_SetAttribute_MissingName(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"set_attribute","selector":"#link","value":"#"}`)
	if !ok {
		t.Fatal("set_attribute without name should return result")
	}

	if !result.IsError {
		t.Error("set_attribute without name MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "name") {
			t.Errorf("error should mention name parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

// ============================================
// Pilot Disabled: All 13 DOM actions check pilot
// ============================================

func TestDOMPrimitive_AllActions_PilotDisabled(t *testing.T) {
	env := newInteractTestEnv(t)

	// All DOM actions with valid params — pilot is disabled by default
	actions := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click","selector":"#btn"}`},
		{"type", `{"what":"type","selector":"#input","text":"hello"}`},
		{"paste", `{"what":"paste","selector":"#editor","text":"hello"}`},
		{"select", `{"what":"select","selector":"#dropdown","value":"opt1"}`},
		{"check", `{"what":"check","selector":"#checkbox"}`},
		{"get_text", `{"what":"get_text","selector":"#el"}`},
		{"get_value", `{"what":"get_value","selector":"#input"}`},
		{"get_attribute", `{"what":"get_attribute","selector":"#link","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","selector":"#el","name":"class","value":"active"}`},
		{"focus", `{"what":"focus","selector":"#input"}`},
		{"scroll_to", `{"what":"scroll_to","selector":"#section"}`},
		{"wait_for", `{"what":"wait_for","selector":"#loading"}`},
		{"key_press", `{"what":"key_press","selector":"#input","text":"Enter"}`},
		{"open_composer", `{"what":"open_composer"}`},
		{"submit_active_composer", `{"what":"submit_active_composer"}`},
		{"confirm_top_dialog", `{"what":"confirm_top_dialog"}`},
		{"dismiss_top_overlay", `{"what":"dismiss_top_overlay"}`},
		{"list_interactive", `{"what":"list_interactive"}`},
	}

	for _, tc := range actions {
		t.Run(tc.action, func(t *testing.T) {
			result, ok := env.callInteract(t, tc.args)
			if !ok {
				t.Fatalf("%s should return result", tc.action)
			}

			if !result.IsError {
				t.Errorf("%s with pilot disabled should return isError:true", tc.action)
			}

			if len(result.Content) > 0 {
				text := strings.ToLower(result.Content[0].Text)
				if !strings.Contains(text, "pilot") {
					t.Errorf("%s error should mention pilot\nGot: %s", tc.action, result.Content[0].Text)
				}
			}
		})
	}
}

// ============================================
// Queued Response: Valid params + pilot enabled returns correlation_id
// NOTE: Skipped — requires pilot enabled which needs extension connection.
// Covered by shell UAT.
// ============================================

func TestDOMPrimitive_Click_ReturnsCorrelationID(t *testing.T) {
	t.Skip("Skipped: requires pilot enabled; covered by shell UAT")
}

func TestDOMPrimitive_ListInteractive_ReturnsCorrelationID(t *testing.T) {
	t.Skip("Skipped: requires pilot enabled; covered by shell UAT")
}

// ============================================
// list_interactive: Does NOT require selector
// ============================================

func TestDOMPrimitive_ListInteractive_NoSelectorNeeded(t *testing.T) {
	env := newInteractTestEnv(t)

	// list_interactive without selector should NOT get a "selector missing" error
	// It should reach the pilot check instead
	result, ok := env.callInteract(t, `{"what":"list_interactive"}`)
	if !ok {
		t.Fatal("list_interactive should return result")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if strings.Contains(text, "selector") {
			t.Errorf("list_interactive should NOT require selector\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_IntentActions_NoSelectorNeeded(t *testing.T) {
	env := newInteractTestEnv(t)

	actions := []string{
		"open_composer",
		"submit_active_composer",
		"confirm_top_dialog",
		"dismiss_top_overlay",
	}

	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			result, ok := env.callInteract(t, `{"what":"`+action+`"}`)
			if !ok {
				t.Fatalf("%s should return result", action)
			}

			if len(result.Content) > 0 {
				text := strings.ToLower(result.Content[0].Text)
				if strings.Contains(text, "selector") {
					t.Errorf("%s should NOT require selector\nGot: %s", action, result.Content[0].Text)
				}
			}
		})
	}
}

// ============================================
// Parameter Validation: paste action
// ============================================

func TestDOMPrimitive_Paste_MissingText(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"paste","selector":"#editor"}`)
	if !ok {
		t.Fatal("paste without text should return result")
	}

	if !result.IsError {
		t.Error("paste without text MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "text") {
			t.Errorf("error should mention text parameter\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_Paste_MissingSelector(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"paste","text":"hello"}`)
	if !ok {
		t.Fatal("paste without selector should return result")
	}

	if !result.IsError {
		t.Error("paste without selector MUST return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "selector") {
			t.Errorf("error should mention selector\nGot: %s", result.Content[0].Text)
		}
	}
}

func TestDOMPrimitive_Paste_PilotDisabled(t *testing.T) {
	env := newInteractTestEnv(t)

	result, ok := env.callInteract(t, `{"what":"paste","selector":"#editor","text":"hello"}`)
	if !ok {
		t.Fatal("paste should return result")
	}

	if !result.IsError {
		t.Error("paste with pilot disabled should return isError:true")
	}

	if len(result.Content) > 0 {
		text := strings.ToLower(result.Content[0].Text)
		if !strings.Contains(text, "pilot") {
			t.Errorf("paste error should mention pilot\nGot: %s", result.Content[0].Text)
		}
	}
}

// ============================================
// Safety Net: All DOM actions don't panic
// ============================================

func TestDOMPrimitive_AllActions_NoPanic(t *testing.T) {
	env := newInteractTestEnv(t)

	allActions := []struct {
		action string
		args   string
	}{
		{"click", `{"what":"click","selector":"#btn"}`},
		{"type", `{"what":"type","selector":"#input","text":"hello"}`},
		{"paste", `{"what":"paste","selector":"#editor","text":"hello"}`},
		{"select", `{"what":"select","selector":"#dropdown","value":"opt1"}`},
		{"check", `{"what":"check","selector":"#checkbox"}`},
		{"get_text", `{"what":"get_text","selector":"#el"}`},
		{"get_value", `{"what":"get_value","selector":"#input"}`},
		{"get_attribute", `{"what":"get_attribute","selector":"#link","name":"href"}`},
		{"set_attribute", `{"what":"set_attribute","selector":"#el","name":"class","value":"active"}`},
		{"focus", `{"what":"focus","selector":"#input"}`},
		{"scroll_to", `{"what":"scroll_to","selector":"#section"}`},
		{"wait_for", `{"what":"wait_for","selector":"#loading"}`},
		{"key_press", `{"what":"key_press","selector":"#input","text":"Enter"}`},
		{"open_composer", `{"what":"open_composer"}`},
		{"submit_active_composer", `{"what":"submit_active_composer"}`},
		{"confirm_top_dialog", `{"what":"confirm_top_dialog"}`},
		{"dismiss_top_overlay", `{"what":"dismiss_top_overlay"}`},
		{"list_interactive", `{"what":"list_interactive"}`},
	}

	for _, tc := range allActions {
		t.Run(tc.action, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("interact(%s) PANICKED: %v", tc.action, r)
				}
			}()

			args := json.RawMessage(tc.args)
			req := JSONRPCRequest{JSONRPC: "2.0", ID: 1}
			resp := env.handler.toolInteract(req, args)

			if resp.Result == nil && resp.Error == nil {
				t.Errorf("interact(%s) returned nil response", tc.action)
			}
		})
	}
}
