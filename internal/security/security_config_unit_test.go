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
	if err == nil || !strings.Contains(err.Error(), "not in interactive mode") {
		t.Fatalf("AddToWhitelist non-interactive error = %v, want not-in-interactive", err)
	}
	err = SetMinSeverity("high")
	if err == nil || !strings.Contains(err.Error(), "not in interactive mode") {
		t.Fatalf("SetMinSeverity non-interactive error = %v, want not-in-interactive", err)
	}
	err = ClearWhitelist()
	if err == nil || !strings.Contains(err.Error(), "not in interactive mode") {
		t.Fatalf("ClearWhitelist non-interactive error = %v, want not-in-interactive", err)
	}

	restoreMode()
	restoreMode = setModeForTest(false, true)
	defer restoreMode()

	err = AddToWhitelist("https://cdn.example.com")
	if err == nil || !strings.Contains(err.Error(), "not yet fully implemented") {
		t.Fatalf("AddToWhitelist interactive error = %v, want not-yet-implemented", err)
	}
	err = SetMinSeverity("high")
	if err == nil || !strings.Contains(err.Error(), "not yet fully implemented") {
		t.Fatalf("SetMinSeverity interactive error = %v, want not-yet-implemented", err)
	}
	err = ClearWhitelist()
	if err == nil || !strings.Contains(err.Error(), "not yet fully implemented") {
		t.Fatalf("ClearWhitelist interactive error = %v, want not-yet-implemented", err)
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
