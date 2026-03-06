// Purpose: Step-level execution helpers for replay_sequence.

package main

import (
	"encoding/json"
	"strings"
	"time"
)

func resolveReplayStepArgs(seq *Sequence, params sequenceReplayParams, idx int) json.RawMessage {
	stepArgs := seq.Steps[idx]
	if params.OverrideSteps != nil && string(params.OverrideSteps[idx]) != "null" {
		stepArgs = params.OverrideSteps[idx]
	}
	return stepArgs
}

func (h *ToolHandler) executeReplayStep(req JSONRPCRequest, stepArgs json.RawMessage, stepIndex int, stepTimeout time.Duration) (SequenceStepResult, bool) {
	actionName := extractReplayActionName(stepArgs)
	replayStepArgs := forceReplayAsyncInteractStep(stepArgs)

	stepStart := time.Now()
	stepResp := h.toolInteract(req, replayStepArgs)
	stepDuration := time.Since(stepStart).Milliseconds()

	stepResult := SequenceStepResult{
		StepIndex:  stepIndex,
		Action:     actionName,
		DurationMs: stepDuration,
	}

	if corrID := extractCorrelationIDFromToolResponse(stepResp); corrID != "" {
		stepResult.CorrelationID = corrID
		if stepTimeout > 0 {
			cmd, found := h.capture.WaitForCommand(corrID, stepTimeout)
			if found {
				switch cmd.Status {
				case "pending":
					stepResult.Status = "queued"
				case "complete":
					stepResult.Status = "ok"
				default:
					stepResult.Status = "error"
					if cmd.Error != "" {
						stepResult.Error = cmd.Error
					} else {
						stepResult.Error = "command failed with status " + cmd.Status
					}
				}
			} else {
				stepResult.Status = "queued"
			}
		}
	}

	stepRespIsError := isErrorResponse(stepResp)
	if stepRespIsError && stepResult.Status == "" {
		stepResult.Status = "error"
		stepResult.Error = extractErrorMessage(stepResp)
	}
	if stepResult.Status == "" {
		stepResult.Status = "ok"
	}

	return stepResult, stepRespIsError
}

func extractReplayActionName(stepArgs json.RawMessage) string {
	var stepAction struct {
		What   string `json:"what"`
		Action string `json:"action"`
	}
	_ = json.Unmarshal(stepArgs, &stepAction) // best-effort extraction
	if stepAction.What != "" {
		return stepAction.What
	}
	return stepAction.Action
}

// forceReplayAsyncInteractStep ensures replayed interact steps do not block on
// extension execution. This keeps replay_sequence deterministic and avoids
// transport-level timeouts for long-running actions.
func forceReplayAsyncInteractStep(stepArgs json.RawMessage) json.RawMessage {
	var argsMap map[string]any
	if err := json.Unmarshal(stepArgs, &argsMap); err != nil {
		return stepArgs
	}
	argsMap["sync"] = false
	argsMap["wait"] = false
	updated, err := json.Marshal(argsMap)
	if err != nil {
		return stepArgs
	}
	return updated
}

// extractCorrelationIDFromToolResponse returns correlation_id from JSON tool responses.
func extractCorrelationIDFromToolResponse(resp JSONRPCResponse) string {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil || len(result.Content) == 0 {
		return ""
	}
	text := result.Content[0].Text
	jsonStart := strings.Index(text, "{")
	if jsonStart < 0 {
		return ""
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(text[jsonStart:]), &data); err != nil {
		return ""
	}
	correlationID, _ := data["correlation_id"].(string)
	return correlationID
}

// extractErrorMessage extracts the error message text from an error response.
func extractErrorMessage(resp JSONRPCResponse) string {
	var result MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "unknown error"
	}
	if len(result.Content) > 0 {
		text := result.Content[0].Text
		// Try to extract message from structured error JSON.
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
