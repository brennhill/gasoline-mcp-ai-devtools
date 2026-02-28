// Purpose: Re-exports canonical enhanced action type aliases for capture package compatibility.
// Why: Keeps capture call sites stable while canonical type ownership lives in internal/types.
// Docs: docs/features/feature/normalized-event-schema/index.md

package capture

import "github.com/dev-console/dev-console/internal/types"

// EnhancedAction is an alias to canonical definition in internal/types/network.go
type EnhancedAction = types.EnhancedAction

// EnhancedActionFilter is an alias to canonical definition in internal/types/network.go
type EnhancedActionFilter = types.EnhancedActionFilter
