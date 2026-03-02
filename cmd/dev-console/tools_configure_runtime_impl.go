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

func (h *ToolHandler) configureTelemetryImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.server == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Server not initialized", "Internal error — do not retry")}
	}

	var params struct {
		TelemetryMode string `json:"telemetry_mode"`
	}
	lenientUnmarshal(args, &params)

	if params.TelemetryMode == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Telemetry mode", map[string]any{
			"status":         "ok",
			"telemetry_mode": h.server.getTelemetryMode(),
		})}
	}

	mode, ok := normalizeTelemetryMode(params.TelemetryMode)
	if !ok {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Invalid telemetry_mode: "+params.TelemetryMode,
			"Use telemetry_mode: off, auto, or full",
			withParam("telemetry_mode"),
		)}
	}

	h.server.setTelemetryMode(mode)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Telemetry mode updated", map[string]any{
		"status":         "ok",
		"telemetry_mode": mode,
	})}
}

func (h *ToolHandler) configureSecurityModeImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	if h.capture == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrNotInitialized,
			"Capture subsystem not initialized",
			"Internal error — do not retry",
		)}
	}

	var params struct {
		Mode    string `json:"mode"`
		Confirm bool   `json:"confirm"`
	}
	lenientUnmarshal(args, &params)

	mode := strings.ToLower(strings.TrimSpace(params.Mode))
	if mode == "" {
		current, productionParity, rewrites := h.capture.GetSecurityMode()
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security mode", map[string]any{
			"status":                    "ok",
			"security_mode":             current,
			"production_parity":         productionParity,
			"insecure_rewrites_applied": rewrites,
			"requires_confirmation_for_insecure_mode": true,
		})}
	}

	switch mode {
	case capture.SecurityModeNormal:
		h.capture.SetSecurityMode(capture.SecurityModeNormal, nil)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             capture.SecurityModeNormal,
			"production_parity":         true,
			"insecure_rewrites_applied": []string{},
		})}
	case capture.SecurityModeInsecureProxy:
		if !params.Confirm {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
				ErrInvalidParam,
				"security_mode=insecure_proxy requires explicit confirmation",
				"Retry with confirm=true to acknowledge altered-environment debugging mode",
				withParam("confirm"),
			)}
		}
		rewrites := []string{"csp_headers"}
		h.capture.SetSecurityMode(capture.SecurityModeInsecureProxy, rewrites)
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Security mode updated", map[string]any{
			"status":                    "ok",
			"security_mode":             capture.SecurityModeInsecureProxy,
			"production_parity":         false,
			"insecure_rewrites_applied": rewrites,
			"warning":                   "Altered environment active. Findings are not production-parity evidence.",
		})}
	default:
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"Invalid security mode: "+params.Mode,
			"Use mode: normal or insecure_proxy",
			withParam("mode"),
		)}
	}
}

// configureRestartImpl handles restart requests that reach the daemon.
// Sends self-SIGTERM so the bridge auto-respawns a fresh daemon.
// This covers the case where the daemon is responsive but needs a clean restart.
func (h *ToolHandler) configureRestartImpl(req JSONRPCRequest) JSONRPCResponse {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Daemon restarting", map[string]any{
		"status":    "ok",
		"restarted": true,
		"message":   "Daemon shutting down — bridge will respawn automatically",
	})}

	// Send SIGTERM to self after a brief delay so the response is sent first.
	util.SafeGo(func() {
		time.Sleep(100 * time.Millisecond)
		p, _ := os.FindProcess(os.Getpid())
		_ = p.Signal(syscall.SIGTERM)
	})

	return resp
}

// configureStreamingWrapperImpl repackages streaming_action -> action for toolConfigureStreaming.
func (h *ToolHandler) configureStreamingWrapperImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	rewritten, err := cfg.RewriteStreamingArgs(args)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}
	return h.toolConfigureStreaming(req, rewritten)
}

func (h *ToolHandler) configureTestBoundaryStartImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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

func (h *ToolHandler) configureTestBoundaryEndImpl(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
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
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
			ErrInvalidParam,
			"No active test boundary for test_id '"+result.TestID+"'",
			"Call configure({what: 'test_boundary_start', test_id: '"+result.TestID+"'}) first",
			withParam("test_id"),
		)}
	}

	return cfg.BuildTestBoundaryEndResponse(req.ID, result, wasActive)
}
