// Purpose: Implements pilot/extension/csp/tab-tracking gate checks for tool handlers.
// Why: Keeps runtime precondition checks and recovery hints isolated from error alias definitions.

package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"
)

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
	return fail(req, ErrCodePilotDisabled, "AI Web Pilot is explicitly disabled",
		"Enable AI Web Pilot in the extension popup", opts...,
	), true
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
	// Use shutdownCtx so the wait aborts promptly when the server shuts down,
	// preventing goroutine leaks. Falls back to context.Background() if the
	// handler was constructed without a shutdown context (e.g., in tests).
	ctx := h.shutdownCtx
	if ctx == nil {
		ctx = context.Background()
	}
	if h.capture.WaitForExtensionConnected(ctx, timeout) {
		return JSONRPCResponse{}, false
	}
	opts := append([]func(*StructuredError){
		h.diagnosticHint(),
		withRetryable(true),
		withRetryAfterMs(3000),
		withRecoveryToolCall(map[string]any{
			"tool":      "observe",
			"arguments": map[string]any{"what": "pilot"},
		}),
	}, extraOpts...)
	return fail(req, ErrNoData, "Extension not connected. Commands cannot be dispatched.",
		"Check that the Kaboom browser extension is installed and the page is open.",
		opts...,
	), true
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
	// Recovery template: LLM should re-send its original call with world='auto'.
	// The 'script' param is intentionally omitted — the LLM fills it from its original call.
	return fail(req, ErrExtError,
		fmt.Sprintf("Page CSP blocks MAIN world script execution (level: %s). Use world='auto' or world='isolated' to bypass.", level),
		"Retry with world='auto' (falls back to isolated/structured), world='isolated' (DOM access, no page JS), or use DOM primitives (click, type).",
		h.diagnosticHint(),
		withRecoveryToolCall(map[string]any{
			"tool":      "interact",
			"arguments": map[string]any{"what": "execute_js", "world": "auto"},
		}),
	), true
}

// requireSessionStore returns (resp, true) if the session store is not initialized.
// Usage: if resp, blocked := h.requireSessionStore(req); blocked { return resp }
func (h *ToolHandler) requireSessionStore(req JSONRPCRequest) (JSONRPCResponse, bool) {
	if h.sessionStoreImpl != nil {
		return JSONRPCResponse{}, false
	}
	return fail(req, ErrNotInitialized, "Session store not initialized", "Internal error — do not retry"), true
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
	}, extraOpts...)
	return fail(req, ErrNoData, "No tab is being tracked. Navigate to a page first.",
		"Open a page in the browser, or call interact(what='navigate', url='...').",
		opts...,
	), true
}

// requireString returns (resp, true) if value is empty, short-circuiting the caller.
// Usage: if resp, blocked := requireString(req, params.Name, "name", "Add the 'name' parameter"); blocked { return resp }
func requireString(req JSONRPCRequest, value, paramName, hint string) (JSONRPCResponse, bool) {
	if value == "" {
		return fail(req, ErrMissingParam,
			fmt.Sprintf("Required parameter '%s' is missing", paramName),
			hint, withParam(paramName)), true
	}
	return JSONRPCResponse{}, false
}

// requireOneOf returns (resp, true) if value is not in validValues, short-circuiting the caller.
// Usage: if resp, blocked := requireOneOf(req, params.Mode, "mode", []string{"a","b"}, "Use a valid mode"); blocked { return resp }
func requireOneOf(req JSONRPCRequest, value string, paramName string, validValues []string, hint string) (JSONRPCResponse, bool) {
	for _, v := range validValues {
		if value == v {
			return JSONRPCResponse{}, false
		}
	}
	return fail(req, ErrMissingParam,
		fmt.Sprintf("Parameter '%s' must be one of: %s", paramName, strings.Join(validValues, ", ")),
		hint, withParam(paramName)), true
}
