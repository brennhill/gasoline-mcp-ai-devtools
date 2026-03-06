// Purpose: Counts action types and builds type-value maps for recording diff analysis.
// Why: Provides reusable helper functions shared by diff comparison and report generation.
package recording

func CountActionTypes(actions []RecordingAction) (errors, clicks, types, navigates int) {
	for _, action := range actions {
		switch action.Type {
		case "error":
			errors++
		case "click":
			clicks++
		case "type":
			types++
		case "navigate":
			navigates++
		}
	}
	return
}

func BuildTypeValueMap(actions []RecordingAction) map[string]string {
	values := make(map[string]string)
	for _, action := range actions {
		if action.Type == "type" && action.Selector != "" {
			values[action.Selector] = action.Text
		}
	}
	return values
}

func (r *RecordingManager) CategorizeActionTypes(recording *Recording) map[string]int {
	counts := make(map[string]int)
	for _, action := range recording.Actions {
		counts[action.Type]++
	}
	return counts
}
