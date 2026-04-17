package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestManualExtensionSetupChecklist_IncludesRequiredSteps(t *testing.T) {
	extPath := `/Users/tester/KaboomAgenticDevtoolExtension`
	checklist := manualExtensionSetupChecklist(extPath)
	joined := strings.Join(checklist, "\n")

	required := []string{
		"MANUAL STEP REQUIRED",
		"cannot click browser UI controls",
		"chrome://extensions (or brave://extensions)",
		"Enable Developer mode",
		"Load unpacked",
		extPath,
		"Pin Kaboom",
		"Track This Tab",
	}

	for _, want := range required {
		if !strings.Contains(joined, want) {
			t.Fatalf("checklist missing %q; got:\n%s", want, joined)
		}
	}
}

func TestExtensionInstallDir_DefaultVisiblePath(t *testing.T) {
	t.Setenv("KABOOM_EXTENSION_DIR", "")
	home := "/Users/tester"
	want := filepath.Join(home, "KaboomAgenticDevtoolExtension")

	if got := extensionInstallDir(home); got != want {
		t.Fatalf("extensionInstallDir(%q) = %q, want %q", home, got, want)
	}
}

func TestExtensionInstallDir_EnvOverride(t *testing.T) {
	override := "/tmp/custom-kaboom-ext"
	t.Setenv("KABOOM_EXTENSION_DIR", override)
	home := "/Users/tester"

	if got := extensionInstallDir(home); got != override {
		t.Fatalf("extensionInstallDir(%q) = %q, want env override %q", home, got, override)
	}
}

func TestInstallerLegacyServerKeys_IncludeKaboomAndKaboomVariants(t *testing.T) {
	joined := strings.Join(installerLegacyServerKeys, "\n")
	required := []string{
		"kaboom-agentic-browser",
		"kaboom",
		"strum-browser-devtools",
		"strum-agentic-browser",
		"strum",
	}

	for _, want := range required {
		if !strings.Contains(joined, want) {
			t.Fatalf("installerLegacyServerKeys missing %q; got %v", want, installerLegacyServerKeys)
		}
	}
}
