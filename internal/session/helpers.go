// helpers.go — Helper functions for session package.
// validateName, removeFromOrder, extractPath functions.
package session

import (
	"fmt"
	"strings"
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

// extractPath returns just the path component of a URL, stripping query params.
func ExtractURLPath(url string) string {
	if idx := strings.Index(url, "?"); idx >= 0 {
		return url[:idx]
	}
	return url
}
