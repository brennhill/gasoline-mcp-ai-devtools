package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleBrowserActionNavigate_InsecureSchemeRequiresSecurityMode(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"navigate","url":"gasoline-insecure://https://example.com"}`)
	if !ok {
		t.Fatal("navigate should return result")
	}
	if !result.IsError {
		t.Fatal("navigate with gasoline-insecure scheme should fail when security mode is normal")
	}
	if text := firstText(result); !strings.Contains(strings.ToLower(text), "security_mode") {
		t.Fatalf("error should mention security_mode enablement, got: %s", text)
	}

	if pq := env.capture.GetLastPendingQuery(); pq != nil {
		t.Fatalf("no pending query should be queued on validation failure, got type=%s", pq.Type)
	}
}

func TestHandleBrowserActionNavigate_RewritesGasolineInsecureURL(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	enable := callConfigureRaw(env.handler, `{"what":"security_mode","mode":"insecure_proxy","confirm":true}`)
	enableResult := parseToolResult(t, enable)
	if enableResult.IsError {
		t.Fatalf("security mode enable failed: %s", firstText(enableResult))
	}

	result, ok := env.callInteract(t, `{"what":"navigate","url":"gasoline-insecure://https://example.com/path?q=1#frag"}`)
	if !ok {
		t.Fatal("navigate should return result")
	}
	if result.IsError {
		t.Fatalf("navigate should succeed with insecure mode enabled, got: %s", firstText(result))
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("navigate should create pending query")
	}
	if pq.Type != "browser_action" {
		t.Fatalf("pending query type = %q, want browser_action", pq.Type)
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	if got, _ := params["action"].(string); got != "navigate" {
		t.Fatalf("action = %q, want navigate", got)
	}
	urlValue, _ := params["url"].(string)
	if !strings.Contains(urlValue, "http://127.0.0.1:7890/insecure-proxy?target=") {
		t.Fatalf("rewritten url = %q, want insecure proxy url", urlValue)
	}
	if !strings.Contains(urlValue, "https%3A%2F%2Fexample.com%2Fpath%3Fq%3D1%23frag") {
		t.Fatalf("rewritten url missing encoded target, got: %q", urlValue)
	}
}

func TestHandleBrowserActionNewTab_RewritesGasolineInsecureURL(t *testing.T) {
	t.Parallel()
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	enable := callConfigureRaw(env.handler, `{"what":"security_mode","mode":"insecure_proxy","confirm":true}`)
	if parseToolResult(t, enable).IsError {
		t.Fatalf("security mode enable failed: %s", firstText(parseToolResult(t, enable)))
	}

	result, ok := env.callInteract(t, `{"what":"new_tab","url":"gasoline-insecure://https://example.org"}`)
	if !ok {
		t.Fatal("new_tab should return result")
	}
	if result.IsError {
		t.Fatalf("new_tab should succeed with insecure mode enabled, got: %s", firstText(result))
	}

	pq := env.capture.GetLastPendingQuery()
	if pq == nil {
		t.Fatal("new_tab should create pending query")
	}

	var params map[string]any
	if err := json.Unmarshal(pq.Params, &params); err != nil {
		t.Fatalf("unmarshal params: %v", err)
	}
	urlValue, _ := params["url"].(string)
	if !strings.Contains(urlValue, "http://127.0.0.1:7890/insecure-proxy?target=https%3A%2F%2Fexample.org") {
		t.Fatalf("new_tab rewritten url = %q, want insecure proxy URL", urlValue)
	}
}
