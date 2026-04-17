// mcp_identity.go - Canonical MCP identity constants and compatibility helpers.
// Purpose: Centralize server naming used by install/config/runtime handshake surfaces.

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/identity"

const (
	// mcpServerName is the canonical MCP server identity shown to clients and LLMs.
	mcpServerName = identity.MCPServerName
)

var legacyMCPServerNames = append([]string(nil), identity.LegacyMCPServerNames...)
