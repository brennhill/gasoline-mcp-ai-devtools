// workflow.go — Workflow helper types and pure functions for the interact tool.
// Provides step tracing, error detection, and result assembly for compound actions.
package interact

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/dev-console/dev-console/internal/mcp"
)

// WorkflowStep records a single step's outcome within a workflow trace.
type WorkflowStep struct {
	Action        string `json:"action"`
	CorrelationID string `json:"correlation_id,omitempty"`
	Status        string `json:"status"` // "success", "error", "skipped"
	TimingMs      int64  `json:"timing_ms"`
	Detail        string `json:"detail,omitempty"`
}

// FormField represents a single field to fill in a form workflow.
type FormField struct {
	Selector string `json:"selector"`
	Value    string `json:"value"`
	Index    *int   `json:"index,omitempty"`
}

// IsErrorResponse checks if a JSONRPCResponse represents an error.
func IsErrorResponse(resp mcp.JSONRPCResponse) bool {
	if resp.Error != nil {
		return true
	}
	var result mcp.MCPToolResult
	if err := json.Unmarshal(resp.Result, &result); err == nil {
		return result.IsError
	}
	return false
}

// ResponseStatus returns "success" or "error" based on the response.
func ResponseStatus(resp mcp.JSONRPCResponse) string {
	if IsErrorResponse(resp) {
		return "error"
	}
	return "success"
}

// WorkflowResult wraps the final step's response with workflow metadata (trace + timing).
// On failure, the response uses isError=true so the MCP envelope correctly signals an error.
func WorkflowResult(req mcp.JSONRPCRequest, workflow string, trace []WorkflowStep, lastResp mcp.JSONRPCResponse, start time.Time) mcp.JSONRPCResponse {
	totalMs := time.Since(start).Milliseconds()

	// Count steps by status
	successCount := 0
	for _, s := range trace {
		if s.Status == "success" {
			successCount++
		}
	}
	allSuccess := successCount == len(trace)
	failed := IsErrorResponse(lastResp)

	status := "success"
	if failed {
		status = "failed"
	} else if !allSuccess {
		status = "partial_failure"
	}

	var summary string
	if failed {
		summary = fmt.Sprintf("%s failed at step %d/%d (%dms)", workflow, len(trace), len(trace), totalMs)
	} else {
		summary = fmt.Sprintf("%s completed (%d/%d steps succeeded, %dms)", workflow, successCount, len(trace), totalMs)
	}

	data := map[string]any{
		"workflow":   workflow,
		"status":     status,
		"trace":      trace,
		"total_ms":   totalMs,
		"steps":      len(trace),
		"successful": successCount,
	}

	// Extract the failing step's error detail for context
	if failed {
		var lastResult mcp.MCPToolResult
		if json.Unmarshal(lastResp.Result, &lastResult) == nil && len(lastResult.Content) > 0 {
			data["error_detail"] = lastResult.Content[0].Text
		} else if lastResp.Error != nil {
			data["error_detail"] = lastResp.Error.Message
		}
	}

	dataJSON, _ := json.Marshal(data)
	resultText := summary + "\n" + string(dataJSON)

	result := mcp.MCPToolResult{
		Content: []mcp.MCPContentBlock{{Type: "text", Text: resultText}},
		IsError: failed,
	}
	resultJSON, _ := json.Marshal(result)
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(resultJSON)}
}
