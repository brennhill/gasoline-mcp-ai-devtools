// cli_adapter.go — Bridges the main package to the internal/cli sub-package.
// Why: Keeps CLI subsystem in its own package while main dispatches to it with injected runtime config.

package main

import (
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/cli"
)

// cliRuntimeConfig builds the RuntimeConfig that injects main-package values into the CLI package.
func cliRuntimeConfig() cli.RuntimeConfig {
	return cli.RuntimeConfig{
		DefaultPort:        defaultPort,
		MaxPostBodySize:    maxPostBodySize,
		IsServerRunning:    isServerRunning,
		WaitForServer:      func(port int, timeout time.Duration) bool { return waitForServer(port, timeout) },
		DaemonProcessArgv0: daemonProcessArgv0,
	}
}
