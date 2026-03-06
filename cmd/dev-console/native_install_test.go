package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManualExtensionSetupChecklist_IncludesRequiredSteps(t *testing.T) {
	extPath := `/Users/tester/GasolineAgenticDevtoolExtension`
	checklist := manualExtensionSetupChecklist(extPath)
	joined := strings.Join(checklist, "\n")

	required := []string{
		"MANUAL STEP REQUIRED",
		"cannot click browser UI controls",
		"chrome://extensions (or brave://extensions)",
		"Enable Developer mode",
		"Load unpacked",
		extPath,
		"Pin Gasoline",
		"Track This Tab",
	}

	for _, want := range required {
		if !strings.Contains(joined, want) {
			t.Fatalf("checklist missing %q; got:\n%s", want, joined)
		}
	}
}

func TestExtensionInstallDir_DefaultVisiblePath(t *testing.T) {
	t.Setenv("GASOLINE_EXTENSION_DIR", "")
	home := "/Users/tester"
	want := filepath.Join(home, "GasolineAgenticDevtoolExtension")

	if got := extensionInstallDir(home); got != want {
		t.Fatalf("extensionInstallDir(%q) = %q, want %q", home, got, want)
	}
}

func TestExtensionInstallDir_EnvOverride(t *testing.T) {
	override := "/tmp/custom-gasoline-ext"
	t.Setenv("GASOLINE_EXTENSION_DIR", override)
	home := "/Users/tester"

	if got := extensionInstallDir(home); got != override {
		t.Fatalf("extensionInstallDir(%q) = %q, want env override %q", home, got, override)
	}
}

func TestInstallerPreferredBinaryPath_PrefersCanonicalWhenLegacyNameUsed(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, "gasoline-agentic-devtools")
	if err := os.WriteFile(canonical, []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile(canonical) error = %v", err)
	}

	legacy := filepath.Join(root, "gasoline")
	if got := installerPreferredBinaryPath(legacy); got != canonical {
		t.Fatalf("installerPreferredBinaryPath(%q) = %q, want %q", legacy, got, canonical)
	}
}

func TestInstallerPreferredBinaryPath_KeepsLegacyWhenCanonicalMissing(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "gasoline-agentic-browser")

	if got := installerPreferredBinaryPath(legacy); got != legacy {
		t.Fatalf("installerPreferredBinaryPath(%q) = %q, want unchanged path", legacy, got)
	}
}
