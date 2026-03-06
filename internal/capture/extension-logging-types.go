// Purpose: Re-exports canonical extension logging type aliases for capture package compatibility.
// Why: Keeps capture call sites stable while canonical type ownership lives in internal/types.
// Docs: docs/features/feature/backend-log-streaming/index.md

package capture

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/types"

// ExtensionLog is an alias to canonical definition in internal/types/log.go
type ExtensionLog = types.ExtensionLog

// PollingLogEntry is an alias to canonical definition in internal/types/log.go
type PollingLogEntry = types.PollingLogEntry

// HTTPDebugEntry is an alias to canonical definition in internal/types/log.go
type HTTPDebugEntry = types.HTTPDebugEntry
