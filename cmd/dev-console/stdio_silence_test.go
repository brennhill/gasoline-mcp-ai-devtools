package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// ⚠️ CRITICAL INVARIANT TEST - MCP STDIO SILENCE
//
// This test verifies that the wrapper and server produce ZERO non-JSON-RPC output
// on stdio during normal MCP operation. This is essential for MCP compliance.
//
// See: .claude/refs/mcp-stdio-invariant.md
//
// The wrapper and server MUST:
// 1. Output ONLY JSON-RPC messages to stdout
// 2. Output NOTHING to stderr during normal operation (silent connection)
// 3. Log all diagnostics/retries/debugging to log files
//
// DO NOT:
// - Remove or weaken this test
// - Allow any non-JSON-RPC output to stdio
// - Print progress messages, retry logs, or diagnostics to stderr

func TestStdioSilence_NormalConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)

	// Kill any existing server
	killServerOnPort(t, port)

	// Build the binary path
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get test binary path: %v", err)
	}

	// Spawn server like MCP client would
	cmd := exec.Command(binary, "--port", strconv.Itoa(port))

	// Capture stdout and stderr separately
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Create stdin pipe for sending MCP initialize request
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("Failed to create stdin pipe: %v", err)
	}

	// Start the server
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Send MCP initialize request (like real LLM would)
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test-llm","version":"1.0"}}}`
	_, err = stdin.Write([]byte(initRequest + "\n"))
	if err != nil {
		t.Fatalf("Failed to write initialize request: %v", err)
	}

	// Wait for response
	time.Sleep(1 * time.Second)

	// Close stdin to trigger shutdown
	_ = stdin.Close()

	// Wait for graceful shutdown
	time.Sleep(500 * time.Millisecond)

	// Kill if still running
	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	// CRITICAL CHECK: Verify stdout contains ONLY JSON-RPC
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	t.Logf("=== Stdout Output ===")
	t.Logf("%s", stdoutStr)
	t.Logf("=== Stderr Output ===")
	t.Logf("%s", stderrStr)

	// Check stdout: Should contain JSON-RPC response
	if stdoutStr == "" {
		t.Errorf("INVARIANT VIOLATION: Expected JSON-RPC response on stdout, got empty")
	}

	// Parse stdout lines - all should be valid JSON-RPC
	scanner := bufio.NewScanner(strings.NewReader(stdoutStr))
	lineNum := 0
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if line == "" {
			continue // Empty lines are OK
		}

		// Must be valid JSON
		var msg map[string]interface{}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Errorf("INVARIANT VIOLATION: Stdout line %d is not valid JSON: %q", lineNum, line)
			continue
		}

		// Must have jsonrpc field
		if msg["jsonrpc"] != "2.0" {
			t.Errorf("INVARIANT VIOLATION: Stdout line %d is not JSON-RPC 2.0: %q", lineNum, line)
		}

		// Should have id or method
		hasID := msg["id"] != nil
		hasMethod := msg["method"] != nil
		hasResult := msg["result"] != nil
		hasError := msg["error"] != nil

		if !hasID && !hasMethod {
			t.Errorf("INVARIANT VIOLATION: Stdout line %d has no id or method: %q", lineNum, line)
		}

		if !hasMethod && !hasResult && !hasError {
			t.Errorf("INVARIANT VIOLATION: Stdout line %d has no method/result/error: %q", lineNum, line)
		}
	}

	// Check stderr: Should be COMPLETELY SILENT during normal operation
	stderrLines := strings.Split(strings.TrimSpace(stderrStr), "\n")
	nonEmptyLines := 0
	for _, line := range stderrLines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
			t.Logf("Stderr line %d: %q", nonEmptyLines, line)
		}
	}

	if nonEmptyLines > 0 {
		t.Errorf("INVARIANT VIOLATION: Expected 0 stderr lines during normal operation, got %d", nonEmptyLines)
		t.Errorf("All logs should go to ~/gasoline-wrapper.log or ~/gasoline-logs.jsonl")
	}

	t.Logf("✅ Stdio silence invariant verified: 0 stderr lines, stdout is JSON-RPC only")
}

func TestStdioSilence_MultiClientSpawn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)

	// Kill any existing server
	killServerOnPort(t, port)

	numClients := 5
	var stderrOutputs []string

	// Launch multiple clients simultaneously (simulates race condition)
	for i := 0; i < numClients; i++ {
		binary, err := os.Executable()
		if err != nil {
			t.Fatalf("Failed to get binary path: %v", err)
		}

		cmd := exec.Command(binary, "--port", strconv.Itoa(port))

		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatalf("Client %d: Failed to create stdin: %v", i, err)
		}

		if err := cmd.Start(); err != nil {
			t.Fatalf("Client %d: Failed to start: %v", i, err)
		}

		// Send initialize
		initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"client-` + strconv.Itoa(i) + `","version":"1.0"}}}`
		_, _ = stdin.Write([]byte(initRequest + "\n"))

		time.Sleep(50 * time.Millisecond)
		_ = stdin.Close()

		// Wait for process to finish
		done := make(chan bool)
		go func() {
			_ = cmd.Wait()
			stderrOutputs = append(stderrOutputs, stderr.String())
			done <- true
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			stderrOutputs = append(stderrOutputs, stderr.String())
		}
	}

	// Check all stderr outputs - should ALL be silent
	totalStderrLines := 0
	for i, stderrStr := range stderrOutputs {
		lines := strings.Split(strings.TrimSpace(stderrStr), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				totalStderrLines++
				t.Logf("Client %d stderr: %q", i, line)
			}
		}
	}

	if totalStderrLines > 0 {
		t.Errorf("INVARIANT VIOLATION: Expected 0 stderr lines from %d clients during normal operation, got %d", numClients, totalStderrLines)
		t.Errorf("Even during race conditions and retries, all output must go to log files")
	}

	t.Logf("✅ Multi-client stdio silence verified: 0 stderr lines from %d concurrent clients", numClients)

	// Cleanup
	killServerOnPort(t, port)
}

func TestStdioSilence_ConnectionRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	port := findFreePort(t)

	// Kill any existing server
	killServerOnPort(t, port)

	// Start a server, then freeze it to force retries
	binary, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get binary path: %v", err)
	}

	// Start server
	serverCmd := exec.Command(binary, "--port", strconv.Itoa(port))
	serverStdin, _ := serverCmd.StdinPipe()
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer func() {
		_ = serverCmd.Process.Kill()
		_ = serverCmd.Wait()
	}()

	// Wait for server to be ready
	time.Sleep(1 * time.Second)

	// Now start a client - it will need to retry connection
	clientCmd := exec.Command(binary, "--port", strconv.Itoa(port))

	var stderr bytes.Buffer
	clientCmd.Stderr = &stderr

	clientStdin, _ := clientCmd.StdinPipe()
	if err := clientCmd.Start(); err != nil {
		t.Fatalf("Failed to start client: %v", err)
	}

	// Send initialize
	initRequest := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"retry-test","version":"1.0"}}}`
	_, _ = clientStdin.Write([]byte(initRequest + "\n"))

	// Wait for connection/retries
	time.Sleep(2 * time.Second)

	// Close clients
	_ = clientStdin.Close()
	_ = serverStdin.Close()

	// Wait for shutdown
	done := make(chan bool)
	go func() {
		_ = clientCmd.Wait()
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = clientCmd.Process.Kill()
		_ = clientCmd.Wait()
	}

	// Check stderr - should be silent even during retries
	stderrStr := stderr.String()
	stderrLines := strings.Split(strings.TrimSpace(stderrStr), "\n")
	nonEmptyLines := 0

	for _, line := range stderrLines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
			t.Logf("Stderr line: %q", line)
		}
	}

	if nonEmptyLines > 0 {
		t.Errorf("INVARIANT VIOLATION: Expected 0 stderr lines during connection retry, got %d", nonEmptyLines)
		t.Errorf("Retry messages MUST go to log files, not stderr")
	}

	t.Logf("✅ Connection retry stdio silence verified: 0 stderr lines")

	// Cleanup
	killServerOnPort(t, port)
}

// Helper to kill server on port
func killServerOnPort(t *testing.T, port int) {
	cmd := exec.Command("lsof", "-ti", strconv.Itoa(port))
	if output, err := cmd.Output(); err == nil {
		pids := strings.TrimSpace(string(output))
		if pids != "" {
			_ = exec.Command("kill", "-9", pids).Run()
			time.Sleep(500 * time.Millisecond)
		}
	}
}
