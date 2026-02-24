package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// runNativeInstall detects and configures all supported MCP clients.
func runNativeInstall() {
	fmt.Println("🚀 Resetting Gasoline...")
	runForceCleanup() // Kill all stale daemons

	fmt.Println("⚙️  Configuring MCP clients...")

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: Could not determine gasoline binary path: %v
", err)
		os.Exit(1)
	}

	// 1. Claude Code
	installClaudeCode(exe)

	// 2. File-based configs
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
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		configs = append(configs, struct {
			name     string
			path     string
			key      string
			isCustom bool
		}{"Claude Desktop", "~/Library/Application Support/Claude/claude_desktop_config.json", "mcpServers", false})
		configs = append(configs, struct {
			name     string
			path     string
			key      string
			isCustom bool
		}{"VS Code", "~/Library/Application Support/Code/User/mcp.json", "mcpServers", false})
	} else if runtime.GOOS == "linux" {
		configs = append(configs, struct {
			name     string
			path     string
			key      string
			isCustom bool
		}{"VS Code", "~/.config/Code/User/mcp.json", "mcpServers", false})
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
		}

		if _, err := os.Stat(filepath.Dir(path)); os.IsNotExist(err) {
			continue // Client directory doesn't exist, skip
		}

		fmt.Printf("   - %s: ", cfg.name)
		if err := mergeJSONConfig(path, cfg.key, exe, cfg.isCustom); err != nil {
			fmt.Printf("⚠️  %v
", err)
		} else {
			fmt.Println("✅ Done")
		}
	}

	fmt.Println("
✨ Gasoline is now configured for all detected clients.")
}

func installClaudeCode(exePath string) {
	if _, err := exec.LookPath("claude"); err != nil {
		return
	}

	fmt.Printf("   - Claude Code: ")
	entry := map[string]any{
		"command": exePath,
		"args":    []string{},
	}
	data, _ := json.Marshal(entry)

	cmd := exec.Command("claude", "mcp", "add-json", "--scope", "user", "gasoline")
	cmd.Stdin = strings.NewReader(string(data))
	// Unset CLAUDECODE to avoid nested session errors
	cmd.Env = append(os.Environ(), "CLAUDECODE=")

	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  %v
", err)
	} else {
		fmt.Println("✅ Done")
	}
}

func mergeJSONConfig(path, key, exePath string, isCustom bool) error {
	data := make(map[string]any)
	if bytes, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(bytes, &data)
	}

	if _, ok := data[key]; !ok {
		data[key] = make(map[string]any)
	}

	servers, ok := data[key].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected format for key %q", key)
	}

	if isCustom {
		// Specific formats
		if key == "mcp" { // OpenCode
			servers["gasoline"] = map[string]any{
				"type":    "local",
				"command": []string{exePath},
				"enabled": true,
			}
		} else if key == "context_servers" { // Zed
			servers["gasoline"] = map[string]any{
				"source":  "custom",
				"command": exePath,
				"args":    []string{},
			}
		}
	} else {
		// Standard MCP format
		servers["gasoline"] = map[string]any{
			"command": exePath,
			"args":    []string{},
		}
	}

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(out, '
'), 0600)
}
