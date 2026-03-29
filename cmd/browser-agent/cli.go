// Purpose: Implements standalone CLI mode execution flow, daemon bootstrap, and tool call dispatch.
// Why: Enables scriptable local usage without requiring direct MCP client integration.
// Docs: docs/features/feature/enhanced-cli-config/index.md

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

// cliToolNames lists valid tool names for CLI mode detection.
var cliToolNames = map[string]bool{
	"observe":   true,
	"analyze":   true,
	"generate":  true,
	"configure": true,
	"interact":  true,
}

// cliConfig holds resolved CLI configuration.
type cliConfig struct {
	Port    int
	Format  string
	Timeout int // milliseconds
}

// IsCLIMode returns true if the first argument is a known tool name.
func IsCLIMode(args []string) bool {
	if len(args) == 0 {
		return false
	}
	return cliToolNames[args[0]]
}

// runCLIMode is the main CLI flow. Returns exit code.
func runCLIMode(args []string) int {
	cfg, remaining := resolveCLIConfig(args)

	if len(remaining) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: kaboom <tool> <action> [flags]\n")
		fmt.Fprintf(os.Stderr, "  Tools: observe, analyze, generate, configure, interact\n")
		fmt.Fprintf(os.Stderr, "  Example: kaboom observe errors --limit 50\n")
		return 2
	}

	tool := remaining[0]
	action := remaining[1]
	toolArgs := remaining[2:]

	// Parse tool-specific arguments
	mcpArgs, err := parseCLIArgs(tool, action, toolArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 2
	}

	// Ensure daemon is running and get base URL
	baseURL, err := ensureDaemon(cfg.Port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	// Long-running modes get extended timeout
	timeout := cfg.Timeout
	if tool == "analyze" && normalizeAction(action) == "accessibility" {
		timeout = 35000
	}
	if tool == "observe" && normalizeAction(action) == "command_result" && timeout < 60000 {
		timeout = 60000
	}

	// Call the tool
	result, err := callTool(baseURL, tool, mcpArgs, timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	return formatResult(cfg.Format, tool, normalizeAction(action), result)
}

// resolveCLIConfig resolves config from defaults < env < flags, stripping global flags.
func resolveCLIConfig(args []string) (cliConfig, []string) {
	cfg := cliConfig{
		Port:    defaultPort,
		Format:  "human",
		Timeout: 15000,
	}

	applyCLIEnvOverrides(&cfg)

	remaining := applyCLIFlagOverrides(args, &cfg)

	return cfg, remaining
}

// applyCLIEnvOverrides applies KABOOM_PORT and KABOOM_FORMAT environment variables.
func applyCLIEnvOverrides(cfg *cliConfig) {
	if envPort := os.Getenv("KABOOM_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			cfg.Port = p
		}
	}
	if envFormat := os.Getenv("KABOOM_FORMAT"); envFormat != "" {
		cfg.Format = envFormat
	}
}

// applyCLIFlagOverrides strips --port, --format, --timeout flags and applies their values.
func applyCLIFlagOverrides(args []string, cfg *cliConfig) []string {
	remaining := args

	var portStr string
	portStr, remaining = cliParseFlag(remaining, "--port")
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			cfg.Port = p
		}
	}

	var format string
	format, remaining = cliParseFlag(remaining, "--format")
	if format != "" {
		cfg.Format = format
	}

	var timeoutStr string
	timeoutStr, remaining = cliParseFlag(remaining, "--timeout")
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil {
			cfg.Timeout = t
		}
	}

	return remaining
}

// ensureDaemon checks if the server is running and spawns it if needed.
// Returns the base URL (e.g., "http://127.0.0.1:7890").
func ensureDaemon(port int) (string, error) {
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	if isServerRunning(port) {
		return baseURL, nil
	}

	// Spawn daemon
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot find executable: %w", err)
	}

	cmd := exec.Command(exe, "--daemon", "--port", fmt.Sprintf("%d", port)) // #nosec G204,G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- CLI opens browser with known URL
	cmd.Args[0] = daemonProcessArgv0(exe)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	util.SetDetachedProcess(cmd)

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for daemon to become ready
	if !waitForServer(port, 4*time.Second) {
		return "", fmt.Errorf("daemon started but not responding on port %d after 4s", port)
	}

	return baseURL, nil
}
