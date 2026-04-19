// runner_test.go — Tests for the install-script runner argv builder.
// The actual spawn path is platform-specific and tested via the HTTP
// handler integration level.

package upgrade

import (
	"strings"
	"testing"
)

func TestBuildInstallCmd_UsesBashDashC(t *testing.T) {
	t.Parallel()
	name, args, err := buildInstallCmd("https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "bash" {
		t.Errorf("name = %q, want %q", name, "bash")
	}
	if len(args) != 2 || args[0] != "-c" {
		t.Errorf("args = %v, want [-c <script>]", args)
	}
}

func TestBuildInstallCmd_EmbedsCurlPipeBash(t *testing.T) {
	t.Parallel()
	url := "https://raw.githubusercontent.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/STABLE/scripts/install.sh"
	_, args, err := buildInstallCmd(url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	script := args[1]
	if !strings.Contains(script, "curl -sSL") {
		t.Errorf("script missing curl invocation: %q", script)
	}
	if !strings.Contains(script, url) {
		t.Errorf("script missing URL: %q", script)
	}
	if !strings.Contains(script, "| bash") {
		t.Errorf("script missing '| bash' pipe: %q", script)
	}
}

func TestBuildInstallCmd_RejectsEmptyURL(t *testing.T) {
	t.Parallel()
	if _, _, err := buildInstallCmd(""); err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
}

func TestBuildInstallCmd_RejectsNonHTTPSURL(t *testing.T) {
	t.Parallel()
	if _, _, err := buildInstallCmd("http://example.com/install.sh"); err == nil {
		t.Fatal("expected error for non-https URL, got nil")
	}
	if _, _, err := buildInstallCmd("file:///etc/passwd"); err == nil {
		t.Fatal("expected error for file:// URL, got nil")
	}
}

func TestBuildInstallCmd_RejectsShellMetacharsInURL(t *testing.T) {
	t.Parallel()
	// URL is embedded inside single quotes in the bash -c script; a literal
	// single quote or a backtick would break the quoting and allow injection.
	bad := []string{
		"https://example.com/install.sh'; rm -rf /",
		"https://example.com/install.sh`whoami`",
		"https://example.com/install.sh$(whoami)",
		"https://example.com/install.sh\nrm -rf /",
	}
	for _, u := range bad {
		if _, _, err := buildInstallCmd(u); err == nil {
			t.Errorf("expected error for dangerous URL %q, got nil", u)
		}
	}
}
