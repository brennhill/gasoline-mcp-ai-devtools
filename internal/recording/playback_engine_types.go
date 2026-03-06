// Purpose: Declares playback result types (PlaybackResult, Coordinates) used by the replay execution engine.
// Docs: docs/features/feature/playback-engine/index.md

package recording

import "time"

type PlaybackResult struct {
	Status          string
	ActionIndex     int
	ActionType      string
	SelectorUsed    string
	ExecutedAt      time.Time
	DurationMs      int64
	Error           string
	Coordinates     *Coordinates
	SelectorFragile bool
}

type Coordinates struct {
	X int
	Y int
}

type PlaybackSession struct {
	RecordingID      string
	StartedAt        time.Time
	Results          []PlaybackResult
	ActionsExecuted  int
	ActionsFailed    int
	SelectorFailures map[string]int
}
