package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/dev-console/dev-console/internal/state"
)

func TestApplyParallelModeStateDir_AutoGeneratesWhenMissing(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(state.StateDirEnv, stateRoot)

	stateDir := ""
	if err := applyParallelModeStateDir(true, &stateDir); err != nil {
		t.Fatalf("applyParallelModeStateDir() error = %v", err)
	}
	if strings.TrimSpace(stateDir) == "" {
		t.Fatal("stateDir should be auto-generated in parallel mode")
	}
	if !strings.HasPrefix(stateDir, filepath.Join(stateRoot, "parallel")+string(filepath.Separator)) {
		t.Fatalf("stateDir = %q, want prefix %q", stateDir, filepath.Join(stateRoot, "parallel")+string(filepath.Separator))
	}
}

func TestApplyParallelModeStateDir_PreservesExplicitStateDir(t *testing.T) {
	explicit := filepath.Join(t.TempDir(), "isolated")
	stateDir := explicit
	if err := applyParallelModeStateDir(true, &stateDir); err != nil {
		t.Fatalf("applyParallelModeStateDir() error = %v", err)
	}
	if stateDir != explicit {
		t.Fatalf("stateDir = %q, want explicit %q", stateDir, explicit)
	}
}

func TestApplyParallelModeStateDir_NoopWhenDisabled(t *testing.T) {
	stateDir := ""
	if err := applyParallelModeStateDir(false, &stateDir); err != nil {
		t.Fatalf("applyParallelModeStateDir() error = %v", err)
	}
	if stateDir != "" {
		t.Fatalf("stateDir = %q, want unchanged empty string", stateDir)
	}
}
