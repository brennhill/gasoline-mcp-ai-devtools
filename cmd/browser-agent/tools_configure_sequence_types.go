// Purpose: Shared sequence constants and data types for configure sequence tooling.

package main

import (
	"encoding/json"
	"regexp"
	"sync"
)

const (
	sequenceNamespace  = "sequences"
	maxSequenceSteps   = 50
	maxSequenceNameLen = 64
	defaultStepTimeout = 10000 // ms
)

var sequenceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

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

// replayMu prevents concurrent sequence replays and batch executions.
// Shared between replay_sequence and batch execution paths.
var replayMu sync.Mutex
