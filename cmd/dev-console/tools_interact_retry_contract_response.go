// Purpose: Builds retry_context and terminal guidance into command results.
// Why: Enforces one-retry policy with explicit stop conditions and evidence requirements.
// Docs: docs/features/feature/interact-explore/index.md

package main

import (
	"fmt"
	"strings"
)

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

	state, ok := h.getRetryState(correlationID)
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
