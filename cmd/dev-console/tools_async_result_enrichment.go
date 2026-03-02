// Purpose: Enriches async command payloads with agent-facing context and recovery hints.
// Why: Centralizes response metadata promotion and failure diagnosis in one module.

package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

// enrichedFieldKeys lists the fields that enrichCommandResponseData() surfaces
// from the inner extension result to the top-level response. Both the enrichment
// loop and stripEnrichedFieldsFromResult() reference this slice so the two sides
// cannot diverge.
var enrichedFieldKeys = []string{
	"timing", "dom_changes", "dom_summary", "dom_mutations", "analysis",
	"content_script_status", "target_context",
	"message", "hint", "retry", "retryable", "csp_blocked", "failure_cause", "error_code",
	"candidates", "match_count", "match_strategy",
	"viewport",
}

func enrichCommandResponseData(result json.RawMessage, responseData map[string]any, corrID ...string) (embeddedErr string, hasEmbeddedErr bool) {
	if len(result) == 0 {
		return "", false
	}

	var extResult map[string]any
	if err := json.Unmarshal(result, &extResult); err != nil {
		return "", false
	}

	// Surface extension enrichment fields to top-level for easier LLM consumption.
	// Non-tab fields are always surfaced.
	for _, key := range enrichedFieldKeys {
		if v, ok := extResult[key]; ok {
			responseData[key] = v
		}
	}

	// Deduplicate tab context: only include resolved_*/final_url/title when URLs changed.
	resolvedURL, _ := extResult["resolved_url"].(string)
	effectiveURL, _ := extResult["effective_url"].(string)
	effectiveTitle, _ := extResult["effective_title"].(string)

	// Always surface effective_* when present.
	if effectiveURL != "" {
		responseData["effective_url"] = effectiveURL
	}
	if v, ok := extResult["effective_tab_id"]; ok {
		responseData["effective_tab_id"] = v
	}
	if effectiveTitle != "" {
		responseData["effective_title"] = effectiveTitle
	}

	// Only surface resolved_*/final_url/title when URL changed (navigation/redirect).
	if resolvedURL != "" && effectiveURL != "" && resolvedURL != effectiveURL {
		responseData["resolved_tab_id"] = extResult["resolved_tab_id"]
		responseData["resolved_url"] = resolvedURL
		if v, ok := extResult["final_url"]; ok {
			responseData["final_url"] = v
		}
		if v, ok := extResult["title"]; ok {
			responseData["title"] = v
		}
		responseData["tab_changed"] = true
		responseData["navigation_detected"] = true
		responseData["navigation_note"] = fmt.Sprintf("Page navigated from %s to %s", resolvedURL, effectiveURL)
	}

	// Surface execute_js return values prominently so agents don't have to
	// dig into result.result. The field is named "return_value" to distinguish
	// the script's return value from the overall command result envelope.
	// Only applies to execute_js commands (corrID prefix "exec_") to avoid
	// leaking internal result fields from other action types.
	cid := ""
	if len(corrID) > 0 {
		cid = corrID[0]
	}
	if v, ok := extResult["result"]; ok && strings.HasPrefix(cid, "exec_") {
		responseData["return_value"] = v
	}

	if success, ok := extResult["success"].(bool); ok && !success {
		msg := embeddedCommandError(extResult)
		if msg == "" {
			msg = "Command reported success=false"
		}
		return msg, true
	}

	if _, ok := extResult["error"]; ok {
		msg := embeddedCommandError(extResult)
		if msg != "" {
			return msg, true
		}
	}

	return "", false
}

func embeddedCommandError(extResult map[string]any) string {
	if msg, ok := extResult["error"].(string); ok && msg != "" {
		return msg
	}
	if msg, ok := extResult["message"].(string); ok && msg != "" {
		return msg
	}
	return ""
}

func annotateCSPFailure(responseData map[string]any, cmdError string, result json.RawMessage) {
	cspBlocked, errorCode, message := detectCSPFailure(cmdError, result)
	if !cspBlocked {
		return
	}

	responseData["csp_blocked"] = true
	responseData["failure_cause"] = "csp"

	if errorCode != "" {
		responseData["error_code"] = errorCode
	}
	if message != "" {
		if _, exists := responseData["message"]; !exists {
			responseData["message"] = message
		}
	}
	if _, exists := responseData["retry"]; !exists {
		responseData["retry"] = cspRetryNavigationGuidance
	}
}

func annotateInteractFailureRecovery(responseData map[string]any, cmdError string, result json.RawMessage) {
	code := detectInteractFailureCode(responseData, cmdError, result)
	canonical, playbook, ok := lookupInteractFailurePlaybook(code)
	if !ok {
		return
	}
	if _, exists := responseData["error_code"]; !exists {
		responseData["error_code"] = canonical
	}
	if _, exists := responseData["retry"]; !exists {
		responseData["retry"] = playbook.RetrySuggestion
	}
	if _, exists := responseData["hint"]; !exists {
		responseData["hint"] = "Recovery playbook available for " + canonical + " in configure tutorial."
	}
	if _, exists := responseData["retryable"]; !exists {
		responseData["retryable"] = true
	}

	// For ambiguous_target: compute suggested_element_id from first visible candidate.
	if canonical == "ambiguous_target" {
		annotateSuggestedElementID(responseData)
	}
}

// annotateSuggestedElementID picks the first visible candidate's element_id
// and sets it as suggested_element_id for single-retry LLM recovery.
func annotateSuggestedElementID(responseData map[string]any) {
	// Skip if extension already set suggested_element_id (e.g. ranked resolution).
	if existing, ok := responseData["suggested_element_id"].(string); ok && existing != "" {
		return
	}
	candidates, ok := responseData["candidates"].([]any)
	if !ok || len(candidates) == 0 {
		return
	}
	for _, c := range candidates {
		cMap, ok := c.(map[string]any)
		if !ok {
			continue
		}
		visible, _ := cMap["visible"].(bool)
		elementID, _ := cMap["element_id"].(string)
		if visible && elementID != "" {
			responseData["suggested_element_id"] = elementID
			return
		}
	}
}

func detectInteractFailureCode(responseData map[string]any, cmdError string, result json.RawMessage) string {
	candidates := make([]string, 0, 4)
	if v, ok := responseData["error_code"].(string); ok && strings.TrimSpace(v) != "" {
		candidates = append(candidates, v)
	}
	if v, ok := responseData["error"].(string); ok && strings.TrimSpace(v) != "" {
		candidates = append(candidates, v)
	}

	if len(result) > 0 {
		var extResult map[string]any
		if err := json.Unmarshal(result, &extResult); err == nil {
			if v, ok := extResult["error"].(string); ok && strings.TrimSpace(v) != "" {
				candidates = append(candidates, v)
			}
			if v, ok := extResult["error_code"].(string); ok && strings.TrimSpace(v) != "" {
				candidates = append(candidates, v)
			}
		}
	}

	if strings.TrimSpace(cmdError) != "" {
		candidates = append(candidates, cmdError)
	}

	for _, candidate := range candidates {
		if normalized := normalizeInteractFailureCode(candidate); normalized != "" {
			return normalized
		}
	}
	return ""
}

func detectCSPFailure(cmdError string, result json.RawMessage) (bool, string, string) {
	errorCode := ""
	message := ""

	if len(result) > 0 {
		var extResult map[string]any
		if err := json.Unmarshal(result, &extResult); err == nil {
			if v, ok := extResult["error"].(string); ok {
				errorCode = strings.TrimSpace(v)
			}
			if v, ok := extResult["message"].(string); ok {
				message = strings.TrimSpace(v)
			}
			if v, ok := extResult["csp_blocked"].(bool); ok && v {
				return true, errorCode, message
			}
			if v, ok := extResult["failure_cause"].(string); ok && strings.EqualFold(strings.TrimSpace(v), "csp") {
				return true, errorCode, message
			}
			for _, key := range []string{"error", "message", "hint"} {
				if v, ok := extResult[key].(string); ok && looksLikeCSP(v) {
					return true, errorCode, message
				}
			}
		}
	}

	if looksLikeCSP(cmdError) {
		if errorCode == "" {
			errorCode = strings.TrimSpace(cmdError)
		}
		return true, errorCode, message
	}

	return false, errorCode, message
}

func looksLikeCSP(value string) bool {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return false
	}
	return strings.Contains(v, "csp") ||
		strings.Contains(v, "content security policy") ||
		strings.Contains(v, "trusted type") ||
		strings.Contains(v, "unsafe-eval") ||
		strings.Contains(v, "blocks content scripts") ||
		strings.Contains(v, "blocked content scripts") ||
		strings.Contains(v, "restricted_page")
}
