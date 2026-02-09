// lifecycle.go — Server lifecycle management.
// Handles checking if gasoline-mcp is running, auto-starting it, and waiting for readiness.
package server

import (
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	// startTimeout is how long to wait for the server to become ready.
	startTimeout = 5 * time.Second
	// healthPollInterval is how often to check server health during startup.
	healthPollInterval = 100 * time.Millisecond
)

// EnsureRunning checks if the MCP server is running and starts it if needed.
// Returns the connected client and any error.
func EnsureRunning(port int, autoStart bool) (*Client, error) {
	client := NewClientWithPort(port)

	// Check if server is already running
	if client.HealthCheck() {
		return client, nil
	}

	// Server not running - should we start it?
	if !autoStart {
		return nil, fmt.Errorf("server not running on port %d. Start it with: gasoline-mcp", port)
	}

	// Auto-start the server
	if err := startServer(port); err != nil {
		return nil, fmt.Errorf("auto-start server: %w", err)
	}

	// Wait for server to be ready
	if err := waitForReady(client); err != nil {
		return nil, fmt.Errorf("server start timeout: %w", err)
	}

	return client, nil
}

// startServer launches gasoline-mcp as a background process.
func startServer(port int) error {
	// Look for gasoline-mcp in PATH
	binary, err := exec.LookPath("gasoline-mcp")
	if err != nil {
		return fmt.Errorf("gasoline-mcp not found in PATH. Install: npm i -g gasoline-mcp")
	}

	cmd := exec.Command(binary, "--port", fmt.Sprintf("%d", port))
	cmd.Stdout = nil // Suppress stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = nil // Let the process live after parent exits

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start gasoline-mcp: %w", err)
	}

	// Detach: we don't wait for the process to finish
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// waitForReady polls the server health endpoint until it responds or times out.
func waitForReady(client *Client) error {
	deadline := time.Now().Add(startTimeout)

	for time.Now().Before(deadline) {
		if client.HealthCheck() {
			return nil
		}
		time.Sleep(healthPollInterval)
	}

	return fmt.Errorf("server did not become ready within %s", startTimeout)
}
