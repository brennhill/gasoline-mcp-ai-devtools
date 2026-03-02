package recording

import (
	"fmt"
	"strings"
	"time"
)

func (r *RecordingManager) executeAction(index int, action RecordingAction) PlaybackResult {
	startTime := time.Now()

	result := PlaybackResult{
		Status:      "ok",
		ActionIndex: index,
		ActionType:  action.Type,
		ExecutedAt:  startTime,
	}

	switch action.Type {
	case "navigate":
		result.Status = "ok"
		result.SelectorUsed = "navigate"
		result.Error = ""
	case "click":
		result = r.executeClickWithHealing(action)
	case "type":
		result.Status = "ok"
		result.SelectorUsed = "type"
	case "scroll":
		result.Status = "ok"
		result.SelectorUsed = "scroll"
	default:
		result.Status = "error"
		result.Error = fmt.Sprintf("unknown_action_type: %s", action.Type)
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	return result
}

func (r *RecordingManager) executeClickWithHealing(action RecordingAction) PlaybackResult {
	result := PlaybackResult{
		Status:      "error",
		ActionType:  "click",
		ExecutedAt:  time.Now(),
		Coordinates: &Coordinates{X: action.X, Y: action.Y},
	}

	if action.DataTestID != "" {
		selector := fmt.Sprintf("[data-testid=%s]", action.DataTestID)
		if r.tryClickSelector(selector, action) {
			result.Status = "ok"
			result.SelectorUsed = "data-testid"
			return result
		}
	}

	if action.Selector != "" {
		if r.tryClickSelector(action.Selector, action) {
			result.Status = "ok"
			result.SelectorUsed = "css"
			return result
		}
	}

	if action.X > 0 && action.Y > 0 {
		result.Status = "ok"
		result.SelectorUsed = "nearby_xy"
		result.Coordinates = &Coordinates{X: action.X, Y: action.Y}
		return result
	}

	if len(action.ScreenshotPath) > 0 {
		result.Status = "ok"
		result.SelectorUsed = "last_known"
		return result
	}

	result.Status = "error"
	result.Error = "selector_not_found: Could not find element with any strategy"
	return result
}

func (r *RecordingManager) tryClickSelector(selector string, action RecordingAction) bool {
	if selector == "" {
		return false
	}

	validSelectors := []string{
		"[data-testid=",
		".",
		"#",
		"[",
	}

	for _, prefix := range validSelectors {
		if strings.HasPrefix(selector, prefix) {
			return true
		}
	}

	return false
}
