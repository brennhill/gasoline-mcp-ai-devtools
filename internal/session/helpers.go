// Purpose: Implements session lifecycle, snapshots, and diff state management.
// Docs: docs/features/feature/observe/index.md
// Docs: docs/features/feature/pagination/index.md

// helpers.go — Helper functions for session package.
// validateName, removeFromOrder, extractPath functions.
package session

import (
	"fmt"

	"github.com/dev-console/dev-console/internal/util"
)

// validateName checks snapshot name constraints.
func (sm *SessionManager) validateName(name string) error {
	if name == "" {
		return fmt.Errorf("snapshot name cannot be empty")
	}
	if name == reservedSnapshotName {
		return fmt.Errorf("snapshot name %q is reserved", reservedSnapshotName)
	}
	if len(name) > maxSnapshotNameLen {
		return fmt.Errorf("snapshot name exceeds %d characters", maxSnapshotNameLen)
	}
	return nil
}

// removeFromOrder removes a name from the insertion order slice.
func (sm *SessionManager) removeFromOrder(name string) {
	for i, n := range sm.order {
		if n == name {
			newOrder := make([]string, len(sm.order)-1)
			copy(newOrder, sm.order[:i])
			copy(newOrder[i:], sm.order[i+1:])
			sm.order = newOrder
			return
		}
	}
}

// ExtractURLPath delegates to util.ExtractURLPath for URL path extraction.
func ExtractURLPath(rawURL string) string {
	return util.ExtractURLPath(rawURL)
}
