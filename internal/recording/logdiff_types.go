// Purpose: Compares two recordings to produce a log diff: new errors, missing events, changed values, and action stats.
// Docs: docs/features/feature/playback-engine/index.md

package recording

type LogDiffResult struct {
	Status            string
	OriginalRecording string
	ReplayRecording   string
	Summary           string
	NewErrors         []DiffLogEntry
	MissingEvents     []DiffLogEntry
	ChangedValues     []ValueChange
	ActionStats       ActionComparison
}

type DiffLogEntry struct {
	Type       string
	Severity   string
	Level      string
	Message    string
	Timestamp  int64
	Selector   string
	ActionType string
}

type ValueChange struct {
	Field     string
	FromValue string
	ToValue   string
	Timestamp int64
}

type ActionComparison struct {
	OriginalCount     int
	ReplayCount       int
	ErrorsOriginal    int
	ErrorsReplay      int
	ClicksOriginal    int
	ClicksReplay      int
	TypesOriginal     int
	TypesReplay       int
	NavigatesOriginal int
	NavigatesReplay   int
}
