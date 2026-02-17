// tools_interact_upload.go — MCP interact upload action handler.
// Implements the "upload" action for the interact tool with 4-stage escalation.
// Stage 4 (OS automation) requires --enable-os-upload-automation flag.
//
// JSON CONVENTION: All fields MUST use snake_case. See .claude/refs/api-naming-standards.md
// Deviations from snake_case MUST be tagged with // SPEC:<spec-name> at the field level.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// uploadParams holds the parsed and validated upload parameters.
type uploadParams struct {
	Selector            string `json:"selector"`
	APIEndpoint         string `json:"api_endpoint,omitempty"`
	FilePath            string `json:"file_path"`
	Submit              bool   `json:"submit,omitempty"`
	EscalationTimeoutMs int    `json:"escalation_timeout_ms,omitempty"`
}

// handleUpload dispatches the "upload" interact action.
// Validates parameters and queues the upload operation.
func (h *ToolHandler) handleUpload(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	var params uploadParams
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidJSON, "Invalid JSON arguments: "+err.Error(), "Fix JSON syntax and call again")}
	}

	if errResp := validateUploadParams(req, params); errResp != nil {
		return *errResp
	}

	info, errResp := validateUploadFile(req, params.FilePath)
	if errResp != nil {
		return *errResp
	}

	return h.queueUpload(req, params, info)
}

// validateUploadParams checks required parameters for the upload action.
func validateUploadParams(req JSONRPCRequest, params uploadParams) *JSONRPCResponse {
	if params.FilePath == "" {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'file_path' is missing", "Add the 'file_path' parameter with an absolute path to the file", withParam("file_path"))}
		return &resp
	}
	if params.Selector == "" && params.APIEndpoint == "" {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrMissingParam, "Required parameter 'selector' is missing. Provide a CSS selector for the file input element, or use 'apiEndpoint' for direct API uploads.", "Add the 'selector' parameter (e.g., '#Filedata') or 'apiEndpoint'", withParam("selector"))}
		return &resp
	}
	if !filepath.IsAbs(params.FilePath) {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrPathNotAllowed, "file_path must be an absolute path. Relative paths are not allowed for security.", "Use an absolute path like '/Users/user/Videos/video.mp4'", withParam("file_path"))}
		return &resp
	}
	return nil
}

// validateUploadFile checks that the file exists, is readable, and is not a directory.
func validateUploadFile(req JSONRPCRequest, filePath string) (os.FileInfo, *JSONRPCResponse) {
	info, err := os.Stat(filePath)
	if err != nil {
		resp := uploadFileStatError(req, filePath, err)
		return nil, &resp
	}
	if info.IsDir() {
		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "Path is a directory, not a file: "+filePath, "Provide a path to a file, not a directory", withParam("file_path"))}
		return nil, &resp
	}
	return info, nil
}

func uploadFileStatError(req JSONRPCRequest, filePath string, err error) JSONRPCResponse {
	if os.IsNotExist(err) {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInvalidParam, "File not found: "+filePath+". Verify the file path is correct.", "Check the file path and try again", withParam("file_path"))}
	}
	if os.IsPermission(err) {
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrPathNotAllowed, "Permission denied reading file: "+filePath+". Check file permissions.", "Fix file permissions with: chmod +r "+filePath, withParam("file_path"))}
	}
	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpStructuredError(ErrInternal, "Failed to access file: "+err.Error(), "Check the file path and permissions")}
}

// queueUpload builds the upload payload and queues it for the extension.
func (h *ToolHandler) queueUpload(req JSONRPCRequest, params uploadParams, info os.FileInfo) JSONRPCResponse {
	if params.EscalationTimeoutMs <= 0 {
		params.EscalationTimeoutMs = defaultEscalationTimeoutMs
	}

	fileName := filepath.Base(params.FilePath)
	mimeType := detectMimeType(fileName)
	fileSize := info.Size()
	progressTier := getProgressTier(fileSize)
	correlationID := fmt.Sprintf("upload_%d_%d", time.Now().UnixNano(), randomInt63())

	uploadPayload := map[string]any{
		"action": "upload", "selector": params.Selector,
		"file_path": params.FilePath, "file_name": fileName,
		"file_size": fileSize, "mime_type": mimeType,
		"submit": params.Submit, "escalation_timeout_ms": params.EscalationTimeoutMs,
		"progress_tier": string(progressTier),
	}
	if params.APIEndpoint != "" {
		uploadPayload["api_endpoint"] = params.APIEndpoint
	}

	// Error impossible: map contains only primitive types from input
	payloadJSON, _ := json.Marshal(uploadPayload)
	query := queries.PendingQuery{Type: "upload", Params: payloadJSON, CorrelationID: correlationID}
	h.capture.CreatePendingQueryWithTimeout(query, 10*time.Minute, req.ClientID)

	h.recordAIAction("upload", "", map[string]any{
		"file_path": params.FilePath, "file_name": fileName,
		"file_size": fileSize, "selector": params.Selector,
		"progress_tier": string(progressTier),
	})

	return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcpJSONResponse("Upload queued", map[string]any{
		"status": "queued", "correlation_id": correlationID,
		"final": false,
		"file_name": fileName, "file_size": fileSize,
		"mime_type": mimeType, "progress_tier": string(progressTier),
		"message": "Upload queued for execution. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})}
}
