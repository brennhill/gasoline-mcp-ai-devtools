// Purpose: Test aliases for evidence types/functions that moved to internal/toolinteract.

package main

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolinteract"
)

// evidenceShot aliases the exported type from toolinteract.
type evidenceShot = toolinteract.EvidenceShot

// evidenceCaptureFn is a shim that mimics the old package-level var.
// Tests read/write this var; the setter syncs to toolinteract.SetEvidenceCaptureFn.
var evidenceCaptureFn func(*ToolHandler, string) evidenceShot

// syncEvidenceCaptureFn installs the current evidenceCaptureFn into toolinteract.
// Call after assigning evidenceCaptureFn in a test.
func syncEvidenceCaptureFn() {
	if evidenceCaptureFn == nil {
		toolinteract.ResetEvidenceCaptureFn()
		return
	}
	fn := evidenceCaptureFn
	toolinteract.SetEvidenceCaptureFn(func(_ *toolinteract.Deps, clientID string) toolinteract.EvidenceShot {
		return fn(nil, clientID)
	})
}
