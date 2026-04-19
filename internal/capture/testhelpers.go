// Purpose: Provides test fixture constructors shared by capture package tests.
// Why: Reduces duplicated test bootstrapping code and keeps test setup behavior consistent.
// Docs: docs/features/feature/self-testing/index.md

package capture

import (
	"testing"
)

// setupTestCapture creates a new Capture instance for testing.
func setupTestCapture(t *testing.T) *Capture {
	t.Helper()
	return NewCapture()
}
