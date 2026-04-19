// Purpose: Tests for security boundary enforcement and isolation.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"os"
	"strings"
	"testing"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
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
	// Test: MCP_MODE=1 should enable MCP mode
	t.Setenv("MCP_MODE", "1")
	InitMode()

	if !IsMCPMode() {
		t.Error("MCP_MODE=1 should enable MCP mode")
	}

	if IsInteractiveTerminal() {
		t.Error("MCP mode should never be interactive")
	}
}

func TestIsMCPMode_DefaultsToFalse(t *testing.T) {
	// Test: No MCP_MODE env var should default to false
	t.Setenv("MCP_MODE", "")
	InitMode()

	if IsMCPMode() {
		t.Error("Without MCP_MODE env var, should not be in MCP mode")
	}
}

// ============================================
// Security Config Modification Guard Tests
// ============================================

func TestAddToWhitelist_BlockedInMCPMode(t *testing.T) {
	t.Setenv("MCP_MODE", "1")
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
	t.Setenv("MCP_MODE", "1")
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
	t.Setenv("MCP_MODE", "1")
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
	t.Setenv("MCP_MODE", "1")
	InitMode()

	// Override config path
	originalConfigPath := getSecurityConfigPath()
	setSecurityConfigPath(configPath)
	defer setSecurityConfigPath(originalConfigPath)

	// Attempt to modify config via MCP tool (should fail)
	_ = AddToWhitelist("https://evil.xyz")

	// Read config file
	configData, err := os.ReadFile(configPath) // nosemgrep: go_filesystem_rule-fileread -- test helper reads fixture/output file
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Verify config unchanged
	if string(configData) != initialConfig {
		t.Error("MCP calls should not modify security config file")
	}
}

// ============================================
// Network Security Check Wiring Tests
// ============================================

func TestScan_NetworkCheckDetectsSuspiciousTLD(t *testing.T) {
	scanner := NewSecurityScanner()

	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "https://cdn-analytics.xyz/tracker.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	if len(result.Findings) == 0 {
		t.Fatal("Expected network findings for suspicious TLD .xyz")
	}

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && strings.Contains(f.Title, ".xyz") {
			found = true
			if f.Severity != "medium" {
				t.Errorf("Expected medium severity for .xyz TLD, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("Expected a 'network' finding mentioning .xyz TLD")
	}
}

func TestScan_NetworkCheckDetectsTyposquatting(t *testing.T) {
	scanner := NewSecurityScanner()

	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "https://unpkg.cm/library.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && f.Severity == "high" {
			found = true
		}
	}
	if !found {
		t.Error("Expected high-severity network finding for typosquatting domain unpkg.cm")
	}
}

func TestScan_NetworkCheckDetectsMixedContent(t *testing.T) {
	scanner := NewSecurityScanner()

	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "http://cdn.example.com/script.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" && strings.Contains(f.Title, "mixed content") {
			found = true
			if f.Severity != "high" {
				t.Errorf("Expected high severity for script mixed content, got %s", f.Severity)
			}
		}
	}
	if !found {
		t.Error("Expected mixed content finding for HTTP script on HTTPS page")
	}
}

func TestScan_NetworkCheckSafeOriginNoFindings(t *testing.T) {
	scanner := NewSecurityScanner()

	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "https://cdn.example.com/library.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
		Checks:   []string{"network"},
	}

	result := scanner.Scan(input)

	for _, f := range result.Findings {
		if f.Check == "network" {
			t.Errorf("Safe origin should not produce network findings, got: %s", f.Title)
		}
	}
}

func TestScan_NetworkCheckIncludedByDefault(t *testing.T) {
	scanner := NewSecurityScanner()

	// No explicit Checks — should run all including "network"
	input := SecurityScanInput{
		WaterfallEntries: []capture.NetworkWaterfallEntry{
			{URL: "https://cdn-analytics.xyz/tracker.js", InitiatorType: "script"},
		},
		PageURLs: []string{"https://myapp.com"},
	}

	result := scanner.Scan(input)

	found := false
	for _, f := range result.Findings {
		if f.Check == "network" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Network check should run by default when no checks specified")
	}
}
