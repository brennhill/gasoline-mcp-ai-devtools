// tools_interact_retry_contract.go — Deterministic retry contract for interact actions.
//
// Contract goals:
// - Max one retry per step (2 attempts total)
// - Retry attempt must use a changed strategy
// - Terminal failures include an actionable evidence summary
// - Retry metadata is exposed via retry_context
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const maxRetryAttemptsPerStep = 2

type commandRetryState struct {
	Attempt             int
	MaxAttempts         int
	Action              string
	Strategy            string
	StrategyFingerprint string
	ChangedStrategy     bool
	PolicyViolation     string
	ParentCorrelationID string
	CreatedAt           time.Time
}

type retryTerminalDecision struct {
	Terminal bool
	Cause    string
}

func parseRetryParentCorrelationID(args json.RawMessage) string {
	var params struct {
		CorrelationID string `json:"correlation_id"`
	}
	lenientUnmarshal(args, &params)
	return strings.TrimSpace(params.CorrelationID)
}

func deriveRetryStrategy(action string, args json.RawMessage) (strategy string, fingerprint string) {
	var payload map[string]any
	lenientUnmarshal(args, &payload)

	f := map[string]any{
		"action": strings.ToLower(strings.TrimSpace(action)),
	}
	for _, key := range []string{
		"selector",
		"scope_selector",
		"scope_rect",
		"annotation_rect",
		"element_id",
		"index",
		"frame",
		"world",
		"text",
		"value",
		"wait_for",
	} {
		if v, ok := payload[key]; ok {
			f[key] = v
		}
	}
	fingerprint = stableMarshalForRetry(f)

	switch {
	case payload["element_id"] != nil:
		return "element_handle", fingerprint
	case payload["scope_selector"] != nil || payload["scope_rect"] != nil || payload["annotation_rect"] != nil:
		return "scoped_selector", fingerprint
	case payload["frame"] != nil:
		return "frame_targeted", fingerprint
	case payload["selector"] != nil:
		return "selector", fingerprint
	case payload["index"] != nil:
		return "indexed", fingerprint
	case payload["world"] != nil:
		return "world_switch", fingerprint
	default:
		return "default", fingerprint
	}
}

func stableMarshalForRetry(v map[string]any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func (h *ToolHandler) armRetryContract(correlationID, action string, args json.RawMessage) {
	if h == nil || correlationID == "" {
		return
	}

	if action == "" {
		action = canonicalActionFromInteractArgs(args)
	}
	action = strings.ToLower(strings.TrimSpace(action))
	strategy, fingerprint := deriveRetryStrategy(action, args)
	parentCorrID := parseRetryParentCorrelationID(args)

	state := &commandRetryState{
		Attempt:             1,
		MaxAttempts:         maxRetryAttemptsPerStep,
		Action:              action,
		Strategy:            strategy,
		StrategyFingerprint: fingerprint,
		ChangedStrategy:     true,
		ParentCorrelationID: parentCorrID,
		CreatedAt:           time.Now(),
	}

	if parentCorrID != "" {
		if parent, ok := h.getRetryState(parentCorrID); ok {
			state.Attempt = parent.Attempt + 1
			if state.Attempt > state.MaxAttempts {
				state.Attempt = state.MaxAttempts
				state.PolicyViolation = "attempt_limit_exceeded"
			}
			state.ChangedStrategy = state.StrategyFingerprint != parent.StrategyFingerprint
			if !state.ChangedStrategy {
				state.PolicyViolation = "strategy_unchanged"
			}
		} else {
			// Treat explicit parent chaining as retry attempt even if parent context has expired.
			state.Attempt = 2
			state.PolicyViolation = "parent_context_missing"
		}
	}

	h.retryContractMu.Lock()
	if h.retryByCommand == nil {
		h.retryByCommand = make(map[string]*commandRetryState)
	}
	h.retryByCommand[correlationID] = state
	h.pruneRetryStatesLocked(2048)
	h.retryContractMu.Unlock()
}

func (h *ToolHandler) getRetryState(correlationID string) (*commandRetryState, bool) {
	h.retryContractMu.Lock()
	defer h.retryContractMu.Unlock()
	state, ok := h.retryByCommand[correlationID]
	return state, ok
}

func (h *ToolHandler) pruneRetryStatesLocked(maxEntries int) {
	if len(h.retryByCommand) <= maxEntries {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	for key, st := range h.retryByCommand {
		if oldestKey == "" || st.CreatedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = st.CreatedAt
		}
	}
	if oldestKey != "" {
		delete(h.retryByCommand, oldestKey)
	}
}

func deriveRetryReason(responseData map[string]any, fallback string) string {
	if responseData != nil {
		if code, ok := responseData["error_code"].(string); ok && strings.TrimSpace(code) != "" {
			return strings.TrimSpace(code)
		}
		if errCode, ok := responseData["error"].(string); ok && strings.TrimSpace(errCode) != "" {
			return strings.TrimSpace(errCode)
		}
	}
	if strings.TrimSpace(fallback) != "" {
		return strings.TrimSpace(fallback)
	}
	return "unknown"
}

func (h *ToolHandler) attachRetryContext(correlationID string, responseData map[string]any, status string, fallbackReason string) retryTerminalDecision {
	if h == nil || correlationID == "" || responseData == nil {
		return retryTerminalDecision{}
	}

	h.retryContractMu.Lock()
	state, ok := h.retryByCommand[correlationID]
	h.retryContractMu.Unlock()
	if !ok || state == nil {
		return retryTerminalDecision{}
	}

	reason := deriveRetryReason(responseData, fallbackReason)
	if strings.EqualFold(status, "complete") && reason == "unknown" {
		reason = "success"
	}

	retryContext := map[string]any{
		"attempt":          state.Attempt,
		"max_attempts":     state.MaxAttempts,
		"strategy":         state.Strategy,
		"changed_strategy": state.ChangedStrategy,
		"reason":           reason,
	}
	if state.ParentCorrelationID != "" {
		retryContext["parent_correlation_id"] = state.ParentCorrelationID
	}
	if state.PolicyViolation != "" {
		retryContext["policy_violation"] = state.PolicyViolation
	}

	decision := retryTerminalDecision{}
	failureStatus := status == "error" || status == "timeout" || status == "expired" || status == "cancelled"
	if failureStatus {
		if state.Attempt >= state.MaxAttempts {
			decision.Terminal = true
			decision.Cause = "max_attempts_reached"
		}
		if state.Attempt > 1 && !state.ChangedStrategy {
			decision.Terminal = true
			decision.Cause = "strategy_not_changed"
		}
	}

	retryContext["terminal_stop"] = decision.Terminal
	if decision.Cause != "" {
		retryContext["terminal_cause"] = decision.Cause
	}
	responseData["retry_context"] = retryContext

	if !failureStatus {
		return decision
	}

	if decision.Terminal {
		responseData["terminal"] = true
		responseData["retryable"] = false
		if _, exists := responseData["retry"]; !exists {
			responseData["retry"] = "Terminal after two attempts. Stop retrying this step and report evidence_summary."
		}
		responseData["evidence_summary"] = buildRetryEvidenceSummary(correlationID, reason, retryContext, responseData)
		return decision
	}

	// Non-terminal failure on attempt 1: allow one retry with a changed strategy.
	if _, exists := responseData["retryable"]; !exists {
		responseData["retryable"] = true
	}
	if _, exists := responseData["retry"]; !exists {
		responseData["retry"] = "Retry once with a changed strategy (scope_selector/scope_rect/element_id/index/frame/world). If the second attempt fails, stop and report evidence_summary."
	}

	return decision
}

func buildRetryEvidenceSummary(correlationID, reason string, retryContext map[string]any, responseData map[string]any) map[string]any {
	summary := map[string]any{
		"correlation_id": correlationID,
		"failure_reason": reason,
		"next_action":    "Stop retries for this step and report this bundle.",
		"required": []string{
			"command_result",
			"screenshot",
			"scoped_list_interactive_output",
		},
	}

	if retryContext != nil {
		summary["retry_context"] = retryContext
	}
	if responseData != nil {
		if evidence, ok := responseData["evidence"]; ok {
			summary["captured_evidence"] = evidence
		}
		if u, ok := responseData["effective_url"].(string); ok && strings.TrimSpace(u) != "" {
			summary["url"] = u
		} else if u, ok := responseData["resolved_url"].(string); ok && strings.TrimSpace(u) != "" {
			summary["url"] = u
		}
	}
	return summary
}

func retryContextAttempt(data map[string]any) (int, bool) {
	retryContext, ok := data["retry_context"].(map[string]any)
	if !ok {
		return 0, false
	}
	v, ok := retryContext["attempt"].(float64)
	if !ok {
		return 0, false
	}
	return int(v), true
}

func retryContextChangedStrategy(data map[string]any) (bool, bool) {
	retryContext, ok := data["retry_context"].(map[string]any)
	if !ok {
		return false, false
	}
	v, ok := retryContext["changed_strategy"].(bool)
	return v, ok
}

func retryContextTerminal(data map[string]any) (bool, bool) {
	retryContext, ok := data["retry_context"].(map[string]any)
	if !ok {
		return false, false
	}
	v, ok := retryContext["terminal_stop"].(bool)
	return v, ok
}

func retryContextReason(data map[string]any) string {
	retryContext, ok := data["retry_context"].(map[string]any)
	if !ok {
		return ""
	}
	v, _ := retryContext["reason"].(string)
	return strings.TrimSpace(v)
}

func retryContextString(data map[string]any) string {
	retryContext, ok := data["retry_context"].(map[string]any)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", retryContext)
}
