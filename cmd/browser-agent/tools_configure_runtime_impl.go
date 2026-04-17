// Purpose: Implements configure runtime controls (restart, test boundary wrappers).
// Why: Isolates runtime/environment mutations from the configure router.

package main

import (
	"encoding/json"
	"os"
	"syscall"
	"time"

	cfg "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/tools/configure"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/util"
)

const (
	// restartSelfSignalDelay is the pause before sending SIGTERM to self during a
	// daemon restart, giving the JSON-RPC response time to flush to the client.
	restartSelfSignalDelay = 100 * time.Millisecond
)

// toolConfigureRestart handles restart requests that reach the daemon.
// Sends self-SIGTERM so the bridge auto-respawns a fresh daemon.
// This covers the case where the daemon is responsive but needs a clean restart.
func (h *ToolHandler) toolConfigureRestart(req JSONRPCRequest) JSONRPCResponse {
	resp := succeed(req, "Daemon restarting", map[string]any{
		"status":    "ok",
		"restarted": true,
		"message":   "Daemon shutting down — bridge will respawn automatically",
	})

	// Send SIGTERM to self after a brief delay so the response is sent first.
	util.SafeGo(func() {
		time.Sleep(restartSelfSignalDelay)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	})

	return resp
}


func (h *ToolHandler) toolConfigureTestBoundaryStart(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, errResp := cfg.ParseTestBoundaryStart(req.ID, args)
	if errResp != nil {
		return *errResp
	}

	// Track the active boundary.
	h.activeBoundariesMu.Lock()
	defer h.activeBoundariesMu.Unlock()
	if h.activeBoundaries == nil {
		h.activeBoundaries = make(map[string]time.Time)
	}
	h.activeBoundaries[result.TestID] = time.Now()

	return cfg.BuildTestBoundaryStartResponse(req.ID, result)
}

func (h *ToolHandler) toolConfigureTestBoundaryEnd(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	result, errResp := cfg.ParseTestBoundaryEnd(req.ID, args)
	if errResp != nil {
		return *errResp
	}

	// Check if this boundary was actually started.
	wasActive := func() bool {
		h.activeBoundariesMu.Lock()
		defer h.activeBoundariesMu.Unlock()
		_, active := h.activeBoundaries[result.TestID]
		if active {
			delete(h.activeBoundaries, result.TestID)
		}
		return active
	}()

	if !wasActive {
		return fail(req, ErrInvalidParam,
			"No active test boundary for test_id '"+result.TestID+"'",
			"Call configure({what: 'test_boundary_start', test_id: '"+result.TestID+"'}) first",
			withParam("test_id"))
	}

	return cfg.BuildTestBoundaryEndResponse(req.ID, result, wasActive)
}
