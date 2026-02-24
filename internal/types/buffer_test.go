// Purpose: Validate buffer_test.go behavior and guard against regressions.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/observe/index.md

package types

import "testing"

func TestBufferClearCountsTotal(t *testing.T) {
	t.Parallel()

	counts := &BufferClearCounts{
		NetworkWaterfall: 2,
		NetworkBodies:    3,
		WebSocketEvents:  4,
		WebSocketStatus:  5,
		Actions:          6,
		Logs:             7,
		ExtensionLogs:    8,
	}
	if got := counts.Total(); got != 35 {
		t.Fatalf("Total() = %d, want 35", got)
	}
}
