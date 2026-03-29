// Purpose: Implement doc.go internal behavior used by MCP runtime features.
// Why: Maintains stable server behavior across tool and transport paths.
// Docs: docs/features/feature/pagination/index.md

// doc.go — Package documentation for multi-client session management.

// Package session provides multi-client session management for the Kaboom MCP server.
//
// Features:
//   - Client registration and isolation (each client has unique ID and working directory)
//   - Session state tracking (active, idle, stale detection)
//   - Last poll timestamp tracking for client health monitoring
//   - Thread-safe client registry with automatic cleanup
//   - Verification loop support for test flake detection
//
// The ClientRegistry maintains state for multiple Claude Code sessions connecting
// to a single Kaboom server instance. Each client is identified by X-Kaboom-Client
// header and maintains isolated state (current working directory, last poll time, etc.).
//
// Clients are considered stale if they haven't polled within 3 seconds.
package session
