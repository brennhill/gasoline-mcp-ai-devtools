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
	if err := installClaudeCode(exe); err != nil {
		stderrf("⚠️  Claude Code config: %v\n", err)
	}

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

		if err := mergeJSONConfig(path, cfg.key, exe, cfg.isCustom); err != nil {
			stderrf("⚠️  %s config: %v\n", cfg.name, err)
		}
	}

	// 3b. Codex (TOML-based config)
	installCodex(home, exe)

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

func installClaudeCode(exePath string) error {
	if _, err := exec.LookPath("claude"); err != nil {
		return nil // claude CLI not installed, nothing to do
	}

	// Remove legacy MCP server registrations before adding the canonical one.
	for _, legacy := range installerLegacyServerKeys {
		cmd := exec.Command("claude", "mcp", "remove", "--scope", "user", legacy)
		cmd.Env = append(os.Environ(), "CLAUDECODE=")
		_ = cmd.Run() //nolint:errcheck // best-effort legacy removal
	}

	entry := map[string]any{
		"command": exePath,
		"args":    []string{},
	}
	data, _ := json.Marshal(entry) //nolint:errcheck // map[string]any always marshals

	cmd := exec.Command("claude", "mcp", "add-json", "--scope", "user", mcpServerName)
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Env = append(os.Environ(), "CLAUDECODE=")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude mcp add-json failed: %v\n%s", err, out)
	}
	return nil
}

// installCodex updates ~/.codex/config.toml to register the MCP server.
// Codex uses TOML with [mcp_servers.<name>] sections.
func installCodex(home, exePath string) {
	configPath := filepath.Join(home, ".codex", "config.toml")
	if _, err := os.Stat(filepath.Dir(configPath)); os.IsNotExist(err) {
		return
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return // no config file, nothing to do
	}

	// Build set of section headers to remove (legacy + canonical, we re-add canonical).
	removeSet := map[string]bool{
		"[mcp_servers." + mcpServerName + "]": true,
	}
	for _, legacy := range installerLegacyServerKeys {
		removeSet["[mcp_servers."+legacy+"]"] = true
	}

	// Filter out old sections line by line.
	lines := strings.Split(string(raw), "\n")
	var out []string
	skipping := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if removeSet[trimmed] {
			skipping = true
			continue
		}
		// A new TOML section header ends the skip.
		if skipping && strings.HasPrefix(trimmed, "[") {
			skipping = false
		}
		if !skipping {
			out = append(out, line)
		}
	}

	// Strip trailing blank lines before appending.
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}

	// Append the canonical entry.
	out = append(out, "",
		"[mcp_servers."+mcpServerName+"]",
		fmt.Sprintf("command = %q", exePath),
	)
	out = append(out, "") // trailing newline

	_ = os.WriteFile(configPath, []byte(strings.Join(out, "\n")), 0o600) //nolint:errcheck // best-effort
}

func mergeJSONConfig(path, key, exePath string, isCustom bool) error {
	data := make(map[string]any)
	if bytes, err := os.ReadFile(path); err == nil {
		if len(bytes) > 0 {
			if err := json.Unmarshal(bytes, &data); err != nil {
				return fmt.Errorf("refusing to overwrite %s: existing file has invalid JSON (%v). Fix the file manually or back it up before retrying", path, err)
			}
		}
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

	// Back up existing file before overwriting.
	if existing, err := os.ReadFile(path); err == nil && len(existing) > 0 {
		_ = os.WriteFile(path+".bak", existing, 0600)
	}

	return os.WriteFile(path, append(out, '\n'), 0600)
}
