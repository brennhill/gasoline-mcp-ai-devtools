// Purpose: Re-exports canonical network/waterfall type aliases for capture package compatibility.
// Why: Keeps capture call sites stable while canonical type ownership lives in internal/types.
// Docs: docs/features/feature/normalized-event-schema/index.md

package capture

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/types"

// NetworkWaterfallEntry is an alias to canonical definition in internal/types/network.go
type NetworkWaterfallEntry = types.NetworkWaterfallEntry

// NetworkWaterfallPayload is an alias to canonical definition in internal/types/network.go
type NetworkWaterfallPayload = types.NetworkWaterfallPayload

// NetworkBody is an alias to canonical definition in internal/types/network.go
type NetworkBody = types.NetworkBody

// NetworkBodyFilter is an alias to canonical definition in internal/types/network.go
type NetworkBodyFilter = types.NetworkBodyFilter
