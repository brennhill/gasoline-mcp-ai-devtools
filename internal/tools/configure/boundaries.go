// boundaries.go — Pure functions for test boundary start/end response building.
package configure

import (
	"encoding/json"

	"github.com/dev-console/dev-console/internal/mcp"
)

// TestBoundaryStartResult holds the validated parameters for a test_boundary_start request.
type TestBoundaryStartResult struct {
	TestID string
	Label  string
}

// ParseTestBoundaryStart validates test_boundary_start arguments and returns
// the result or an MCP error response. If the error response is non-nil,
// the caller should return it directly.
func ParseTestBoundaryStart(reqID any, args json.RawMessage) (*TestBoundaryStartResult, *mcp.JSONRPCResponse) {
	var params struct {
		TestID string `json:"test_id"`
		Label  string `json:"label"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
			return nil, &resp
		}
	}

	if params.TestID == "" {
		resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: mcp.StructuredErrorResponse(mcp.ErrMissingParam, "Required parameter 'test_id' is missing", "Add the 'test_id' parameter", mcp.WithParam("test_id"))}
		return nil, &resp
	}

	label := params.Label
	if label == "" {
		label = "Test: " + params.TestID
	}

	return &TestBoundaryStartResult{TestID: params.TestID, Label: label}, nil
}

// BuildTestBoundaryStartResponse builds the MCP response for a validated test_boundary_start.
func BuildTestBoundaryStartResponse(reqID any, r *TestBoundaryStartResult) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: mcp.JSONResponse("Test boundary started", map[string]any{
		"status":  "ok",
		"test_id": r.TestID,
		"label":   r.Label,
		"message": "Test boundary started",
	})}
}

// TestBoundaryEndResult holds the validated parameters for a test_boundary_end request.
type TestBoundaryEndResult struct {
	TestID string
}

// ParseTestBoundaryEnd validates test_boundary_end arguments and returns
// the result or an MCP error response.
func ParseTestBoundaryEnd(reqID any, args json.RawMessage) (*TestBoundaryEndResult, *mcp.JSONRPCResponse) {
	var params struct {
		TestID string `json:"test_id"`
	}
	if len(args) > 0 {
		if err := json.Unmarshal(args, &params); err != nil {
			resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
			return nil, &resp
		}
	}

	if params.TestID == "" {
		resp := mcp.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: mcp.StructuredErrorResponse(mcp.ErrMissingParam, "Required parameter 'test_id' is missing", "Add the 'test_id' parameter", mcp.WithParam("test_id"))}
		return nil, &resp
	}

	return &TestBoundaryEndResult{TestID: params.TestID}, nil
}

// BuildTestBoundaryEndResponse builds the MCP response for a validated test_boundary_end.
func BuildTestBoundaryEndResponse(reqID any, r *TestBoundaryEndResult) mcp.JSONRPCResponse {
	return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: reqID, Result: mcp.JSONResponse("Test boundary ended", map[string]any{
		"status":     "ok",
		"test_id":    r.TestID,
		"was_active": true,
		"message":    "Test boundary ended",
	})}
}
