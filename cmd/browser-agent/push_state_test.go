package main

import (
	"encoding/json"
	"testing"
)

func TestExtractClientCapabilities_ClaudeCode(t *testing.T) {
	raw := json.RawMessage(`{
		"capabilities": {"sampling": {}},
		"clientInfo": {"name": "claude-code", "version": "1.0"}
	}`)
	caps := extractClientCapabilities(raw)
	if !caps.SupportsSampling {
		t.Fatal("should detect sampling support")
	}
	if caps.ClientName != "claude-code" {
		t.Fatalf("expected claude-code, got %s", caps.ClientName)
	}
}

func TestExtractClientCapabilities_NoSampling(t *testing.T) {
	raw := json.RawMessage(`{
		"capabilities": {},
		"clientInfo": {"name": "cursor"}
	}`)
	caps := extractClientCapabilities(raw)
	if caps.SupportsSampling {
		t.Fatal("should not detect sampling without field")
	}
	if caps.ClientName != "cursor" {
		t.Fatalf("expected cursor, got %s", caps.ClientName)
	}
}

func TestExtractClientCapabilities_Empty(t *testing.T) {
	caps := extractClientCapabilities(json.RawMessage(`{}`))
	if caps.SupportsSampling || caps.SupportsNotifications {
		t.Fatal("empty params should have no capabilities")
	}
}

func TestExtractClientCapabilities_Malformed(t *testing.T) {
	caps := extractClientCapabilities(json.RawMessage(`not json`))
	if caps.ClientName != "" {
		t.Fatal("malformed should return empty")
	}
}

func TestPushState_SetGet(t *testing.T) {
	// Save original and restore after test
	orig := getPushClientCapabilities()
	defer setPushClientCapabilities(orig)

	setPushClientCapabilities(extractClientCapabilities(json.RawMessage(`{
		"capabilities": {"sampling": {}},
		"clientInfo": {"name": "test-client"}
	}`)))

	caps := getPushClientCapabilities()
	if caps.ClientName != "test-client" {
		t.Fatalf("expected test-client, got %s", caps.ClientName)
	}
	if !caps.SupportsSampling {
		t.Fatal("should support sampling")
	}
}
