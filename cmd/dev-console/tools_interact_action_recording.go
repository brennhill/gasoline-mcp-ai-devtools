// Purpose: Records interact actions in AI-enhanced action history buffers.
// Why: Isolates capture action-recording behavior from dispatch routing logic.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"time"

	"github.com/dev-console/dev-console/internal/capture"
)

// recordAIAction records an AI-driven action to the enhanced actions buffer.
func (h *ToolHandler) recordAIAction(actionType string, url string, details map[string]any) {
	action := capture.EnhancedAction{
		Type:      actionType,
		Timestamp: time.Now().UnixMilli(),
		URL:       url,
		Source:    "ai",
	}
	if len(details) > 0 {
		action.Selectors = details
	}
	h.capture.AddEnhancedActions([]capture.EnhancedAction{action})
}

// recordAIEnhancedAction records a fully populated AI-driven action.
func (h *ToolHandler) recordAIEnhancedAction(action capture.EnhancedAction) {
	action.Timestamp = time.Now().UnixMilli()
	action.Source = "ai"
	h.capture.AddEnhancedActions([]capture.EnhancedAction{action})
}
