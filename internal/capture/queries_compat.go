// queries_compat.go — Unexported re-exports for backward compatibility.
// Provides capture-package-level access to constants and functions that moved
// to internal/queries but are referenced by tests remaining in this package.
package capture

import "github.com/dev-console/dev-console/internal/queries"

const (
	queryResultTTL = queries.QueryResultTTL // Re-export for queries_lifecycle_test.go
)
