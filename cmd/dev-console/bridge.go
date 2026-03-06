// Purpose: Orchestrates bridge-mode transport forwarding between MCP stdio and daemon HTTP.
// Why: Keeps request/response forwarding resilient across daemon restarts and transport disruptions.
// Docs: docs/features/feature/bridge-restart/index.md

package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/bridge"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/schema"
	statecfg "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/state"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

// isGasolineService accepts canonical and legacy server names for compatibility.
func isGasolineService(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == mcpServerName {
		return true
	}
	for _, legacy := range legacyMCPServerNames {
		if n == legacy {
			return true
		}
	}
	return false
}

// toolCallTimeout delegates to internal/bridge for per-request timeout logic.
func toolCallTimeout(req JSONRPCRequest) time.Duration {
	return bridge.ToolCallTimeout(req.Method, req.Params)
}

// mcpStdoutMu serializes all writes to stdout so concurrent bridgeForwardRequest
// goroutines cannot interleave JSON-RPC responses.
var mcpStdoutMu sync.Mutex

// buildDaemonCmd resolves the current executable and builds an exec.Cmd for the
// daemon process with the appropriate flags and detached-process settings.
func (s *daemonState) buildDaemonCmd() (*exec.Cmd, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("Cannot find executable: %w", err)
	}

	args := []string{"--daemon", "--port", fmt.Sprintf("%d", s.port)}
	if stateDir := os.Getenv(statecfg.StateDirEnv); stateDir != "" {
		args = append(args, "--state-dir", stateDir)
	}
	if s.logFile != "" {
		args = append(args, "--log-file", s.logFile)
	}
	if s.maxEntries > 0 {
		args = append(args, "--max-entries", fmt.Sprintf("%d", s.maxEntries))
	}
	cmd := exec.Command(exe, args...) // #nosec G702 -- exe is our own binary path from os.Executable with fixed flags // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command, go_subproc_rule-subproc -- bridge spawns own daemon
	cmd.Args[0] = daemonProcessArgv0(exe)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.Stdin = nil
	util.SetDetachedProcess(cmd)
	return cmd, nil
}

// isConnectionError delegates to internal/bridge for connection error detection.
func isConnectionError(err error) bool {
	return bridge.IsConnectionError(err)
}

// flushStdout syncs stdout and logs any errors (best-effort)
func flushStdout() {
	syncStdoutBestEffort()
}

// bridgeStdioToHTTPFast forwards JSON-RPC with fast-start: responds to initialize/tools/list
// immediately while daemon starts in background. Only blocks on tools/call.
// #lizard forgives
func bridgeStdioToHTTPFast(endpoint string, state *daemonState, port int) {
	reader := bufio.NewReaderSize(os.Stdin, 64*1024)

	client := &http.Client{} // per-request timeouts via context

	// Start push relay goroutine to poll daemon inbox and relay to Claude via stdio.
	pushRelayDone := make(chan struct{})
	startBridgePushRelay(client, endpoint, pushRelayDone)

	var wg sync.WaitGroup
	responseSent := make(chan bool, 1)
	var responseOnce sync.Once
	stats := &bridgeSessionStats{}
	signalResponseSent := func() {
		responseOnce.Do(func() { responseSent <- true })
	}

	toolsList := schema.AllTools()

	var readErr error
	for {
		line, framing, err := readMCPStdioMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			debugf("stdin read error: %v", err)
			readErr = err
			break
		}
		if len(line) == 0 {
			continue
		}
		stats.requests++
		if framing == bridge.StdioFramingContentLength {
			stats.contentLengthFraming++
		} else {
			stats.lineFraming++
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			stats.parseErrors++
			sendBridgeParseError(line, err, framing)
			signalResponseSent()
			continue
		}
		if req.HasInvalidID() {
			stats.invalidIDs++
			sendBridgeError(nil, -32600, "Invalid Request: id must be string or number when present", framing)
			signalResponseSent()
			continue
		}
		debugf("request method=%s id=%v", req.Method, req.ID)
		stats.lastMethod = req.Method

		// FAST PATH: Handle initialize and tools/list directly (no daemon needed)
		if handleFastPath(req, toolsList, framing) {
			stats.fastPath++
			signalResponseSent()
			continue
		}

		// RESTART FAST PATH: configure(action="restart") handled in bridge, not daemon
		if handleBridgeRestart(req, state, port, framing) {
			stats.fastPath++
			signalResponseSent()
			continue
		}

		// SLOW PATH: Check daemon status for tools/call and other methods.
		shouldForward := true
		if status := checkDaemonStatus(state, req, port); status != "" {
			if status == "method_not_found" {
				stats.methodNotFound++
			}
			if status == "starting" {
				stats.starting++
				// During startup, tools/call should wait-and-forward rather than
				// immediately returning a retry envelope to stdio clients.
				if req.Method == "tools/call" {
					shouldForward = true
				} else {
					shouldForward = false
				}
			} else {
				shouldForward = false
			}
			if !shouldForward {
				handleDaemonNotReady(req, status, signalResponseSent, framing)
				continue
			}
		}

		// Forward to HTTP server concurrently
		timeout := toolCallTimeout(req)
		reqCopy, lineCopy := req, append([]byte(nil), line...)
		stats.forwarded++
		wg.Add(1)
		util.SafeGo(func() {
			defer wg.Done()
			bridgeForwardRequest(client, endpoint, reqCopy, lineCopy, timeout, state, signalResponseSent, framing)
		})
	}

	close(pushRelayDone)
	bridgeShutdown(&wg, readErr, responseSent, stats)
}

// Forwarding, error responses, and restart handling moved to bridge_forward.go
