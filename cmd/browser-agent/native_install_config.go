// Purpose: MCP client config file reading, merging, and writing for the --install flow.
// Why: Isolates disk I/O and JSON manipulation from install orchestration and UI output.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// installerLegacyServerKeys are historical MCP config IDs that are migrated
// to the canonical mcpServerName during install.
var installerLegacyServerKeys = []string{
	"gasoline-agentic-browser",
	"gasoline",
}

func installClaudeCode(exePath string) error {
	if _, err := exec.LookPath("claude"); err != nil {
		return nil // Claude Code not installed, skip silently
	}

	entry := map[string]any{
		"command": exePath,
		"args":    []string{},
	}
	data, _ := json.Marshal(entry)

	cmd := exec.Command("claude", "mcp", "add-json", "--scope", "user", mcpServerName)
	cmd.Stdin = strings.NewReader(string(data))
	cmd.Env = append(os.Environ(), "CLAUDECODE=")

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("claude mcp add-json failed: %v (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return nil
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
