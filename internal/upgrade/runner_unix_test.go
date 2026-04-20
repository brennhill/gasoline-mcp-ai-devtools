//go:build !windows

// runner_unix_test.go — Asserts the detached-spawn contract on Unix:
// Setsid must be true so the installer survives the daemon it kills, stdio
// must be fully detached, and the child inherits KABOOM_SELF_UPDATE=1.

package upgrade

import (
	"testing"
)

func TestNewInstallCmd_SetsidAndDetachedStdio(t *testing.T) {
	t.Parallel()
	cmd, err := newInstallCmd("https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmd == nil {
		t.Fatal("cmd is nil")
	}
	if cmd.Path == "" {
		t.Fatal("cmd.Path not set")
	}
	if cmd.Stdin != nil {
		t.Errorf("Stdin = %v, want nil (detached)", cmd.Stdin)
	}
	if cmd.Stdout != nil {
		t.Errorf("Stdout = %v, want nil (detached)", cmd.Stdout)
	}
	if cmd.Stderr != nil {
		t.Errorf("Stderr = %v, want nil (detached)", cmd.Stderr)
	}
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil; Setsid must be set")
	}
	if !cmd.SysProcAttr.Setsid {
		t.Error("SysProcAttr.Setsid = false; installer must run in a new session so pkill on the daemon doesn't TERM the script")
	}
	found := false
	for _, kv := range cmd.Env {
		if kv == "KABOOM_SELF_UPDATE=1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("KABOOM_SELF_UPDATE=1 missing from cmd.Env; installer cannot detect self-update invocation")
	}
	// Env should also include the parent PATH etc. — sanity check length.
	if len(cmd.Env) <= 1 {
		t.Errorf("cmd.Env has %d entries; expected parent environment to be inherited", len(cmd.Env))
	}
}

func TestNewInstallCmd_PropagatesArgvError(t *testing.T) {
	t.Parallel()
	if _, err := newInstallCmd(""); err == nil {
		t.Fatal("expected error for empty URL")
	}
	if _, err := newInstallCmd("http://example.com/install.sh"); err == nil {
		t.Fatal("expected error for non-https URL")
	}
}
