// Purpose: Owns types.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// types.go — Query-related types for extension-server RPC
// PendingQuery, PendingQueryResponse, and CommandResult handle the async
// query mechanism where MCP server sends queries to the browser extension
// and waits for responses.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package queries

import (
	"encoding/json"
	"time"
)

// ============================================
// Query Types
// ============================================

// PendingQuery represents a query waiting for extension response
type PendingQuery struct {
	Type          string          `json:"type"`
	Params        json.RawMessage `json:"params"`
	TabID         int             `json:"tab_id,omitempty"`         // Target tab ID (0 = active tab)
	CorrelationID string          `json:"correlation_id,omitempty"` // LLM-facing tracking ID for async commands
}

// PendingQueryResponse is the response format for pending queries
type PendingQueryResponse struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	Params        json.RawMessage `json:"params"`
	TabID         int             `json:"tab_id,omitempty"`         // Target tab ID (0 = active tab)
	CorrelationID string          `json:"correlation_id,omitempty"` // LLM-facing tracking ID for async commands
}

// CommandResult represents the result of an async command execution
type CommandResult struct {
	CorrelationID string          `json:"correlation_id"`
	Status        string          `json:"status"` // "pending", "complete", "error", "timeout", "expired", "cancelled"
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
	CompletedAt   time.Time       `json:"completed_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	ExpiresAt     time.Time       `json:"expires_at,omitempty"` // hard deadline to avoid stuck-pending commands
}

// ElapsedMs returns milliseconds from creation to completion (or now if still pending).
func (cr *CommandResult) ElapsedMs() int64 {
	end := cr.CompletedAt
	if end.IsZero() {
		end = time.Now()
	}
	return end.Sub(cr.CreatedAt).Milliseconds()
}

// ============================================
// Constants
// ============================================

const (
	// DefaultQueryTimeout is the default timeout for extension queries.
	// Extension polls every 1-2s, fast timeout prevents MCP hangs.
	DefaultQueryTimeout = 2 * time.Second

	// AsyncCommandTimeout is the timeout for async commands (execute_js, browser actions).
	// Commands are queued and polled by the extension, then completed asynchronously.
	// This timeout must be >= extension execution timeout to avoid premature expiration.
	AsyncCommandTimeout = 60 * time.Second
)
