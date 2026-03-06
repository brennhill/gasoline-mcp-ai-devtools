// Purpose: Provides fragile-selector detection for playback actions to flag selectors likely to break across runs.
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
