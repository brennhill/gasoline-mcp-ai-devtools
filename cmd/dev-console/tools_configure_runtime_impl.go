// Purpose: Implements configure runtime controls (telemetry, security mode, restart, streaming/test boundary wrappers).
// Why: Isolates runtime/environment mutations from the configure router.

package main

import (
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"
	cfg "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/tools/configure"
	"github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/util"
)

const (
	// restartSelfSignalDelay is the pause before sending SIGTERM to self during a
	// daemon restart, giving the JSON-RPC response time to flush to the client.
	restartSelfSignalDelay = 100 * time.Millisecond
)

func (h *ToolHandler) toolConfigureTelemetry(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.server == nil {
		return fail(req, ErrNotInitialized, "Server not initialized", "Internal error — do not retry")
	}

	var params struct {
		TelemetryMode string `json:"telemetry_mode"`
	}
	lenientUnmarshal(args, &params)

	if params.TelemetryMode == "" {
		return succeed(req, "Telemetry mode", map[string]any{
			"status":         "ok",
			"telemetry_mode": h.server.getTelemetryMode(),
		})
	}

	mode, ok := normalizeTelemetryMode(params.TelemetryMode)
	if !ok {
		return fail(req, ErrInvalidParam,
			"Invalid telemetry_mode: "+params.TelemetryMode,
			"Use telemetry_mode: off, auto, or full",
			withParam("telemetry_mode"))
	}

	h.server.setTelemetryMode(mode)
	return succeed(req, "Telemetry mode updated", map[string]any{
		"status":         "ok",
		"telemetry_mode": mode,
	})
}

func (h *ToolHandler) toolConfigureSecurityMode(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.capture == nil {
		return fail(req, ErrNotInitialized,
			"Capture subsystem not initialized",
			"Internal error — do not retry")
	}

	var params struct {
		Mode    string `json:"mode"`
		Confirm bool   `json:"confirm"`
	}
	lenientUnmarshal(args, &params)

	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if mode == "" {
		current, productionParity, rewrites := h.capture.GetSecurityMode()
		return succeed(req, "Security mode", map[string]any{
			"status":                    "ok",
			"security_mode":             current,
			"production_parity":         productionParity,
			"insecure_rewrites_applied": rewrites,
			"requires_confirmation_for_insecure_mode": true,
		})
	}

	switch mode {
	case capture.SecurityModeNormal:
		h.capture.SetSecurityMode(capture.SecurityModeNormal, nil)
		return succeed(req, "Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             capture.SecurityModeNormal,
			"production_parity":         true,
			"insecure_rewrites_applied": []string{},
		})
	case capture.SecurityModeInsecureProxy:
		if !params.Confirm {
			return fail(req, ErrInvalidParam,
				"security_mode=insecure_proxy requires explicit confirmation",
				"Retry with confirm=true to acknowledge altered-environment debugging mode",
				withParam("confirm"))
		}
		rewrites := []string{"csp_headers"}
		h.capture.SetSecurityMode(capture.SecurityModeInsecureProxy, rewrites)
		return succeed(req, "Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             capture.SecurityModeInsecureProxy,
			"production_parity":         false,
			"insecure_rewrites_applied": rewrites,
			"warning":                   "Altered environment active. Findings are not production-parity evidence.",
		})
	default:
		return fail(req, ErrInvalidParam,
			"Invalid security mode: "+params.Mode,
			"Use mode: normal or insecure_proxy",
			withParam("mode"))
	}
}

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
