// Purpose: Re-exports query lifecycle constants needed for capture-package compatibility tests.
// Why: Preserves legacy package-level references while query logic is owned by internal/queries.
// Docs: docs/features/feature/query-service/index.md

package capture

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/queries"

const (
	queryResultTTL = queries.QueryResultTTL // Re-export for queries_lifecycle_test.go
)
