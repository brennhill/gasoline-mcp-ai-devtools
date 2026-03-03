// Purpose: Unit tests for security config logic.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/security-hardening/index.md

package security

import (
	"strings"
	"testing"
)

func setModeForTest(mcp, interactive bool) func() {
	modeMu.Lock()
	prevMCP := isMCPMode
	prevInteractive := isInteractive
	isMCPMode = mcp
	isInteractive = interactive
	modeMu.Unlock()

	return func() {
		modeMu.Lock()
		isMCPMode = prevMCP
		isInteractive = prevInteractive
		modeMu.Unlock()
	}
}

func TestSecurityConfigGuardsNonInteractiveAndInteractivePaths(t *testing.T) {
	restorePath := getSecurityConfigPath()
	setSecurityConfigPath("/tmp/security-config-test.json")
	defer setSecurityConfigPath(restorePath)

	restoreMode := setModeForTest(false, false)
	err := AddToWhitelist("https://cdn.example.com")
	if err == nil || !strings.Contains(err.Error(), "manual-only") {
		t.Fatalf("AddToWhitelist non-interactive error = %v, want manual-only guidance", err)
	}
	err = SetMinSeverity("high")
	if err == nil || !strings.Contains(err.Error(), "manual-only") {
		t.Fatalf("SetMinSeverity non-interactive error = %v, want manual-only guidance", err)
	}
	err = ClearWhitelist()
	if err == nil || !strings.Contains(err.Error(), "manual-only") {
		t.Fatalf("ClearWhitelist non-interactive error = %v, want manual-only guidance", err)
	}

	restoreMode()
	restoreMode = setModeForTest(false, true)
	defer restoreMode()

	err = AddToWhitelist("https://cdn.example.com")
	if err == nil || !strings.Contains(err.Error(), "manual-only") {
		t.Fatalf("AddToWhitelist interactive error = %v, want manual-only guidance", err)
	}
	err = SetMinSeverity("high")
	if err == nil || !strings.Contains(err.Error(), "manual-only") {
		t.Fatalf("SetMinSeverity interactive error = %v, want manual-only guidance", err)
	}
	err = ClearWhitelist()
	if err == nil || !strings.Contains(err.Error(), "manual-only") {
		t.Fatalf("ClearWhitelist interactive error = %v, want manual-only guidance", err)
	}
}

func TestSecurityConfigEditInstructionUsesConfiguredPath(t *testing.T) {
	original := getSecurityConfigPath()
	setSecurityConfigPath("/tmp/custom-security.json")
	defer setSecurityConfigPath(original)

	got := securityConfigEditInstruction()
	if !strings.Contains(got, "/tmp/custom-security.json") {
		t.Fatalf("securityConfigEditInstruction() = %q, expected configured path", got)
	}
}

func TestSecurityConfigMutationAttemptsAreAuditedInMemory(t *testing.T) {
	restoreMode := setModeForTest(false, true)
	defer restoreMode()
	ClearSecurityAuditEvents()
	t.Cleanup(ClearSecurityAuditEvents)

	_ = AddToWhitelist("https://cdn.example.com")
	events := GetSecurityAuditEvents()
	if len(events) == 0 {
		t.Fatal("expected at least one audit event")
	}

	last := events[len(events)-1]
	if last.Action != "security_config_mutation_blocked" {
		t.Fatalf("audit action = %q, want security_config_mutation_blocked", last.Action)
	}
	if last.Persistent {
		t.Fatalf("Persistent = true, want false for in-memory audit events")
	}
	if !strings.Contains(last.Reason, "manual-only") {
		t.Fatalf("audit reason = %q, want manual-only guidance", last.Reason)
	}
}
