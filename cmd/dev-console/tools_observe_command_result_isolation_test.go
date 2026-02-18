package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestObserveCommandResult_ClientIsolation(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.RegisterCommandForClient("corr-owned", "q-owned", 30*time.Second, "client-a")
	h.capture.CompleteCommand("corr-owned", json.RawMessage(`{"ok":true}`), "")

	reqA := JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "client-a"}
	respA := h.toolObserveCommandResult(reqA, json.RawMessage(`{"correlation_id":"corr-owned"}`))
	var resultA MCPToolResult
	if err := json.Unmarshal(respA.Result, &resultA); err != nil {
		t.Fatalf("client-a unmarshal failed: %v", err)
	}
	if resultA.IsError {
		t.Fatalf("client-a should read owned command result, got error: %s", resultA.Content[0].Text)
	}

	reqB := JSONRPCRequest{JSONRPC: "2.0", ID: 2, ClientID: "client-b"}
	respB := h.toolObserveCommandResult(reqB, json.RawMessage(`{"correlation_id":"corr-owned"}`))
	var resultB MCPToolResult
	if err := json.Unmarshal(respB.Result, &resultB); err != nil {
		t.Fatalf("client-b unmarshal failed: %v", err)
	}
	if !resultB.IsError {
		t.Fatal("client-b should not read command owned by client-a")
	}
	if !strings.Contains(resultB.Content[0].Text, "Command not found") {
		t.Fatalf("unexpected client-b error text: %s", resultB.Content[0].Text)
	}
}

func TestObserveCommandResult_AnnotationClientIsolation(t *testing.T) {
	t.Parallel()
	h := newTestToolHandler()

	h.capture.RegisterCommandForClient("ann_owned", "q-ann-owned", 30*time.Second, "client-a")
	h.capture.CompleteCommand("ann_owned", json.RawMessage(`{"ok":true}`), "")

	reqB := JSONRPCRequest{JSONRPC: "2.0", ID: 1, ClientID: "client-b"}
	respB := h.toolObserveCommandResult(reqB, json.RawMessage(`{"correlation_id":"ann_owned"}`))
	var resultB MCPToolResult
	if err := json.Unmarshal(respB.Result, &resultB); err != nil {
		t.Fatalf("client-b unmarshal failed: %v", err)
	}
	if !resultB.IsError {
		t.Fatal("client-b should not read annotation command owned by client-a")
	}
	if !strings.Contains(resultB.Content[0].Text, "Annotation command not found") {
		t.Fatalf("unexpected annotation error text: %s", resultB.Content[0].Text)
	}
}
