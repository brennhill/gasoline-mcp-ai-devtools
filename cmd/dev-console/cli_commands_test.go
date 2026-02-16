// cli_commands_test.go — Tests for uncovered CLI argument parser branches.
// Core tests are in cli_test.go; this file covers remaining edge cases.
package main

import (
	"path/filepath"
	"testing"
)

// ============================================
// parseObserveArgs — uncovered flags
// ============================================

func TestParseObserveArgs_StatusMaxAndLastN(t *testing.T) {
	t.Parallel()

	result, err := parseObserveArgs("network_bodies", []string{
		"--status-max", "499",
		"--last-n", "10",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["status_max"] != 499 {
		t.Fatalf("status_max = %v, want 499", result["status_max"])
	}
	if result["last_n"] != 10 {
		t.Fatalf("last_n = %v, want 10", result["last_n"])
	}
}

func TestParseObserveArgs_BodyFilters(t *testing.T) {
	t.Parallel()

	result, err := parseObserveArgs("network_bodies", []string{
		"--body-key", "id",
		"--body-path", "data.items[0]",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["body_key"] != "id" {
		t.Fatalf("body_key = %v, want id", result["body_key"])
	}
	if result["body_path"] != "data.items[0]" {
		t.Fatalf("body_path = %v, want data.items[0]", result["body_path"])
	}
}

// ============================================
// parseGenerateArgs — uncovered flags
// ============================================

func TestParseGenerateArgs_MethodAndBaseURL(t *testing.T) {
	t.Parallel()

	result, err := parseGenerateArgs("har", []string{
		"--method", "POST",
		"--base-url", "http://localhost:3000",
		"--url", "https://api.example.com",
		"--include-screenshots",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["method"] != "POST" {
		t.Fatalf("method = %v, want POST", result["method"])
	}
	if result["base_url"] != "http://localhost:3000" {
		t.Fatalf("base_url = %v, want http://localhost:3000", result["base_url"])
	}
	if result["url"] != "https://api.example.com" {
		t.Fatalf("url = %v, want https://api.example.com", result["url"])
	}
	if result["include_screenshots"] != true {
		t.Fatal("include_screenshots should be true")
	}
}

// ============================================
// parseConfigureArgs — uncovered flags
// ============================================

func TestParseConfigureArgs_DataPlainString(t *testing.T) {
	t.Parallel()

	result, err := parseConfigureArgs("store", []string{"--data", "not-json"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["data"] != "not-json" {
		t.Fatalf("data = %v, want not-json (plain string fallback)", result["data"])
	}
}

func TestParseConfigureArgs_RemainingFlags(t *testing.T) {
	t.Parallel()

	result, err := parseConfigureArgs("noise_rule", []string{
		"--rule-id", "rule-1",
		"--store-action", "save",
		"--selector", "#app",
		"--namespace", "test-ns",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	checks := map[string]string{
		"rule_id":      "rule-1",
		"store_action": "save",
		"selector":     "#app",
		"namespace":    "test-ns",
	}
	for k, want := range checks {
		if got, _ := result[k].(string); got != want {
			t.Errorf("result[%q] = %q, want %q", k, got, want)
		}
	}
}

// ============================================
// parseInteractArgs — uncovered flags
// ============================================

func TestParseInteractArgs_TimeoutAndSubtitle(t *testing.T) {
	t.Parallel()

	result, err := parseInteractArgs("click", []string{
		"--selector", "#btn",
		"--timeout-ms", "5000",
		"--subtitle", "Clicking button",
		"--reason", "testing flow",
		"--name", "submit",
		"--value", "42",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["timeout_ms"] != 5000 {
		t.Fatalf("timeout_ms = %v, want 5000", result["timeout_ms"])
	}
	if result["subtitle"] != "Clicking button" {
		t.Fatalf("subtitle = %v, want Clicking button", result["subtitle"])
	}
	if result["reason"] != "testing flow" {
		t.Fatalf("reason = %v, want testing flow", result["reason"])
	}
	if result["name"] != "submit" {
		t.Fatalf("name = %v, want submit", result["name"])
	}
	if result["value"] != "42" {
		t.Fatalf("value = %v, want 42", result["value"])
	}
}

func TestParseInteractArgs_FilePathRelative(t *testing.T) {
	t.Parallel()

	result, err := parseInteractArgs("upload", []string{
		"--selector", "input[type=file]",
		"--file-path", "test.png",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	fp, _ := result["file_path"].(string)
	if !filepath.IsAbs(fp) {
		t.Fatalf("relative file_path should be resolved to absolute, got: %s", fp)
	}
}

func TestParseInteractArgs_FilePathAbsolute(t *testing.T) {
	t.Parallel()

	result, err := parseInteractArgs("upload", []string{
		"--selector", "input[type=file]",
		"--file-path", "/tmp/test.png",
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result["file_path"] != "/tmp/test.png" {
		t.Fatalf("file_path = %v, want /tmp/test.png", result["file_path"])
	}
}

// ============================================
// cliParseFlagInt — non-numeric edge case
// ============================================

func TestCliParseFlagInt_NonNumeric(t *testing.T) {
	t.Parallel()

	n, ok, _ := cliParseFlagInt([]string{"--limit", "abc"}, "--limit")
	if ok {
		t.Fatal("non-numeric value should return ok=false")
	}
	if n != 0 {
		t.Fatalf("n = %d, want 0", n)
	}
}

func TestCliParseFlagInt_Missing(t *testing.T) {
	t.Parallel()

	_, ok, _ := cliParseFlagInt([]string{"--other", "5"}, "--limit")
	if ok {
		t.Fatal("missing flag should return ok=false")
	}
}

// ============================================
// cliParseFlag / cliParseFlagBool — missing flag
// ============================================

func TestCliParseFlag_Missing(t *testing.T) {
	t.Parallel()

	val, remaining := cliParseFlag([]string{"--other", "x"}, "--url")
	if val != "" {
		t.Fatalf("missing flag should return empty, got %q", val)
	}
	if len(remaining) != 2 {
		t.Fatalf("remaining should be unchanged, len = %d", len(remaining))
	}
}

func TestCliParseFlagBool_Missing(t *testing.T) {
	t.Parallel()

	ok, remaining := cliParseFlagBool([]string{"--other"}, "--clear")
	if ok {
		t.Fatal("missing bool flag should return false")
	}
	if len(remaining) != 1 {
		t.Fatalf("remaining should be unchanged, len = %d", len(remaining))
	}
}
