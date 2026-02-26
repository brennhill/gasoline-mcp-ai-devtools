package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dev-console/dev-console/internal/capture"
	"github.com/dev-console/dev-console/internal/mcp"
)

// Error code aliases — all callers in package main use these unchanged.
const (
	ErrInvalidJSON          = mcp.ErrInvalidJSON
	ErrMissingParam         = mcp.ErrMissingParam
	ErrInvalidParam         = mcp.ErrInvalidParam
	ErrUnknownMode          = mcp.ErrUnknownMode
	ErrPathNotAllowed       = mcp.ErrPathNotAllowed
	ErrNotInitialized       = mcp.ErrNotInitialized
	ErrNoData               = mcp.ErrNoData
	ErrCodePilotDisabled    = mcp.ErrCodePilotDisabled
	ErrOsAutomationDisabled = mcp.ErrOsAutomationDisabled
	ErrRateLimited          = mcp.ErrRateLimited
	ErrCursorExpired        = mcp.ErrCursorExpired
	ErrExtTimeout           = mcp.ErrExtTimeout
	ErrExtError             = mcp.ErrExtError
	ErrInternal             = mcp.ErrInternal
	ErrMarshalFailed        = mcp.ErrMarshalFailed
	ErrExportFailed         = mcp.ErrExportFailed
)

// StructuredError alias.
type StructuredError = mcp.StructuredError

func mcpStructuredError(code, message, retry string, opts ...func(*StructuredError)) json.RawMessage {
	return mcp.StructuredErrorResponse(code, message, retry, opts...)
}

func withParam(p string) func(*StructuredError)    { return mcp.WithParam(p) }
func withHint(h string) func(*StructuredError)     { return mcp.WithHint(h) }
func withAction(a string) func(*StructuredError)   { return mcp.WithAction(a) }
func withSelector(s string) func(*StructuredError) { return mcp.WithSelector(s) }
func withRetryable(retryable bool) func(*StructuredError) {
	return mcp.WithRetryable(retryable)
}
func withRetryAfterMs(ms int) func(*StructuredError) { return mcp.WithRetryAfterMs(ms) }
func withFinal(final bool) func(*StructuredError)    { return mcp.WithFinal(final) }
func withRecoveryToolCall(toolCall map[string]any) func(*StructuredError) {
	return mcp.WithRecoveryToolCall(toolCall)
}

func retryDefaultsForCode(code string) []func(*StructuredError) {
	return mcp.RetryDefaultsForCode(code)
}

func appendCanonicalWhatAliasWarning(resp JSONRPCResponse, aliasParam, mode string) JSONRPCResponse {
	if strings.TrimSpace(aliasParam) == "" || strings.TrimSpace(mode) == "" {
		return resp
	}
	warning := fmt.Sprintf("Accepted alias parameter '%s'; canonical parameter is 'what' (use what=%q).", aliasParam, mode)
	return appendWarningsToResponse(resp, []string{warning})
}

func whatAliasConflictResponse(req JSONRPCRequest, aliasParam, whatValue, aliasValue, validValues string) JSONRPCResponse {
	hint := "Use only 'what' when specifying tool mode/action."
	if strings.TrimSpace(validValues) != "" {
		hint += " Valid values: " + validValues
	}
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpStructuredError(
			ErrInvalidParam,
			fmt.Sprintf("Conflicting parameters: what=%q and %s=%q", whatValue, aliasParam, aliasValue),
			"Send only the canonical 'what' parameter and retry.",
			withParam("what"),
			withHint(hint),
		),
	}
}

// diagnosticHintString returns a plain-text snapshot of system state.
// Used by both structured errors and JSON error responses.
func (h *ToolHandler) DiagnosticHintString() string {
	extConnected := h.capture.IsExtensionConnected()
	pilotEnabled := h.capture.IsPilotEnabled()
	pilotState := ""
	if status, ok := h.capture.GetPilotStatus().(map[string]any); ok {
		if state, ok := status["state"].(string); ok {
			pilotState = state
		}
		if effective, ok := status["enabled"].(bool); ok {
			pilotEnabled = effective
		}
	}
	enabled, tabID, tabURL := h.capture.GetTrackingStatus()

	var parts []string
	if extConnected {
		parts = append(parts, "extension=connected")
	} else {
		parts = append(parts, "extension=DISCONNECTED")
	}
	pilotToken := "pilot=DISABLED"
	switch pilotState {
	case "assumed_enabled":
		pilotToken = "pilot=ASSUMED_ENABLED(startup)"
	case "explicitly_disabled":
		pilotToken = "pilot=DISABLED(explicit)"
	case "enabled":
		pilotToken = "pilot=enabled"
	default:
		if pilotEnabled {
			pilotToken = "pilot=enabled"
		}
	}
	parts = append(parts, pilotToken)
	if enabled && tabURL != "" {
		parts = append(parts, fmt.Sprintf("tracked_tab=%q (id=%d)", tabURL, tabID))
	} else {
		parts = append(parts, "tracked_tab=NONE")
	}

	cspRestricted, cspLevel := h.capture.GetCSPStatus()
	if cspRestricted {
		parts = append(parts, fmt.Sprintf("csp=RESTRICTED(%s)", cspLevel))
	} else {
		parts = append(parts, "csp=clear")
	}

	hint := "Current state: " + parts[0]
	for _, p := range parts[1:] {
		hint += ", " + p
	}
	return hint
}

// diagnosticHint returns a snapshot of system state for inclusion in structured errors.
func (h *ToolHandler) diagnosticHint() func(*StructuredError) {
	return withHint(h.DiagnosticHintString())
}

// requirePilot returns (resp, true) if AI Web Pilot is disabled, short-circuiting the caller.
// Usage: if resp, blocked := h.requirePilot(req); blocked { return resp }
func (h *ToolHandler) requirePilot(req JSONRPCRequest, extraOpts ...func(*StructuredError)) (JSONRPCResponse, bool) {
	if h.capture.IsPilotActionAllowed() {
		return JSONRPCResponse{}, false
	}
	opts := append([]func(*StructuredError){
		h.diagnosticHint(),
		withRecoveryToolCall(map[string]any{
			"tool":      "observe",
			"arguments": map[string]any{"what": "pilot"},
		}),
	}, extraOpts...)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
		ErrCodePilotDisabled, "AI Web Pilot is explicitly disabled",
		"Enable AI Web Pilot in the extension popup", opts...,
	)}, true
}

// requireExtension returns (resp, true) if the browser extension is not connected,
// short-circuiting the caller with a structured error. On cold starts it waits up to
// ExtensionReadinessTimeout (5s) for the extension to connect before giving up.
// Usage: if resp, blocked := h.requireExtension(req); blocked { return resp }
func (h *ToolHandler) requireExtension(req JSONRPCRequest, extraOpts ...func(*StructuredError)) (JSONRPCResponse, bool) {
	timeout := h.extensionReadinessTimeout
	if timeout <= 0 {
		timeout = capture.ExtensionReadinessTimeout
	}
	// TODO(#302): Use request-scoped context once JSONRPCRequest carries one.
	// context.Background() means cold-start waits are not cancellable during shutdown.
	if h.capture.WaitForExtensionConnected(context.Background(), timeout) {
		return JSONRPCResponse{}, false
	}
	opts := append([]func(*StructuredError){
		h.diagnosticHint(),
		withRetryable(true),
		withRetryAfterMs(3000),
		withRecoveryToolCall(map[string]any{
			"tool":      "observe",
			"arguments": map[string]any{"what": "status"},
		}),
	}, extraOpts...)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
		ErrNoData, "Extension not connected. Commands cannot be dispatched.",
		"Check that the Gasoline browser extension is installed and the page is open.",
		opts...,
	)}, true
}

// requireCSPClear returns (resp, true) if the page's CSP blocks script execution
// for the given world. Only world="main" is blocked — "auto" and "isolated" bypass
// page CSP because the extension's ISOLATED world is not subject to page CSP, and
// "auto" falls back from MAIN → ISOLATED → structured executor automatically.
// Usage: if resp, blocked := h.requireCSPClear(req, world); blocked { return resp }
func (h *ToolHandler) requireCSPClear(req JSONRPCRequest, world string) (JSONRPCResponse, bool) {
	// Only MAIN world execution is blocked by page CSP.
	// ISOLATED world runs in the extension's security context (bypasses page CSP).
	// AUTO tries MAIN first, then falls back to ISOLATED/structured — the extension handles this.
	if world != "main" {
		return JSONRPCResponse{}, false
	}
	restricted, level := h.capture.GetCSPStatus()
	if !restricted {
		return JSONRPCResponse{}, false
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
		ErrExtError,
		fmt.Sprintf("Page CSP blocks MAIN world script execution (level: %s). Use world='auto' or world='isolated' to bypass.", level),
		"Retry with world='auto' (falls back to isolated/structured), world='isolated' (DOM access, no page JS), or use DOM primitives (click, type).",
		h.diagnosticHint(),
		withRecoveryToolCall(map[string]any{
			"tool":      "interact",
			"arguments": map[string]any{"what": "execute_js", "world": "auto"},
		}),
	)}, true
}

// requireTabTracking returns (resp, true) if no tab is being tracked,
// short-circuiting the caller with an immediate structured error (~5ms) instead of
// queuing a command that would time out or target the wrong tab.
// Usage: if resp, blocked := h.requireTabTracking(req); blocked { return resp }
func (h *ToolHandler) requireTabTracking(req JSONRPCRequest, extraOpts ...func(*StructuredError)) (JSONRPCResponse, bool) {
	enabled, _, _ := h.capture.GetTrackingStatus()
	if enabled {
		return JSONRPCResponse{}, false
	}
	opts := append([]func(*StructuredError){
		h.diagnosticHint(),
		withRetryable(true),
		withRetryAfterMs(2000),
		withRecoveryToolCall(map[string]any{
			"tool":      "interact",
			"arguments": map[string]any{"what": "navigate"},
		}),
	}, extraOpts...)
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(
		ErrNoData, "No tab is being tracked. Navigate to a page first.",
		"Open a page in the browser, or call interact(what='navigate', url='...').",
		opts...,
	)}, true
}

