// ai_checkpoint_e2e_test.go — E2E tests for get_changes_since via MCP JSON-RPC
// SKIPPED: These tests require MCPHandler and JSONRPCRequest types from cmd/dev-console.
// Architectural refactoring needed to move these types to internal packages.
package ai

import (
	"testing"
)

// TestE2E_GetChangesSinceSkipped — Placeholder to mark e2e tests as skipped
func TestE2E_GetChangesSinceSkipped(t *testing.T) {
	t.Skip("E2E MCP tests require cmd/dev-console types (MCPHandler, JSONRPCRequest) - requires architectural refactoring")
}
