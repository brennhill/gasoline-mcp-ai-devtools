// Purpose: Implements session lifecycle, snapshots, and diff state management.
// Docs: docs/features/feature/observe/index.md
// Docs: docs/features/feature/pagination/index.md

// constants.go — Session comparison constants.
// All configuration values for session package.
package session

const (
	// Snapshot name constraints
	maxSnapshotNameLen    = 50
	maxConsolePerSnapshot = 50
	maxNetworkPerSnapshot = 100
	reservedSnapshotName  = "current"

	// Performance regression threshold: >50% increase
	perfRegressionRatio = 1.5
)
