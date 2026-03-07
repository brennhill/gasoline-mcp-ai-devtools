// Purpose: Startup config discovery helpers.
// Why: Keeps main.go focused on runtime boot flow by isolating environment/config probing logic.
// Docs: docs/features/architecture/index.md

package main

import (
	"os"
	"path/filepath"
	"strings"
)

// findMCPConfig checks for MCP configuration files in common locations.
// Returns the path if found, empty string otherwise.
func findMCPConfig() string {
	// Claude Code - project-local config
	if _, err := os.Stat(".mcp.json"); err == nil {
		return ".mcp.json"
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// Check common MCP config locations
	locations := []string{
		filepath.Join(home, ".claude.json"),                            // Claude
		filepath.Join(home, ".cursor", "mcp.json"),                     // Cursor
		filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"), // Windsurf
		filepath.Join(home, ".continue", "config.json"),                // Continue
		filepath.Join(home, ".config", "zed", "settings.json"),         // Zed
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			// Verify it actually contains gasoline config
			// #nosec G304 -- paths are from a fixed list of known MCP config locations, not user input
			data, err := os.ReadFile(path) // nosemgrep: go_filesystem_rule-fileread -- CLI tool reads known MCP config locations
			if err == nil && (strings.Contains(string(data), "gasoline") || strings.Contains(string(data), "gasoline-mcp")) {
				return path
			}
		}
	}

	return ""
}
