// Purpose: Implements replay_sequence execution with isolated parsing, planning,
// step execution, and summary shaping.
// Why: Keeps sequence CRUD logic and replay orchestration modular.

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type sequenceReplayParams struct {
	Name          string            `json:"name"`
	OverrideSteps []json.RawMessage `json:"override_steps"`
	StepTimeoutMs int               `json:"step_timeout_ms"`
	ContinueOnErr *bool             `json:"continue_on_error"`
	StopAfterStep int               `json:"stop_after_step"`
}

type sequenceReplayContext struct {
	ContinueOnError bool
	StepTimeout     time.Duration
	MaxSteps        int
}

type sequenceReplayMetrics struct {
	Executed int
	Failed   int
	Queued   int
}

// toolConfigureReplaySequence replays a named sequence by dispatching each step
// through toolInteract with forced async semantics.
func (h *ToolHandler) toolConfigureReplaySequence(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	params, errResp := parseReplaySequenceParams(req, args)
	if errResp != nil {
		return *errResp
	}

	seq, errResp := h.loadSequence(req, params.Name)
	if errResp != nil {
		return *errResp
	}

	ctx, errResp := buildReplayContext(req, seq, params)
	if errResp != nil {
		return *errResp
	}

	if !replayMu.TryLock() {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrInvalidParam, "Another sequence is currently replaying", "Wait for it to complete"),
		}
	}
	defer replayMu.Unlock()

	h.recordAIAction("replay_sequence", "", map[string]any{"name": params.Name, "steps": seq.StepCount})

	start := time.Now()
	results, metrics := h.executeReplaySteps(req, seq, params, ctx)
	totalDuration := time.Since(start).Milliseconds()
	status, message := summarizeReplayOutcome(seq.StepCount, metrics, totalDuration)

	responseData := map[string]any{
		"status":         status,
		"name":           params.Name,
		"steps_executed": metrics.Executed,
		"steps_failed":   metrics.Failed,
		"steps_queued":   metrics.Queued,
		"steps_total":    seq.StepCount,
		"duration_ms":    totalDuration,
		"results":        results,
		"message":        message,
	}

	return succeed(req, "Sequence replay", responseData)
}

func parseReplaySequenceParams(req JSONRPCRequest, args json.RawMessage) (sequenceReplayParams, *JSONRPCResponse) {
	var params sequenceReplayParams
	lenientUnmarshal(args, &params)
	if params.Name == "" {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mcpStructuredError(ErrMissingParam, "Required parameter 'name' is missing", "Add the 'name' parameter", withParam("name")),
		}
		return params, &resp
	}
	return params, nil
}

func buildReplayContext(req JSONRPCRequest, seq *Sequence, params sequenceReplayParams) (sequenceReplayContext, *JSONRPCResponse) {
	if params.OverrideSteps != nil && len(params.OverrideSteps) != seq.StepCount {
		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				fmt.Sprintf("override_steps length (%d) does not match sequence step count (%d)", len(params.OverrideSteps), seq.StepCount),
				"Fix array length to match step count",
				withParam("override_steps"),
			),
		}
		return sequenceReplayContext{}, &resp
	}

	continueOnError := true
	if params.ContinueOnErr != nil {
		continueOnError = *params.ContinueOnErr
	}

	stepTimeoutMs := params.StepTimeoutMs
	if stepTimeoutMs <= 0 {
		stepTimeoutMs = defaultStepTimeout
	}

	maxSteps := seq.StepCount
	if params.StopAfterStep > 0 && params.StopAfterStep < maxSteps {
		maxSteps = params.StopAfterStep
	}

	return sequenceReplayContext{
		ContinueOnError: continueOnError,
		StepTimeout:     time.Duration(stepTimeoutMs) * time.Millisecond,
		MaxSteps:        maxSteps,
	}, nil
}

func (h *ToolHandler) executeReplaySteps(req JSONRPCRequest, seq *Sequence, params sequenceReplayParams, ctx sequenceReplayContext) ([]SequenceStepResult, sequenceReplayMetrics) {
	results := make([]SequenceStepResult, 0, ctx.MaxSteps)
	metrics := sequenceReplayMetrics{}

	for i := 0; i < ctx.MaxSteps; i++ {
		stepArgs := resolveReplayStepArgs(seq, params, i)
		stepResult, stepRespIsError := h.executeReplayStep(req, stepArgs, i, ctx.StepTimeout)

		results = append(results, stepResult)
		metrics.Executed++
		switch stepResult.Status {
		case "queued":
			metrics.Queued++
		case "error":
			metrics.Failed++
		}

		if stepRespIsError && !ctx.ContinueOnError {
			break
		}
	}

	return results, metrics
}

func summarizeReplayOutcome(totalSteps int, metrics sequenceReplayMetrics, totalDurationMs int64) (string, string) {
	switch {
	case metrics.Failed > 0 && metrics.Executed < totalSteps:
		return "error", fmt.Sprintf("Sequence failed at step %d/%d", metrics.Executed, totalSteps)
	case metrics.Queued > 0 && metrics.Failed > 0:
		return "partial", fmt.Sprintf("Sequence replay queued with failures: %d queued, %d failed", metrics.Queued, metrics.Failed)
	case metrics.Queued > 0:
		return "queued", fmt.Sprintf("Sequence replay queued: %d/%d steps still running", metrics.Queued, totalSteps)
	case metrics.Failed > 0:
		return "partial", fmt.Sprintf("Sequence partially replayed: %d/%d steps executed, %d failed", metrics.Executed-metrics.Failed, totalSteps, metrics.Failed)
	default:
		return "ok", fmt.Sprintf("Sequence replayed: %d/%d steps executed in %dms", metrics.Executed, totalSteps, totalDurationMs)
	}
}
