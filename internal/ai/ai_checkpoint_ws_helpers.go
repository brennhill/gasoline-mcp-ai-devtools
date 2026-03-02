package ai

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/capture"

func classifyWSEvent(diff *WebSocketDiff, evt *capture.WebSocketEvent, severity string) {
	switch evt.Event {
	case "close":
		if severity != "errors_only" {
			diff.Disconnections = append(diff.Disconnections, WSDisconnection{
				URL:         evt.URL,
				CloseCode:   evt.CloseCode,
				CloseReason: evt.CloseReason,
			})
		}
	case "open":
		diff.Connections = append(diff.Connections, WSConn{URL: evt.URL, ID: evt.ID})
	case "error":
		diff.Errors = append(diff.Errors, WSError{URL: evt.URL, Message: evt.Data})
	}
}

func capWSDiff(diff *WebSocketDiff) {
	if len(diff.Disconnections) > maxDiffEntriesPerCat {
		diff.Disconnections = diff.Disconnections[:maxDiffEntriesPerCat]
	}
	if len(diff.Connections) > maxDiffEntriesPerCat {
		diff.Connections = diff.Connections[:maxDiffEntriesPerCat]
	}
	if len(diff.Errors) > maxDiffEntriesPerCat {
		diff.Errors = diff.Errors[:maxDiffEntriesPerCat]
	}
}
