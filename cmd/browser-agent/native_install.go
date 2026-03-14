// Purpose: Orchestrates the --install flow: cleanup, client detection, config writing, daemon start.
// Why: Coordinates install steps without mixing config I/O, UI formatting, or process management.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// mcpClientConfig describes a file-based MCP client integration point.
type mcpClientConfig struct {
	name     string
	path     string
	key      string
	isCustom bool
}

func extensionInstallDir(home string) string {
	if override := strings.TrimSpace(os.Getenv("GASOLINE_EXTENSION_DIR")); override != "" {
		return override
	}
	return filepath.Join(home, "GasolineAgenticDevtoolExtension")
}

// mcpClientConfigs returns the list of file-based MCP client integrations
// for the current platform.
func mcpClientConfigs(home string) []mcpClientConfig {
	configs := []mcpClientConfig{
		{"Cursor", "~/.cursor/mcp.json", "mcpServers", false},
		{"Windsurf", "~/.codeium/windsurf/mcp_config.json", "mcpServers", false},
		{"Gemini CLI", "~/.gemini/settings.json", "mcpServers", false},
		{"Antigravity", "~/.gemini/antigravity/mcp_config.json", "mcpServers", false},
		{"OpenCode", "~/.config/opencode/opencode.json", "mcp", true},
		{"Zed", "~/.config/zed/settings.json", "context_servers", true},
	}

	switch runtime.GOOS {
	case "darwin":
		configs = append(configs,
			mcpClientConfig{"Claude Desktop", "Library/Application Support/Claude/claude_desktop_config.json", "mcpServers", false},
			mcpClientConfig{"VS Code", "Library/Application Support/Code/User/mcp.json", "mcpServers", false},
		)
	case "linux":
		configs = append(configs,
			mcpClientConfig{"VS Code", ".config/Code/User/mcp.json", "mcpServers", false},
		)
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		configs = append(configs,
			mcpClientConfig{"Claude Desktop", filepath.Join(appData, "Claude", "claude_desktop_config.json"), "mcpServers", false},
			mcpClientConfig{"VS Code", filepath.Join(appData, "Code", "User", "mcp.json"), "mcpServers", false},
		)
	}

	return configs
}

// runNativeInstall detects and configures all supported MCP clients.
func runNativeInstall() {
	// 1. Silent Reset (Kill stale instances)
	_ = runForceCleanupQuietly()

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: Could not determine gasoline binary path: %v\n", err)
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	extDir := extensionInstallDir(home)

	// 2. Claude Code (CLI-based)
	if err := installClaudeCode(exe); err != nil {
		stderrf("  ⚠️  Claude Code: %v\n", err)
	}

	// 3. File-based configs
	for _, cfg := range mcpClientConfigs(home) {
		path := cfg.path
		if strings.HasPrefix(path, "~/") {
			path = filepath.Join(home, path[2:])
		} else if !filepath.IsAbs(path) && home != "" {
			path = filepath.Join(home, path)
		}

		if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
			continue
		}

		if err := mergeJSONConfig(path, cfg.key, exe, cfg.isCustom); err != nil {
			stderrf("  ⚠️  %s: %v\n", cfg.name, err)
		}
	}

	// 4. Start the Daemon
	stderrf("🚀 Starting Gasoline server...")
	startDaemonSilently(exe)

	// 5. Success output
	printInstallSuccess(exe, extDir)
}

func startDaemonSilently(exe string) {
	args := []string{"--daemon", "--port", "7890"}
	cmd := exec.Command(exe, args...)

	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	util.SetDetachedProcess(cmd)

	if err := cmd.Start(); err != nil {
		stderrf(" ⚠️  (could not start background server: %v)\n", err)
	} else {
		stderrf(" ✅\n")
	}
}
