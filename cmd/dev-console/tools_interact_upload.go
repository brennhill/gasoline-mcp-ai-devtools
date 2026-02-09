// tools_interact_upload.go — MCP interact upload action handler.
// Implements the "upload" action for the interact tool with 4-stage escalation.
// Requires --enable-upload-automation flag to be set on the server.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dev-console/dev-console/internal/queries"
)

// handleUpload dispatches the "upload" interact action.
// Validates parameters, checks security flags, and queues the upload operation.
func (h *ToolHandler) handleUpload(req JSONRPCRequest, args json.RawMessage) JSONRPCResponse {
	// Security gate: upload automation must be explicitly enabled
	if !h.uploadAutomationEnabled {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrUploadDisabled,
				"Upload automation is disabled. Start server with --enable-upload-automation flag.",
				"Restart the server with: gasoline-mcp --enable-upload-automation",
				withHint("Upload automation requires explicit opt-in for security. See docs/features/file-upload/tech-spec.md"),
			),
		}
	}

	// Parse upload parameters
	var params struct {
		Selector            string `json:"selector"`
		APIEndpoint         string `json:"apiEndpoint,omitempty"`
		FilePath            string `json:"file_path"`
		Submit              bool   `json:"submit,omitempty"`
		EscalationTimeoutMs int    `json:"escalation_timeout_ms,omitempty"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidJSON,
				"Invalid JSON arguments: "+err.Error(),
				"Fix JSON syntax and call again",
			),
		}
	}

	// Validate required parameters
	if params.FilePath == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'file_path' is missing",
				"Add the 'file_path' parameter with an absolute path to the file",
				withParam("file_path"),
			),
		}
	}

	if params.Selector == "" && params.APIEndpoint == "" {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrMissingParam,
				"Required parameter 'selector' is missing. Provide a CSS selector for the file input element, or use 'apiEndpoint' for direct API uploads.",
				"Add the 'selector' parameter (e.g., '#Filedata') or 'apiEndpoint'",
				withParam("selector"),
			),
		}
	}

	// Security: reject relative paths
	if !filepath.IsAbs(params.FilePath) {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrPathNotAllowed,
				"file_path must be an absolute path. Relative paths are not allowed for security.",
				"Use an absolute path like '/Users/user/Videos/video.mp4'",
				withParam("file_path"),
			),
		}
	}

	// Verify file exists and is readable
	info, err := os.Stat(params.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrInvalidParam,
					"File not found: "+params.FilePath+". Verify the file path is correct.",
					"Check the file path and try again",
					withParam("file_path"),
				),
			}
		}
		if os.IsPermission(err) {
			return JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mcpStructuredError(
					ErrPathNotAllowed,
					"Permission denied reading file: "+params.FilePath+". Check file permissions.",
					"Fix file permissions with: chmod +r "+params.FilePath,
					withParam("file_path"),
				),
			}
		}
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInternal,
				"Failed to access file: "+err.Error(),
				"Check the file path and permissions",
			),
		}
	}

	if info.IsDir() {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: mcpStructuredError(
				ErrInvalidParam,
				"Path is a directory, not a file: "+params.FilePath,
				"Provide a path to a file, not a directory",
				withParam("file_path"),
			),
		}
	}

	// Set default escalation timeout
	if params.EscalationTimeoutMs <= 0 {
		params.EscalationTimeoutMs = defaultEscalationTimeoutMs
	}

	// Determine file info for response
	fileName := filepath.Base(params.FilePath)
	mimeType := detectMimeType(fileName)
	fileSize := info.Size()
	progressTier := getProgressTier(fileSize)

	// Generate correlation ID for async tracking
	correlationID := fmt.Sprintf("upload_%d_%d", time.Now().UnixNano(), randomInt63())

	// Build upload command payload for the extension
	uploadPayload := map[string]any{
		"action":                "upload",
		"selector":              params.Selector,
		"file_path":             params.FilePath,
		"file_name":             fileName,
		"file_size":             fileSize,
		"mime_type":             mimeType,
		"submit":                params.Submit,
		"escalation_timeout_ms": params.EscalationTimeoutMs,
		"progress_tier":         string(progressTier),
	}
	if params.APIEndpoint != "" {
		uploadPayload["api_endpoint"] = params.APIEndpoint
	}

	payloadJSON, _ := json.Marshal(uploadPayload)

	// Queue upload command for extension to pick up
	query := queries.PendingQuery{
		Type:          "upload",
		Params:        payloadJSON,
		CorrelationID: correlationID,
	}
	h.capture.CreatePendingQueryWithTimeout(query, 10*time.Minute, req.ClientID)

	// Record AI action
	h.recordAIAction("upload", "", map[string]any{
		"file_path":     params.FilePath,
		"file_name":     fileName,
		"file_size":     fileSize,
		"selector":      params.Selector,
		"progress_tier": string(progressTier),
	})

	// Return queued status
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: mcpJSONResponse("Upload queued", map[string]any{
			"status":         "queued",
			"correlation_id": correlationID,
			"file_name":      fileName,
			"file_size":      fileSize,
			"mime_type":      mimeType,
			"progress_tier":  string(progressTier),
			"message":        "Upload queued for execution. Use observe({what: 'command_result', correlation_id: '" + correlationID + "'}) to get the result.",
		}),
	}
}
