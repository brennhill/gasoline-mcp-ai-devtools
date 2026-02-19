// deps.go — Composable dependency interfaces for MCP tool packages.
// Each tool package defines its own Deps interface by embedding these sub-interfaces.
// *ToolHandler satisfies all of them with zero code changes.
package mcp

import "encoding/json"

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
