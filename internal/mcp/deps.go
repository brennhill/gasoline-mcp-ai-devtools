// deps.go — Composable dependency interfaces for MCP tool packages.
// Each tool package defines its own Deps interface by embedding these sub-interfaces.
// *ToolHandler satisfies all of them with zero code changes.
package mcp

import (
	"encoding/json"
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// DiagnosticProvider supplies system state snapshots for error messages.
// Used by all tools to attach "Current state: extension=connected, pilot=enabled, ..."
// hints to structured errors.
type DiagnosticProvider interface {
	DiagnosticHintString() string
}

// AsyncCommandDispatcher manages synchronous-by-default command execution.
// Used by analyze, generate, and interact tools that dispatch commands to
// the browser extension and wait for results.
type AsyncCommandDispatcher interface {
	MaybeWaitForCommand(req JSONRPCRequest, correlationID string, args json.RawMessage, queuedSummary string) JSONRPCResponse
}

// CaptureProvider gives access to the capture instance for buffer reads.
// Used by all 5 tools.
type CaptureProvider interface {
	GetCapture() *capture.Capture
}

// LogBufferReader provides read-only access to server log entries.
// Used by observe, generate (testgen), configure, and analyze (security audit).
type LogBufferReader interface {
	GetLogEntries() ([]LogEntry, []time.Time)
	GetLogTotalAdded() int64
}

// A11yQueryExecutor runs accessibility queries via the browser extension.
// Used by observe (accessibility), generate (SARIF), and analyze (accessibility).
type A11yQueryExecutor interface {
	ExecuteA11yQuery(scope string, tags []string, frame any, forceRefresh bool) (json.RawMessage, error)
}
