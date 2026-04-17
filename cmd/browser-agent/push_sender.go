// push_sender.go — Stdio-based sampling sender and notifier for push delivery.
package main

import (
	"encoding/json"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/push"
)

// stdioSamplingSender sends sampling/createMessage requests via MCP stdio.
type stdioSamplingSender struct{}

func (s *stdioSamplingSender) SendSampling(req push.SamplingRequest) error {
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	writeMCPPayload(payload, getBridgeFraming())
	return nil
}

// stdioNotifier sends MCP notifications via stdio.
type stdioNotifier struct{}

func (n *stdioNotifier) SendNotification(method string, params map[string]any) {
	notif := map[string]any{
		"jsonrpc": JSONRPCVersion,
		"method":  method,
		"params":  params,
	}
	payload, err := json.Marshal(notif)
	if err != nil {
		return
	}
	writeMCPPayload(payload, getBridgeFraming())
}
