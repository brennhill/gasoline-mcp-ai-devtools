// playback.go — Helper methods for RecordingAction playback
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
