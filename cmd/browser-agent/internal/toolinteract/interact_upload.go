// Purpose: Implements the interact upload action with 4-stage escalation (extension inject, form submit, direct API, OS automation).
// Why: Provides reliable file upload across diverse page architectures by cascading through progressively more aggressive strategies.
// Docs: docs/features/feature/file-upload/index.md
package toolinteract

import (
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
	uploadhandler "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"
)

var (
	defaultEscalationTimeoutMs = uploadhandler.DefaultEscalationTimeoutMs
	getProgressTier            = uploadhandler.GetProgressTier
	detectMimeType             = uploadhandler.DetectMimeType
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
func (u *UploadInteractHandler) HandleUpload(req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	var params uploadParams
	if resp, stop := mcp.ParseArgs(req, args, &params); stop {
		return resp
	}

	if errResp := validateUploadParams(req, params); errResp != nil {
		return *errResp
	}

	if resp, blocked := u.deps.RequirePilot(req); blocked {
		return resp
	}
	if resp, blocked := u.deps.RequireExtension(req); blocked {
		return resp
	}
	if resp, blocked := u.deps.RequireTabTracking(req); blocked {
		return resp
	}

	info, errResp := validateUploadFile(req, params.FilePath)
	if errResp != nil {
		return *errResp
	}

	return u.queueUpload(req, args, params, info)
}

// validateUploadParams checks required parameters for the upload action.
func validateUploadParams(req mcp.JSONRPCRequest, params uploadParams) *mcp.JSONRPCResponse {
	if resp, blocked := mcp.RequireString(req, params.FilePath, "file_path", "Add the 'file_path' parameter with an absolute path to the file"); blocked {
		return &resp
	}
	if params.Selector == "" && params.APIEndpoint == "" {
		resp := mcp.Fail(req, mcp.ErrMissingParam, "Required parameter 'selector' is missing. Provide a CSS selector for the file input element, or use 'api_endpoint' for direct API uploads.", "Add the 'selector' parameter (e.g., '#Filedata') or 'api_endpoint'", mcp.WithParam("selector"))
		return &resp
	}
	if !filepath.IsAbs(params.FilePath) {
		resp := mcp.Fail(req, mcp.ErrPathNotAllowed, "file_path must be an absolute path. Relative paths are not allowed for security.", "Use an absolute path like '/Users/user/Videos/video.mp4'", mcp.WithParam("file_path"))
		return &resp
	}
	return nil
}

// validateUploadFile checks that the file exists, is readable, and is not a directory.
func validateUploadFile(req mcp.JSONRPCRequest, filePath string) (os.FileInfo, *mcp.JSONRPCResponse) {
	info, err := os.Stat(filePath)
	if err != nil {
		resp := uploadFileStatError(req, filePath, err)
		return nil, &resp
	}
	if info.IsDir() {
		resp := mcp.Fail(req, mcp.ErrInvalidParam, "Path is a directory, not a file: "+filePath, "Provide a path to a file, not a directory", mcp.WithParam("file_path"))
		return nil, &resp
	}
	return info, nil
}

func uploadFileStatError(req mcp.JSONRPCRequest, filePath string, err error) mcp.JSONRPCResponse {
	if os.IsNotExist(err) {
		return mcp.Fail(req, mcp.ErrInvalidParam, "File not found: "+filePath+". Verify the file path is correct.", "Check the file path and try again", mcp.WithParam("file_path"))
	}
	if os.IsPermission(err) {
		return mcp.Fail(req, mcp.ErrPathNotAllowed, "Permission denied reading file: "+filePath+". Check file permissions.", "Fix file permissions with: chmod +r "+filePath, mcp.WithParam("file_path"))
	}
	return mcp.Fail(req, mcp.ErrInternal, "Failed to access file: "+err.Error(), "Check the file path and permissions")
}

// queueUpload builds the upload payload and queues it for the extension.
func (u *UploadInteractHandler) queueUpload(req mcp.JSONRPCRequest, args json.RawMessage, params uploadParams, info os.FileInfo) mcp.JSONRPCResponse {
	if params.EscalationTimeoutMs <= 0 {
		params.EscalationTimeoutMs = defaultEscalationTimeoutMs
	}

	fileName := filepath.Base(params.FilePath)
	mimeType := detectMimeType(fileName)
	fileSize := info.Size()
	progressTier := getProgressTier(fileSize)
	correlationID := mcp.NewCorrelationID("upload")
	u.actionHandler.ArmEvidenceForCommand(correlationID, "upload", args, req.ClientID)

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
	if enqueueResp, blocked := u.deps.EnqueuePendingQuery(req, query, 10*time.Minute); blocked {
		return enqueueResp
	}

	u.deps.RecordAIAction("upload", "", map[string]any{
		"file_path": params.FilePath, "file_name": fileName,
		"file_size": fileSize, "selector": params.Selector,
		"progress_tier": string(progressTier),
	})

	return mcp.Succeed(req, "Upload queued", map[string]any{
		"status": "queued", "correlation_id": correlationID,
		"file_name": fileName, "file_size": fileSize,
		"mime_type": mimeType, "progress_tier": string(progressTier),
		"message": "Upload queued for execution. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
	})
}
