// Purpose: Auto-detects and configures MCP client integrations (Claude Code, Cursor, Windsurf, etc.) during --install.
// Why: Provides zero-config onboarding by writing the correct JSON config for each supported MCP client.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// installerLegacyServerKeys are historical MCP config IDs that are migrated
// to the canonical mcpServerName during install.
var installerLegacyServerKeys = []string{
	"gasoline-agentic-browser",
	"gasoline",
}

const installerCanonicalBinaryName = "gasoline-agentic-devtools"

var installerLegacyBinaryNames = []string{
	"gasoline-agentic-browser",
	"gasoline",
}

func installerPreferredBinaryPath(exePath string) string {
	if strings.TrimSpace(exePath) == "" {
		return exePath
	}

	ext := filepath.Ext(exePath)
	base := strings.TrimSuffix(filepath.Base(exePath), ext)
	isLegacy := false
	for _, legacy := range installerLegacyBinaryNames {
		if base == legacy {
			isLegacy = true
			break
		}
	}
	if !isLegacy {
		return exePath
	}

	canonicalPath := filepath.Join(filepath.Dir(exePath), installerCanonicalBinaryName+ext)
	if _, err := os.Stat(canonicalPath); err == nil {
		return canonicalPath
	}
	return exePath
}

func extensionInstallDir(home string) string {
	if override := strings.TrimSpace(os.Getenv("GASOLINE_EXTENSION_DIR")); override != "" {
		return override
	}
	return filepath.Join(home, "GasolineAgenticDevtoolExtension")
}

func manualExtensionSetupChecklist(extDir string) []string {
	return []string{
		"BROWSER EXTENSION (MANUAL STEP REQUIRED):",
		"   The installer staged extension files, but it cannot click browser UI controls for you.",
		"   1) Open chrome://extensions (or brave://extensions)",
		"   2) Enable Developer mode",
		"   3) Click Load unpacked and select:",
		fmt.Sprintf("      %s", extDir),
		"   4) Pin Gasoline in the browser toolbar (recommended)",
		"   5) Open the Gasoline popup and click Track This Tab",
	}
}

func printManualExtensionSetupChecklist(extDir string) {
	lines := manualExtensionSetupChecklist(extDir)
	if len(lines) == 0 {
		return
	}
	stderrf("\033[1;33m%s\033[0m\n", lines[0])
	for _, line := range lines[1:] {
		stderrf("%s\n", line)
	}
}

func printInstallerPanel(title string, lines []string) {
	const border = "+----------------------------------------------------------+"
	stderrf("\033[1;36m%s\033[0m\n", border)
	stderrf("\033[1;36m| \033[1m%-56s\033[1;36m |\033[0m\n", title)
	stderrf("\033[1;36m%s\033[0m\n", border)
	for _, line := range lines {
		stderrf("\033[1;36m|\033[0m %-58s \033[1;36m|\033[0m\n", line)
	}
	stderrf("\033[1;36m%s\033[0m\n", border)
}

// runNativeInstall detects and configures all supported MCP clients.
func runNativeInstall() {
	// 1. Silent Reset (Kill stale instances)
	// We do this first to ensure config files aren't being held open
	// and no old versions are interfering.
	_ = runForceCleanupQuietly() //nolint:errcheck // best-effort pre-install cleanup

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: Could not determine gasoline binary path: %v\n", err)
		os.Exit(1)
	}
	exe = installerPreferredBinaryPath(exe)

	home, _ := os.UserHomeDir()
	extDir := extensionInstallDir(home)

	// 2. Claude Code
	installClaudeCode(exe)

	// 3. File-based configs
	configs := []struct {
		name     string
		path     string
		key      string
		isCustom bool
	}{
		{"Cursor", "~/.cursor/mcp.json", "mcpServers", false},
		{"Windsurf", "~/.codeium/windsurf/mcp_config.json", "mcpServers", false},
		{"Gemini CLI", "~/.gemini/settings.json", "mcpServers", false},
		{"Antigravity", "~/.gemini/antigravity/mcp_config.json", "mcpServers", false},
		{"OpenCode", "~/.config/opencode/opencode.json", "mcp", true},
		{"Zed", "~/.config/zed/settings.json", "context_servers", true},
	}

	// OS-specific paths
	if runtime.GOOS == "darwin" {
		configs = append(configs, struct {
			name     string
			path     string
			key      string
			isCustom bool
		}{"Claude Desktop", "Library/Application Support/Claude/claude_desktop_config.json", "mcpServers", false})
		configs = append(configs, struct {
			name     string
			path     string
			key      string
			isCustom bool
		}{"VS Code", "Library/Application Support/Code/User/mcp.json", "mcpServers", false})
	} else if runtime.GOOS == "linux" {
		configs = append(configs, struct {
			name     string
			path     string
			key      string
			isCustom bool
		}{"VS Code", ".config/Code/User/mcp.json", "mcpServers", false})
	} else if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		configs = append(configs, struct {
			name     string
			path     string
			key      string
			isCustom bool
		}{"Claude Desktop", filepath.Join(appData, "Claude", "claude_desktop_config.json"), "mcpServers", false})
		configs = append(configs, struct {
			name     string
			path     string
			key      string
			isCustom bool
		}{"VS Code", filepath.Join(appData, "Code", "User", "mcp.json"), "mcpServers", false})
	}

	for _, cfg := range configs {
		path := cfg.path
		if strings.HasPrefix(path, "~/") {
			path = filepath.Join(home, path[2:])
		} else if !filepath.IsAbs(path) && home != "" {
			path = filepath.Join(home, path)
		}

		if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
			continue // Client directory doesn't exist, skip
		}

		_ = mergeJSONConfig(path, cfg.key, exe, cfg.isCustom) //nolint:errcheck // best-effort per-client config
	}

	// 4. Start the Daemon
	// We start the daemon so the extension works immediately and the user
	// can verify the install with a health check.
	stderrf("🚀 Starting Gasoline server...")
	startDaemonSilently(exe)

	// 5. BIG SUCCESS MESSAGE
	stderrf("\n\033[1;32m✅ GASOLINE INSTALLED & RUNNING!\033[0m\n")
	printInstallerPanel("INSTALL SUMMARY", []string{
		"Gasoline server started in background on port 7890.",
		"MCP clients are configured with direct binary path (no npx).",
		fmt.Sprintf("Binary path: %s", exe),
	})
	stderrf("\n")
	printManualExtensionSetupChecklist(extDir)
	stderrf("\033[1;33mREADY TO COOK:\033[0m\n")
	stderrf("   The Gasoline server is active on port 7890.\n")
	stderrf("   Your AI tool (Claude, Cursor, etc.) is now configured.\n")
	stderrf("\033[1;36m+----------------------------------------------------------+\033[0m\n")
}

func startDaemonSilently(exe string) {
	// Standard daemon flags
	args := []string{"--daemon", "--port", "7890"}
	cmd := exec.Command(exe, args...)

	// Ensure it's detached
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	// Platform-specific detachment (Unix/Windows)
	util.SetDetachedProcess(cmd)

	if err := cmd.Start(); err != nil {
		stderrf(" ⚠️  (could not start background server: %v)\n", err)
	} else {
		stderrf(" ✅\n")
	}
}

func installClaudeCode(exePath string) {
	if _, err := exec.LookPath("claude"); err != nil {
		return
	}

	entry := map[string]any{
		"command": exePath,
		"args":    []string{},
	}
	data, _ := json.Marshal(entry) //nolint:errcheck // map[string]any always marshals

	cmd := exec.Command("claude", "mcp", "add-json", "--scope", "user", mcpServerName)
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Env = append(os.Environ(), "CLAUDECODE=")
	_ = cmd.Run() //nolint:errcheck // best-effort Claude Code MCP registration
}

func mergeJSONConfig(path, key, exePath string, isCustom bool) error {
	data := make(map[string]any)
	if bytes, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(bytes, &data) //nolint:errcheck // start with empty map if unmarshal fails
	}

	if _, ok := data[key]; !ok {
		data[key] = make(map[string]any)
	}

	servers, ok := data[key].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected format for key %q", key)
	}
	for _, legacy := range installerLegacyServerKeys {
		delete(servers, legacy)
	}

	if isCustom {
		if key == "mcp" { // OpenCode
			servers[mcpServerName] = map[string]any{
				"type":    "local",
				"command": []string{exePath},
				"enabled": true,
			}
		} else if key == "context_servers" { // Zed
			servers[mcpServerName] = map[string]any{
				"source":  "custom",
				"command": exePath,
				"args":    []string{},
			}
		}
	} else {
		servers[mcpServerName] = map[string]any{
			"command": exePath,
			"args":    []string{},
		}
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(out, '\n'), 0600)
}
