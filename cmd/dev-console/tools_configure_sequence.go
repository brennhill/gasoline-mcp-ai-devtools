// Purpose: Implements save_sequence, get_sequence, list_sequences, delete_sequence, and replay_sequence for reusable interact macros.
// Why: Enables agents to record and replay named action sequences across sessions without re-specifying steps.
// Docs: docs/features/feature/batch-sequences/index.md

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

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
