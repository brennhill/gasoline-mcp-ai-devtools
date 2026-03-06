// Purpose: Implements recording storage, replay engine execution, and diffing helpers.
// Why: Preserves traceability by storing replayable execution history and comparable outcomes.
// Docs: docs/features/feature/playback-engine/index.md

package recording

import "fmt"

// IsFragileSelectorAction checks if an action's selector is marked as fragile
func (action RecordingAction) IsFragileSelectorAction(fragileSelectors map[string]bool) bool {
	if action.Selector == "" {
		return false
	}

	key := fmt.Sprintf("%s:%s", "css", action.Selector)
	return fragileSelectors[key]
}
