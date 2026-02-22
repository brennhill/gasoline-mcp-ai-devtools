package main

import (
	"strings"
	"testing"
)

func TestToolsConfigureSecurityMode_DefaultStatus(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"security_mode"}`)
	result := parseToolResult(t, resp)
	if result.IsError {
		t.Fatalf("security_mode status should succeed, got: %s", firstText(result))
	}

	data := extractResultJSON(t, result)
	if got, _ := data["security_mode"].(string); got != "normal" {
		t.Fatalf("security_mode = %q, want normal", got)
	}
	if got, ok := data["production_parity"].(bool); !ok || !got {
		t.Fatalf("production_parity = %v, want true", data["production_parity"])
	}
}

func TestToolsConfigureSecurityMode_EnableRequiresConfirm(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	resp := callConfigureRaw(h, `{"what":"security_mode","mode":"insecure_proxy"}`)
	result := parseToolResult(t, resp)
	if !result.IsError {
		t.Fatal("enabling insecure_proxy without confirm should fail")
	}
	if text := firstText(result); text == "" || !containsAll(text, "confirm", "insecure_proxy") {
		t.Fatalf("error should mention confirmation and mode, got: %s", text)
	}
}

func TestToolsConfigureSecurityMode_EnableAndDisable(t *testing.T) {
	t.Parallel()
	h, _, _ := makeToolHandler(t)

	enableResp := callConfigureRaw(h, `{"what":"security_mode","mode":"insecure_proxy","confirm":true}`)
	enableResult := parseToolResult(t, enableResp)
	if enableResult.IsError {
		t.Fatalf("enable insecure_proxy should succeed, got: %s", firstText(enableResult))
	}
	enableData := extractResultJSON(t, enableResult)
	if got, _ := enableData["security_mode"].(string); got != "insecure_proxy" {
		t.Fatalf("enabled security_mode = %q, want insecure_proxy", got)
	}
	if got, ok := enableData["production_parity"].(bool); !ok || got {
		t.Fatalf("enabled production_parity = %v, want false", enableData["production_parity"])
	}

	statusResp := callConfigureRaw(h, `{"what":"security_mode"}`)
	statusResult := parseToolResult(t, statusResp)
	if statusResult.IsError {
		t.Fatalf("security_mode status should succeed, got: %s", firstText(statusResult))
	}
	statusData := extractResultJSON(t, statusResult)
	if got, _ := statusData["security_mode"].(string); got != "insecure_proxy" {
		t.Fatalf("status security_mode = %q, want insecure_proxy", got)
	}

	disableResp := callConfigureRaw(h, `{"what":"security_mode","mode":"normal"}`)
	disableResult := parseToolResult(t, disableResp)
	if disableResult.IsError {
		t.Fatalf("disable insecure mode should succeed, got: %s", firstText(disableResult))
	}
	disableData := extractResultJSON(t, disableResult)
	if got, _ := disableData["security_mode"].(string); got != "normal" {
		t.Fatalf("disabled security_mode = %q, want normal", got)
	}
	if got, ok := disableData["production_parity"].(bool); !ok || !got {
		t.Fatalf("disabled production_parity = %v, want true", disableData["production_parity"])
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !strings.Contains(strings.ToLower(s), strings.ToLower(p)) {
			return false
		}
	}
	return true
}
