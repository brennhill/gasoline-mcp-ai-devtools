// tools_errors.go — Structured error handling (thin wrappers over internal/mcp).
// diagnosticHintString/diagnosticHint stay here (ToolHandler methods).
package main

import (
	"encoding/json"
	"fmt"

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

func withParam(p string) func(*StructuredError) { return mcp.WithParam(p) }
func withHint(h string) func(*StructuredError)  { return mcp.WithHint(h) }
func withRetryable(retryable bool) func(*StructuredError) {
	return mcp.WithRetryable(retryable)
}
func withRetryAfterMs(ms int) func(*StructuredError) { return mcp.WithRetryAfterMs(ms) }
func withFinal(final bool) func(*StructuredError) { return mcp.WithFinal(final) }

func retryDefaultsForCode(code string) []func(*StructuredError) {
	return mcp.RetryDefaultsForCode(code)
}

// diagnosticHintString returns a plain-text snapshot of system state.
// Used by both structured errors and JSON error responses.
func (h *ToolHandler) diagnosticHintString() string {
	extConnected := h.capture.IsExtensionConnected()
	pilotEnabled := h.capture.IsPilotEnabled()
	enabled, tabID, tabURL := h.capture.GetTrackingStatus()

	var parts []string
	if extConnected {
		parts = append(parts, "extension=connected")
	} else {
		parts = append(parts, "extension=DISCONNECTED")
	}
	if pilotEnabled {
		parts = append(parts, "pilot=enabled")
	} else {
		parts = append(parts, "pilot=DISABLED")
	}
	if enabled && tabURL != "" {
		parts = append(parts, fmt.Sprintf("tracked_tab=%q (id=%d)", tabURL, tabID))
	} else {
		parts = append(parts, "tracked_tab=NONE")
	}

	hint := "Current state: " + parts[0]
	for _, p := range parts[1:] {
		hint += ", " + p
	}
	return hint
}

// diagnosticHint returns a snapshot of system state for inclusion in structured errors.
func (h *ToolHandler) diagnosticHint() func(*StructuredError) {
	return withHint(h.diagnosticHintString())
}
