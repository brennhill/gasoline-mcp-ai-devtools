// tools_configure_sequence.go — Macro sequence CRUD and replay handlers.
// Implements save_sequence, get_sequence, list_sequences, delete_sequence, replay_sequence
// for the configure tool. Sequences are named lists of interact actions persisted
// via the session store and replayed through the existing interact handler.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Constants

const (
	sequenceNamespace = "sequences"
	maxSequenceSteps  = 50
	maxSequenceNameLen = 64
	defaultStepTimeout = 10000 // ms
)

var sequenceNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Types

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
	StepIndex  int    `json:"step_index"`
	Action     string `json:"action"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Replay mutex prevents concurrent sequence replays.
var replayMu sync.Mutex

// ============================================
// Save Sequence
// ============================================

func (h *ToolHandler) toolConfigureSaveSequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Tags        []string          `json:"tags"`
		Steps       []json.RawMessage `json:"steps"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	// Validate name
	if params.Name == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'name' is missing", "Add the 'name' parameter", withParam("name"))}
	}
	if len(params.Name) > maxSequenceNameLen {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, fmt.Sprintf("Name exceeds maximum length of %d characters", maxSequenceNameLen), "Use a shorter name", withParam("name"))}
	}
	if !sequenceNamePattern.MatchString(params.Name) {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Name must match ^[a-zA-Z0-9_-]+$", "Use only alphanumeric characters, hyphens, and underscores", withParam("name"))}
	}

	// Validate steps
	if len(params.Steps) == 0 {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Steps must be a non-empty array", "Add at least one step", withParam("steps"))}
	}
	if len(params.Steps) > maxSequenceSteps {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, fmt.Sprintf("Steps exceeds maximum of %d", maxSequenceSteps), "Split into smaller sequences", withParam("steps"))}
	}

	// Validate each step has a what (or action) field
	for i, step := range params.Steps {
		var s struct {
			What   string `json:"what"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal(step, &s); err != nil || (s.What == "" && s.Action == "") {
			return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, fmt.Sprintf("Step[%d] missing required 'what' field", i), "Add a 'what' field to each step", withParam("steps"))}
		}
	}

	// Build and persist
	seq := Sequence{
		Name:        params.Name,
		Description: params.Description,
		Tags:        params.Tags,
		SavedAt:     time.Now().UTC().Format(time.RFC3339),
		StepCount:   len(params.Steps),
		Steps:       params.Steps,
	}

	data, err := json.Marshal(seq)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Failed to serialize sequence: "+err.Error(), "Check step format")}
	}

	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	if err := h.sessionStoreImpl.Save(sequenceNamespace, params.Name, data); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Failed to save sequence: "+err.Error(), "Check disk space")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Sequence saved", map[string]any{
		"status":     "saved",
		"name":       seq.Name,
		"step_count": seq.StepCount,
		"saved_at":   seq.SavedAt,
		"message":    fmt.Sprintf("Sequence saved: %s (%d steps)", seq.Name, seq.StepCount),
	})}
}

// ============================================
// Get Sequence
// ============================================

func (h *ToolHandler) toolConfigureGetSequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name string `json:"name"`
	}
	lenientUnmarshal(args, &params)

	if params.Name == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'name' is missing", "Add the 'name' parameter", withParam("name"))}
	}

	seq, errResp := h.loadSequence(req, params.Name)
	if errResp != nil {
		return *errResp
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Sequence details", map[string]any{
		"status":      "ok",
		"name":        seq.Name,
		"description": seq.Description,
		"tags":        seq.Tags,
		"saved_at":    seq.SavedAt,
		"step_count":  seq.StepCount,
		"steps":       seq.Steps,
	})}
}

// ============================================
// List Sequences
// ============================================

func (h *ToolHandler) toolConfigureListSequences(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Tags []string `json:"tags"`
	}
	lenientUnmarshal(args, &params)

	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	keys, err := h.sessionStoreImpl.List(sequenceNamespace)
	if err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Sequences", map[string]any{
			"status":    "ok",
			"sequences": []any{},
			"count":     0,
		})}
	}

	summaries := make([]SequenceSummary, 0, len(keys))
	for _, key := range keys {
		data, loadErr := h.sessionStoreImpl.Load(sequenceNamespace, key)
		if loadErr != nil {
			continue
		}
		var seq Sequence
		if json.Unmarshal(data, &seq) != nil {
			continue
		}

		// Tag filter: sequence must have ALL requested tags
		if len(params.Tags) > 0 && !hasAllTags(seq.Tags, params.Tags) {
			continue
		}

		summaries = append(summaries, SequenceSummary{
			Name:        seq.Name,
			Description: seq.Description,
			Tags:        seq.Tags,
			SavedAt:     seq.SavedAt,
			StepCount:   seq.StepCount,
		})
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Sequences", map[string]any{
		"status":    "ok",
		"sequences": summaries,
		"count":     len(summaries),
	})}
}

// ============================================
// Delete Sequence
// ============================================

func (h *ToolHandler) toolConfigureDeleteSequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name string `json:"name"`
	}
	lenientUnmarshal(args, &params)

	if params.Name == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'name' is missing", "Add the 'name' parameter", withParam("name"))}
	}

	if h.sessionStoreImpl == nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
	}

	// Check existence first for better error message
	if _, err := h.sessionStoreImpl.Load(sequenceNamespace, params.Name); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Sequence not found: "+params.Name, "Use list_sequences to see available sequences")}
	}

	if err := h.sessionStoreImpl.Delete(sequenceNamespace, params.Name); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Failed to delete sequence: "+err.Error(), "Try again")}
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Sequence deleted", map[string]any{
		"status":  "deleted",
		"name":    params.Name,
		"message": "Sequence deleted: " + params.Name,
	})}
}

// ============================================
// Replay Sequence
// ============================================

func (h *ToolHandler) toolConfigureReplaySequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params struct {
		Name           string            `json:"name"`
		OverrideSteps  []json.RawMessage `json:"override_steps"`
		StepTimeoutMs  int               `json:"step_timeout_ms"`
		ContinueOnErr  *bool             `json:"continue_on_error"`
		StopAfterStep  int               `json:"stop_after_step"`
	}
	lenientUnmarshal(args, &params)

	if params.Name == "" {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'name' is missing", "Add the 'name' parameter", withParam("name"))}
	}

	seq, errResp := h.loadSequence(req, params.Name)
	if errResp != nil {
		return *errResp
	}

	// Validate override_steps length
	if params.OverrideSteps != nil && len(params.OverrideSteps) != seq.StepCount {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, fmt.Sprintf("override_steps length (%d) does not match sequence step count (%d)", len(params.OverrideSteps), seq.StepCount), "Fix array length to match step count", withParam("override_steps"))}
	}

	// Default continue_on_error to true
	continueOnError := true
	if params.ContinueOnErr != nil {
		continueOnError = *params.ContinueOnErr
	}

	if params.StepTimeoutMs <= 0 {
		params.StepTimeoutMs = defaultStepTimeout
	}

	// Acquire replay mutex
	if !replayMu.TryLock() {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Another sequence is currently replaying", "Wait for it to complete")}
	}
	defer replayMu.Unlock()

	// Record audit trail
	h.recordAIAction("replay_sequence", "", map[string]any{"name": params.Name, "steps": seq.StepCount})

	start := time.Now()
	results := make([]SequenceStepResult, 0, seq.StepCount)
	stepsExecuted := 0
	stepsFailed := 0
	maxSteps := seq.StepCount
	if params.StopAfterStep > 0 && params.StopAfterStep < maxSteps {
		maxSteps = params.StopAfterStep
	}

	for i := 0; i < maxSteps; i++ {
		stepArgs := seq.Steps[i]
		// Apply override if present and non-null
		if params.OverrideSteps != nil && string(params.OverrideSteps[i]) != "null" {
			stepArgs = params.OverrideSteps[i]
		}

		// Extract action name for result
		var stepAction struct {
			What   string `json:"what"`
			Action string `json:"action"`
		}
		json.Unmarshal(stepArgs, &stepAction) //nolint:errcheck // best-effort extraction
		actionName := stepAction.What
		if actionName == "" {
			actionName = stepAction.Action
		}

		stepStart := time.Now()
		stepResp := h.toolInteract(req, stepArgs)
		stepDuration := time.Since(stepStart).Milliseconds()

		stepResult := SequenceStepResult{
			StepIndex:  i,
			Action:     actionName,
			DurationMs: stepDuration,
		}

		if isErrorResponse(stepResp) {
			stepResult.Status = "error"
			stepResult.Error = extractErrorMessage(stepResp)
			stepsFailed++
			results = append(results, stepResult)
			stepsExecuted++
			if !continueOnError {
				break
			}
			continue
		}

		stepResult.Status = "ok"
		stepsExecuted++
		results = append(results, stepResult)
	}

	totalDuration := time.Since(start).Milliseconds()

	status := "ok"
	var message string
	if stepsFailed > 0 && stepsExecuted < seq.StepCount {
		status = "error"
		message = fmt.Sprintf("Sequence failed at step %d/%d", stepsExecuted, seq.StepCount)
	} else if stepsFailed > 0 {
		status = "partial"
		message = fmt.Sprintf("Sequence partially replayed: %d/%d steps executed, %d failed", stepsExecuted-stepsFailed, seq.StepCount, stepsFailed)
	} else {
		message = fmt.Sprintf("Sequence replayed: %d/%d steps executed in %dms", stepsExecuted, seq.StepCount, totalDuration)
	}

	responseData := map[string]any{
		"status":         status,
		"name":           params.Name,
		"steps_executed": stepsExecuted,
		"steps_failed":   stepsFailed,
		"steps_total":    seq.StepCount,
		"duration_ms":    totalDuration,
		"results":        results,
		"message":        message,
	}

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Sequence replay", responseData)}
}

// ============================================
// Helpers
// ============================================

// loadSequence loads a sequence from the session store and returns it.
func (h *ToolHandler) loadSequence(req JSONRPCRequest, name string) (*Sequence, *JSONRPCResponse) {
	if h.sessionStoreImpl == nil {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNotInitialized, "Session store not initialized", "Internal error — do not retry")}
		return nil, &resp
	}

	data, err := h.sessionStoreImpl.Load(sequenceNamespace, name)
	if err != nil {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrNoData, "Sequence not found: "+name, "Use list_sequences to see available sequences")}
		return nil, &resp
	}

	var seq Sequence
	if err := json.Unmarshal(data, &seq); err != nil {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Corrupted sequence data: "+err.Error(), "Delete and re-save the sequence")}
		return nil, &resp
	}

	return &seq, nil
}

// hasAllTags returns true if seqTags contains all of requiredTags.
func hasAllTags(seqTags, requiredTags []string) bool {
	tagSet := make(map[string]bool, len(seqTags))
	for _, t := range seqTags {
		tagSet[t] = true
	}
	for _, req := range requiredTags {
		if !tagSet[req] {
			return false
		}
	}
	return true
}

// extractErrorMessage extracts the error message text from an error response.
func extractErrorMessage(resp JSONRPCResponse) string {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "unknown error"
	}
	if len(result.Content) > 0 {
		text := result.Content[0].Text
		// Try to extract message from structured error JSON
		var errData struct {
			Message string `json:"message"`
		}
		jsonStart := strings.Index(text, "{")
		if jsonStart >= 0 {
			if json.Unmarshal([]byte(text[jsonStart:]), &errData) == nil && errData.Message != "" {
				return errData.Message
			}
		}
		return text
	}
	return "unknown error"
}
