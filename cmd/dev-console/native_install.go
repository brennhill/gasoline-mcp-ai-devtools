package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dev-console/dev-console/internal/util"
)

// runNativeInstall detects and configures all supported MCP clients.
func runNativeInstall() {
	// 1. Silent Reset (Kill stale instances)
	// We do this first to ensure config files aren't being held open
	// and no old versions are interfering.
	_ = runForceCleanupQuietly()

	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: Could not determine gasoline binary path: %v\n", err)
		os.Exit(1)
	}

	home, _ := os.UserHomeDir()
	extDir := filepath.Join(home, ".gasoline", "extension")

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

		_ = mergeJSONConfig(path, cfg.key, exe, cfg.isCustom)
	}

	// 4. Start the Daemon
	// We start the daemon so the extension works immediately and the user
	// can verify the install with a health check.
	fmt.Printf("🚀 Starting Gasoline server...")
	startDaemonSilently(exe)

	// 5. BIG SUCCESS MESSAGE
	fmt.Println("\n\033[1;32m✅ GASOLINE INSTALLED & RUNNING!\033[0m")
	fmt.Println("\033[1;34m--------------------------------------------------\033[0m")
	fmt.Printf("\033[1;33m1. INSTALL YOUR EXTENSION AT:\033[0m\n")
	fmt.Printf("   \033[1m%s\033[0m\n", extDir)
	fmt.Println("   (Open chrome://extensions -> Developer mode -> Load unpacked)")
	fmt.Println("")
	fmt.Printf("\033[1;33m2. READY TO COOK:\033[0m\n")
	fmt.Println("   The Gasoline server is active on port 7890.")
	fmt.Println("   Your AI tool (Claude, Cursor, etc.) is now configured.")
	fmt.Println("\033[1;34m--------------------------------------------------\033[0m")
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
		fmt.Printf(" ⚠️  (could not start background server: %v)\n", err)
	} else {
		fmt.Println(" ✅")
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
	data, _ := json.Marshal(entry)

	cmd := exec.Command("claude", "mcp", "add-json", "--scope", "user", "gasoline")
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Env = append(os.Environ(), "CLAUDECODE=")
	_ = cmd.Run()
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
		servers["gasoline"] = map[string]any{
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
