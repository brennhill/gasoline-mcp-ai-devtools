// Purpose: Normalizes async command payloads before they are exposed to agents.
// Why: Keeps schema hardening and token-focused pruning isolated from transport flow.

package main

import (
	"bytes"
	"encoding/json"
	"strings"
)

func normalizeCompleteCommandResult(corrID string, result json.RawMessage) (json.RawMessage, string) {
	if strings.HasPrefix(corrID, "dom_list_") {
		return normalizeListInteractiveResult(result)
	}
	return result, ""
}

func normalizeListInteractiveResult(result json.RawMessage) (json.RawMessage, string) {
	trimmed := bytes.TrimSpace(result)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		payload := buildListInteractiveMissingPayload(nil, "list_interactive returned empty payload")
		return safeMarshal(payload, `{"success":false,"error":"list_interactive_missing_payload","elements":[]}`), "list_interactive_missing_payload"
	}

	var parsed any
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		return result, ""
	}

	m, ok := parsed.(map[string]any)
	if !ok {
		payload := buildListInteractiveMissingPayload(nil, "list_interactive returned non-object payload")
		return safeMarshal(payload, `{"success":false,"error":"list_interactive_missing_payload","elements":[]}`), "list_interactive_missing_payload"
	}

	if elems, exists := m["elements"]; exists {
		if elems == nil {
			m["elements"] = []any{}
			return safeMarshal(m, string(trimmed)), ""
		}
		if _, isArray := elems.([]any); isArray {
			return result, ""
		}
		payload := buildListInteractiveMissingPayload(m, "list_interactive returned invalid elements payload")
		return safeMarshal(payload, `{"success":false,"error":"list_interactive_missing_payload","elements":[]}`), "list_interactive_missing_payload"
	}

	if value, hasValue := m["value"]; hasValue && value == nil {
		payload := buildListInteractiveMissingPayload(m, "list_interactive returned value:null without elements")
		return safeMarshal(payload, `{"success":false,"error":"list_interactive_missing_payload","elements":[]}`), "list_interactive_missing_payload"
	}

	payload := buildListInteractiveMissingPayload(m, "list_interactive response missing elements")
	return safeMarshal(payload, `{"success":false,"error":"list_interactive_missing_payload","elements":[]}`), "list_interactive_missing_payload"
}

func buildListInteractiveMissingPayload(existing map[string]any, message string) map[string]any {
	payload := map[string]any{
		"success":  false,
		"error":    "list_interactive_missing_payload",
		"message":  message,
		"elements": []any{},
	}
	for _, key := range []string{
		"resolved_tab_id",
		"resolved_url",
		"target_context",
		"effective_tab_id",
		"effective_url",
		"effective_title",
	} {
		if existing != nil {
			if v, ok := existing[key]; ok {
				payload[key] = v
			}
		}
	}
	return payload
}

// stripEnrichedFieldsFromResult removes fields from the inner "result" that
// enrichCommandResponseData() already surfaced to the top level. This eliminates
// token-wasting duplication — the agent sees each field exactly once.
func stripEnrichedFieldsFromResult(responseData map[string]any) {
	resultRaw, ok := responseData["result"].(json.RawMessage)
	if !ok || len(resultRaw) == 0 {
		return
	}
	var resultMap map[string]any
	if err := json.Unmarshal(resultRaw, &resultMap); err != nil {
		return
	}

	// Strip keys shared with enrichCommandResponseData, plus result-only keys
	// (URL/tab fields and "result") that are surfaced separately.
	for _, key := range enrichedFieldKeys {
		delete(resultMap, key)
	}
	for _, key := range []string{
		"result",
		"effective_url", "effective_tab_id", "effective_title",
		"resolved_tab_id", "resolved_url", "final_url", "title",
	} {
		delete(resultMap, key)
	}

	if cleaned, err := json.Marshal(resultMap); err == nil {
		responseData["result"] = json.RawMessage(cleaned)
	}
}

// stripSuccessOnlyFields removes internal routing fields that are not useful to
// agents on successful responses. Error responses keep these for debugging.
func stripSuccessOnlyFields(responseData map[string]any) {
	delete(responseData, "target_context")
	delete(responseData, "content_script_status")
	delete(responseData, "created_at")
	delete(responseData, "trace_id")
}

// stripSummaryModeFields removes verbose fields from interact responses when
// summary mode is active. Keeps essential fields (status, timing, matched, errors)
// while stripping detailed diagnostics and large payloads (#447).
func stripSummaryModeFields(responseData map[string]any) {
	for _, key := range []string{
		"dom_summary",        // text description of mutations — dom_changes counts suffice
		"dom_mutations",      // individual mutation entries
		"perf_diff",          // performance before/after delta
		"evidence",           // before/after screenshot bundle (most verbose)
		"transient_elements", // captured UI toasts/alerts
		"trace",              // execution trace timeline
		"analysis",           // long-form analysis text
		"viewport",           // scroll/size metadata
	} {
		delete(responseData, key)
	}
}

// stripRetryContextOnSuccess removes retry_context when the command succeeded
// on the first attempt (no retry occurred), saving ~50 tokens per response.
func stripRetryContextOnSuccess(responseData map[string]any) {
	rc, ok := responseData["retry_context"].(map[string]any)
	if !ok {
		return
	}
	var attempt int
	switch v := rc["attempt"].(type) {
	case int:
		attempt = v
	case float64:
		attempt = int(v)
	}
	if attempt <= 1 {
		delete(responseData, "retry_context")
	}
}
