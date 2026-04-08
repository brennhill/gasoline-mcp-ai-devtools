// Purpose: Shared sequence constants and data types for configure sequence tooling.

package toolconfigure

import (
	"encoding/json"
	"regexp"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/toolconst"
)

const (
	SequenceNamespace  = "sequences"
	MaxSequenceSteps   = toolconst.MaxSequenceSteps
	MaxSequenceNameLen = 64
	DefaultStepTimeout = toolconst.DefaultStepTimeout
)

var SequenceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Sequence represents a named, replayable list of interact actions.
type Sequence struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	SavedAt     string            `json:"saved_at"`
	StepCount   int               `json:"step_count"`
	Steps       []json.RawMessage `json:"steps"`
}

// SequenceSummary is returned by list_sequences (omits step details).
type SequenceSummary struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	SavedAt     string   `json:"saved_at"`
	StepCount   int      `json:"step_count"`
}

// SequenceStepResult captures the outcome of one step during replay.
type SequenceStepResult struct {
	StepIndex     int    `json:"step_index"`
	Action        string `json:"action"`
	Status        string `json:"status"`
	DurationMs    int64  `json:"duration_ms"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Error         string `json:"error,omitempty"`
}

// Note: replayMu lives in the main package and is passed via Deps.ReplayMu.
// This prevents concurrent sequence replays and batch executions.
