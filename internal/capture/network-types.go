// Purpose: Owns network-types.go runtime behavior and integration logic.
// Docs: docs/features/feature/backend-log-streaming/index.md

// network-types.go — Network waterfall and body types.
// NetworkWaterfallEntry represents browser PerformanceResourceTiming data.
// NetworkBody is an alias to canonical definition in internal/types/network.go.
package capture

import "github.com/dev-console/dev-console/internal/types"

// NetworkWaterfallEntry is an alias to canonical definition in internal/types/network.go
type NetworkWaterfallEntry = types.NetworkWaterfallEntry

// NetworkWaterfallPayload is an alias to canonical definition in internal/types/network.go
type NetworkWaterfallPayload = types.NetworkWaterfallPayload

// NetworkBody is an alias to canonical definition in internal/types/network.go
type NetworkBody = types.NetworkBody

// NetworkBodyFilter is an alias to canonical definition in internal/types/network.go
type NetworkBodyFilter = types.NetworkBodyFilter
