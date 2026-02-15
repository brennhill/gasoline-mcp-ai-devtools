package security

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/state"
)

func TestGetSecurityConfigPathUsesStateDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	original := getSecurityConfigPath()
	setSecurityConfigPath("")
	defer setSecurityConfigPath(original)

	got := getSecurityConfigPath()
	want := filepath.Join(stateRoot, "security", "security.json")
	if got != want {
		t.Fatalf("getSecurityConfigPath() = %q, want %q", got, want)
	}
}

func TestAddToWhitelistErrorIncludesResolvedConfigPath(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)
	t.Setenv("MCP_MODE", "1")

	original := getSecurityConfigPath()
	setSecurityConfigPath("")
	defer setSecurityConfigPath(original)

	InitMode()
	err := AddToWhitelist("https://cdn.example.com")
	if err == nil {
		t.Fatal("AddToWhitelist() error = nil, want blocked in MCP mode")
	}

	wantPath := filepath.Join(stateRoot, "security", "security.json")
	if !strings.Contains(err.Error(), wantPath) {
		t.Fatalf("AddToWhitelist() error = %q, want it to include %q", err.Error(), wantPath)
	}
}
