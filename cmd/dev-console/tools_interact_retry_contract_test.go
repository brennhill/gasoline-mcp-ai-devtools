// tools_interact_retry_contract_test.go — TDD tests for deterministic retry contract.
package main

import (
	"encoding/json"
	"testing"
)

func TestRetryContract_FirstFailureIncludesRetryContext(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	result, ok := env.callInteract(t, `{"what":"click","selector":"#retry-btn","background":true}`)
	if !ok || result.IsError {
		t.Fatalf("click should queue successfully, got: %s", firstText(result))
	}
	queued := extractResultJSON(t, result)
	corrID, _ := queued["correlation_id"].(string)
	if corrID == "" {
		t.Fatalf("missing correlation_id in queued response: %v", queued)
	}

	env.capture.ApplyCommandResult(corrID, "error", json.RawMessage(`{"success":false,"error":"element_not_found"}`), "element_not_found")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 2}
	args := json.RawMessage(`{"correlation_id":"` + corrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)
	observe := parseToolResult(t, resp)
	if !observe.IsError {
		t.Fatalf("expected failure result, got: %s", firstText(observe))
	}

	data := extractResultJSON(t, observe)
	attempt, ok := retryContextAttempt(data)
	if !ok || attempt != 1 {
		t.Fatalf("retry_context.attempt = %v (ok=%v), want 1. retry_context=%s", attempt, ok, retryContextString(data))
	}
	if reason := retryContextReason(data); reason != "element_not_found" {
		t.Fatalf("retry_context.reason = %q, want element_not_found", reason)
	}
	if terminal, ok := retryContextTerminal(data); !ok || terminal {
		t.Fatalf("retry_context.terminal_stop = %v (ok=%v), want false", terminal, ok)
	}
	if retryable, ok := data["retryable"].(bool); !ok || !retryable {
		t.Fatalf("retryable = %v (ok=%v), want true on first failure", data["retryable"], ok)
	}
}

func TestRetryContract_SecondFailureWithoutStrategyChangeIsTerminal(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	first, ok := env.callInteract(t, `{"what":"click","selector":"#retry-btn","background":true}`)
	if !ok || first.IsError {
		t.Fatalf("first click should queue successfully, got: %s", firstText(first))
	}
	firstData := extractResultJSON(t, first)
	firstCorrID, _ := firstData["correlation_id"].(string)
	env.capture.ApplyCommandResult(firstCorrID, "error", json.RawMessage(`{"success":false,"error":"ambiguous_target"}`), "ambiguous_target")

	secondArgs := `{"what":"click","selector":"#retry-btn","background":true,"correlation_id":"` + firstCorrID + `"}`
	second, ok := env.callInteract(t, secondArgs)
	if !ok || second.IsError {
		t.Fatalf("second click should queue successfully, got: %s", firstText(second))
	}
	secondData := extractResultJSON(t, second)
	secondCorrID, _ := secondData["correlation_id"].(string)
	env.capture.ApplyCommandResult(secondCorrID, "error", json.RawMessage(`{"success":false,"error":"ambiguous_target"}`), "ambiguous_target")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 3}
	args := json.RawMessage(`{"correlation_id":"` + secondCorrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)
	observe := parseToolResult(t, resp)
	if !observe.IsError {
		t.Fatalf("expected terminal failure, got success: %s", firstText(observe))
	}

	data := extractResultJSON(t, observe)
	attempt, ok := retryContextAttempt(data)
	if !ok || attempt != 2 {
		t.Fatalf("retry_context.attempt = %v (ok=%v), want 2. retry_context=%s", attempt, ok, retryContextString(data))
	}
	changed, ok := retryContextChangedStrategy(data)
	if !ok || changed {
		t.Fatalf("retry_context.changed_strategy = %v (ok=%v), want false", changed, ok)
	}
	if terminal, ok := retryContextTerminal(data); !ok || !terminal {
		t.Fatalf("retry_context.terminal_stop = %v (ok=%v), want true", terminal, ok)
	}
	if terminal, _ := data["terminal"].(bool); !terminal {
		t.Fatalf("terminal = %v, want true", data["terminal"])
	}
	if retryable, _ := data["retryable"].(bool); retryable {
		t.Fatalf("retryable = %v, want false on terminal failure", data["retryable"])
	}

	summary, ok := data["evidence_summary"].(map[string]any)
	if !ok {
		t.Fatalf("evidence_summary missing on terminal failure: %v", data["evidence_summary"])
	}
	if next, _ := summary["next_action"].(string); next == "" {
		t.Fatalf("evidence_summary.next_action missing: %v", summary)
	}
}

func TestRetryContract_SecondFailureWithChangedStrategyStillTerminal(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	first, ok := env.callInteract(t, `{"what":"click","selector":".submit-btn","background":true}`)
	if !ok || first.IsError {
		t.Fatalf("first click should queue successfully, got: %s", firstText(first))
	}
	firstData := extractResultJSON(t, first)
	firstCorrID, _ := firstData["correlation_id"].(string)
	env.capture.ApplyCommandResult(firstCorrID, "error", json.RawMessage(`{"success":false,"error":"ambiguous_target"}`), "ambiguous_target")

	secondArgs := `{"what":"click","selector":".submit-btn","scope_selector":"#active-composer","background":true,"correlation_id":"` + firstCorrID + `"}`
	second, ok := env.callInteract(t, secondArgs)
	if !ok || second.IsError {
		t.Fatalf("second click should queue successfully, got: %s", firstText(second))
	}
	secondData := extractResultJSON(t, second)
	secondCorrID, _ := secondData["correlation_id"].(string)
	env.capture.ApplyCommandResult(secondCorrID, "error", json.RawMessage(`{"success":false,"error":"ambiguous_target"}`), "ambiguous_target")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 4}
	args := json.RawMessage(`{"correlation_id":"` + secondCorrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)
	observe := parseToolResult(t, resp)
	if !observe.IsError {
		t.Fatalf("expected terminal failure on second attempt, got: %s", firstText(observe))
	}

	data := extractResultJSON(t, observe)
	attempt, ok := retryContextAttempt(data)
	if !ok || attempt != 2 {
		t.Fatalf("retry_context.attempt = %v (ok=%v), want 2. retry_context=%s", attempt, ok, retryContextString(data))
	}
	changed, ok := retryContextChangedStrategy(data)
	if !ok || !changed {
		t.Fatalf("retry_context.changed_strategy = %v (ok=%v), want true", changed, ok)
	}
	if terminal, ok := retryContextTerminal(data); !ok || !terminal {
		t.Fatalf("retry_context.terminal_stop = %v (ok=%v), want true", terminal, ok)
	}
}

func TestRetryContract_SecondAttemptSuccessIncludesRetryContext(t *testing.T) {
	env := newInteractTestEnv(t)
	env.capture.SetPilotEnabled(true)

	first, ok := env.callInteract(t, `{"what":"click","selector":"#retry-btn","background":true}`)
	if !ok || first.IsError {
		t.Fatalf("first click should queue successfully, got: %s", firstText(first))
	}
	firstData := extractResultJSON(t, first)
	firstCorrID, _ := firstData["correlation_id"].(string)
	env.capture.ApplyCommandResult(firstCorrID, "error", json.RawMessage(`{"success":false,"error":"element_not_found"}`), "element_not_found")

	secondArgs := `{"what":"click","selector":"#retry-btn","scope_selector":"#dialog","background":true,"correlation_id":"` + firstCorrID + `"}`
	second, ok := env.callInteract(t, secondArgs)
	if !ok || second.IsError {
		t.Fatalf("second click should queue successfully, got: %s", firstText(second))
	}
	secondData := extractResultJSON(t, second)
	secondCorrID, _ := secondData["correlation_id"].(string)
	env.capture.CompleteCommand(secondCorrID, json.RawMessage(`{"success":true}`), "")

	req := JSONRPCRequest{JSONRPC: "2.0", ID: 5}
	args := json.RawMessage(`{"correlation_id":"` + secondCorrID + `"}`)
	resp := env.handler.toolObserveCommandResult(req, args)
	observe := parseToolResult(t, resp)
	if observe.IsError {
		t.Fatalf("expected success on second attempt, got: %s", firstText(observe))
	}

	data := extractResultJSON(t, observe)
	attempt, ok := retryContextAttempt(data)
	if !ok || attempt != 2 {
		t.Fatalf("retry_context.attempt = %v (ok=%v), want 2. retry_context=%s", attempt, ok, retryContextString(data))
	}
	changed, ok := retryContextChangedStrategy(data)
	if !ok || !changed {
		t.Fatalf("retry_context.changed_strategy = %v (ok=%v), want true", changed, ok)
	}
	if terminal, ok := retryContextTerminal(data); !ok || terminal {
		t.Fatalf("retry_context.terminal_stop = %v (ok=%v), want false on success", terminal, ok)
	}
	if reason := retryContextReason(data); reason != "success" {
		t.Fatalf("retry_context.reason = %q, want success", reason)
	}
}
