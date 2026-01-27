package main

import (
	"os"
	"strings"
	"testing"
)

// ============================================
// Security Boundary: LLM Trust Model Tests
// ============================================
// These tests enforce the security boundary between LLM tool calls
// (untrusted) and persistent security configuration (trusted).
// See: docs/specs/security-boundary-llm-trust.md

// ============================================
// MCP Mode Detection Tests
// ============================================

func TestIsMCPMode_DetectsEnvironmentVariable(t *testing.T) {
	// Setup
	originalValue := os.Getenv("MCP_MODE")
	defer os.Setenv("MCP_MODE", originalValue)

	// Test: MCP_MODE=1 should enable MCP mode
	os.Setenv("MCP_MODE", "1")
	InitMode()

	if !IsMCPMode() {
		t.Error("MCP_MODE=1 should enable MCP mode")
	}

	if IsInteractiveTerminal() {
		t.Error("MCP mode should never be interactive")
	}
}

func TestIsMCPMode_DefaultsToFalse(t *testing.T) {
	// Setup
	originalValue := os.Getenv("MCP_MODE")
	defer os.Setenv("MCP_MODE", originalValue)

	// Test: No MCP_MODE env var should default to false
	os.Unsetenv("MCP_MODE")
	InitMode()

	if IsMCPMode() {
		t.Error("Without MCP_MODE env var, should not be in MCP mode")
	}
}

// ============================================
// Security Config Modification Guard Tests
// ============================================

func TestAddToWhitelist_BlockedInMCPMode(t *testing.T) {
	// Setup
	originalValue := os.Getenv("MCP_MODE")
	defer os.Setenv("MCP_MODE", originalValue)

	os.Setenv("MCP_MODE", "1")
	InitMode()

	// Test: MCP mode should block whitelist additions
	err := AddToWhitelist("https://evil.xyz")

	if err == nil {
		t.Fatal("AddToWhitelist should return error in MCP mode")
	}

	if !strings.Contains(err.Error(), "human review") {
		t.Errorf("Error should mention human review, got: %s", err.Error())
	}
}

func TestSetMinSeverity_BlockedInMCPMode(t *testing.T) {
	// Setup
	originalValue := os.Getenv("MCP_MODE")
	defer os.Setenv("MCP_MODE", originalValue)

	os.Setenv("MCP_MODE", "1")
	InitMode()

	// Test: MCP mode should block severity threshold changes
	err := SetMinSeverity("critical")

	if err == nil {
		t.Fatal("SetMinSeverity should return error in MCP mode")
	}

	if !strings.Contains(err.Error(), "human review") {
		t.Errorf("Error should mention human review, got: %s", err.Error())
	}
}

func TestClearWhitelist_BlockedInMCPMode(t *testing.T) {
	// Setup
	originalValue := os.Getenv("MCP_MODE")
	defer os.Setenv("MCP_MODE", originalValue)

	os.Setenv("MCP_MODE", "1")
	InitMode()

	// Test: MCP mode should block whitelist clearing
	err := ClearWhitelist()

	if err == nil {
		t.Fatal("ClearWhitelist should return error in MCP mode")
	}

	if !strings.Contains(err.Error(), "human review") {
		t.Errorf("Error should mention human review, got: %s", err.Error())
	}
}

// ============================================
// Session-Only Override Tests
// ============================================

func TestSessionOverride_NotPersisted(t *testing.T) {
	// Clear audit log before test
	ClearSecurityAuditEvents()

	// Create CSP generator
	gen := NewCSPGenerator()

	// Generate CSP with session-only override
	csp := gen.GenerateCSP(CSPParams{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify override applied in this session
	if !strings.Contains(csp.CSPHeader, "temp.xyz") {
		t.Error("Session override should be applied to CSP header")
	}

	// Generate CSP again without override
	csp2 := gen.GenerateCSP(CSPParams{})

	// Verify override NOT persisted
	if strings.Contains(csp2.CSPHeader, "temp.xyz") {
		t.Error("Session override should NOT persist across invocations")
	}
}

func TestSessionOverride_WarningIncluded(t *testing.T) {
	// Clear audit log before test
	ClearSecurityAuditEvents()

	// Create CSP generator
	gen := NewCSPGenerator()

	// Generate CSP with session-only override
	csp := gen.GenerateCSP(CSPParams{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify warning about session-only override
	foundWarning := false
	for _, warning := range csp.Warnings {
		if strings.Contains(warning, "SESSION-ONLY") && strings.Contains(warning, "temp.xyz") {
			foundWarning = true
			break
		}
	}

	if !foundWarning {
		t.Error("CSP should include SESSION-ONLY warning for override")
	}
}

func TestSessionOverride_AuditInfo(t *testing.T) {
	// Clear audit log before test
	ClearSecurityAuditEvents()

	// Create CSP generator
	gen := NewCSPGenerator()

	// Generate CSP with session-only override
	csp := gen.GenerateCSP(CSPParams{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify audit info included
	if csp.Audit == nil {
		t.Fatal("CSP should include audit information")
	}

	if len(csp.Audit.SessionOverrides) == 0 {
		t.Error("Audit should list session overrides")
	}

	if csp.Audit.OverrideSource != "mcp_tool_parameter" {
		t.Errorf("Audit should track override source, got: %s", csp.Audit.OverrideSource)
	}
}

// ============================================
// Config File Immutability Tests
// ============================================

func TestMCPCalls_DoNotModifyConfigFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := tmpDir + "/security.json"
	initialConfig := `{"version": "1.0", "whitelisted_origins": []}`

	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Setup MCP mode
	originalValue := os.Getenv("MCP_MODE")
	defer os.Setenv("MCP_MODE", originalValue)
	os.Setenv("MCP_MODE", "1")
	InitMode()

	// Override config path
	originalConfigPath := getSecurityConfigPath()
	setSecurityConfigPath(configPath)
	defer setSecurityConfigPath(originalConfigPath)

	// Attempt to modify config via MCP tool (should fail)
	_ = AddToWhitelist("https://evil.xyz")

	// Read config file
	configData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Verify config unchanged
	if string(configData) != initialConfig {
		t.Error("MCP calls should not modify security config file")
	}
}

// ============================================
// Audit Logging Tests
// ============================================

func TestAuditLog_RecordsSecurityDecisions(t *testing.T) {
	// Clear audit log before test
	ClearSecurityAuditEvents()

	// Create CSP generator with audit logging
	gen := NewCSPGenerator()

	// Generate CSP with override
	_ = gen.GenerateCSP(CSPParams{
		WhitelistOverride: []string{"https://temp.xyz"},
	})

	// Verify audit log entry created
	events := GetSecurityAuditEvents()

	if len(events) == 0 {
		t.Fatal("Expected audit log entry for CSP generation with override")
	}

	// Find the CSP generation event
	foundEvent := false
	for _, event := range events {
		if event.Action == "whitelist_override" && event.Origin == "https://temp.xyz" {
			foundEvent = true

			// Verify required fields
			if event.Persistent {
				t.Error("Session override should be marked as non-persistent")
			}
			if event.Source != "mcp" {
				t.Errorf("Source should be 'mcp', got: %s", event.Source)
			}
			if event.Timestamp.IsZero() {
				t.Error("Timestamp should be set")
			}

			break
		}
	}

	if !foundEvent {
		t.Error("Audit log should contain whitelist_override event")
	}
}
