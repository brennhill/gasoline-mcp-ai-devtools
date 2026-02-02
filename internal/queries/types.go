// types.go — Query-related types for extension-server RPC
// PendingQuery, PendingQueryResponse, and CommandResult handle the async
// query mechanism where MCP server sends queries to the browser extension
// and waits for responses.
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
	Status        string          `json:"status"` // "pending", "complete", "timeout", "expired"
	Result        json.RawMessage `json:"result,omitempty"`
	Error         string          `json:"error,omitempty"`
	CompletedAt   time.Time       `json:"completed_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// ============================================
// Constants
// ============================================

const (
	// DefaultQueryTimeout is the default timeout for extension queries.
	// Extension polls every 1-2s, fast timeout prevents MCP hangs.
	DefaultQueryTimeout = 2 * time.Second

	// AsyncCommandTimeout is the timeout for async commands (execute_js, browser actions).
	// Longer timeout allows extension to pick up commands even with network jitter.
	// These commands return immediately with correlation_id, so longer timeout doesn't block MCP.
	AsyncCommandTimeout = 30 * time.Second
)
