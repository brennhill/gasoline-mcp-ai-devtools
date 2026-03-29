// Purpose: Implements screenshot capture response handling for observe modes.
// Docs: docs/features/feature/observe/index.md

package observe

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/mcp"
	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/queries"
)

// GetScreenshot captures a screenshot of the current page via the extension.
func GetScreenshot(deps Deps, req mcp.JSONRPCRequest, args json.RawMessage) mcp.JSONRPCResponse {
	cap := deps.GetCapture()
	enabled, _, _ := cap.GetTrackingStatus()
	if !enabled {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrNoData, "No tab is being tracked. Open the Kaboom extension popup and click 'Track This Tab' on the page you want to monitor. Check observe with what='pilot' for extension status.", "", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	var params struct {
		Format        string `json:"format,omitempty"`
		Quality       int    `json:"quality,omitempty"`
		FullPage      bool   `json:"full_page,omitempty"`
		Selector      string `json:"selector,omitempty"`
		WaitForStable bool   `json:"wait_for_stable,omitempty"`
		SaveTo        string `json:"save_to,omitempty"`
	}
	mcp.LenientUnmarshal(args, &params)

	if params.Format != "" && params.Format != "png" && params.Format != "jpeg" {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, "Invalid screenshot format: "+params.Format,
			"Use 'png' or 'jpeg'", mcp.WithParam("format"),
		)}
	}

	if params.Quality != 0 && (params.Quality < 1 || params.Quality > 100) {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(
			mcp.ErrInvalidParam, fmt.Sprintf("Invalid quality: %d (must be 1-100)", params.Quality),
			"Use a value between 1 and 100", mcp.WithParam("quality"),
		)}
	}

	screenshotParams := map[string]any{}
	if params.Format != "" {
		screenshotParams["format"] = params.Format
	}
	if params.Quality > 0 {
		screenshotParams["quality"] = params.Quality
	}
	if params.FullPage {
		screenshotParams["full_page"] = true
	}
	if params.Selector != "" {
		screenshotParams["selector"] = params.Selector
	}
	if params.WaitForStable {
		screenshotParams["wait_for_stable"] = true
	}

	queryParams, _ := json.Marshal(screenshotParams)

	queryID, qerr := cap.CreatePendingQueryWithTimeout(
		queries.PendingQuery{
			Type:   "screenshot",
			Params: queryParams,
		},
		20*time.Second,
		"",
	)
	if qerr != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrQueueFull, "Command queue full: "+qerr.Error(), "Wait for in-flight commands to complete, then retry.",
			mcp.WithRecoveryToolCall(map[string]any{"tool": "observe", "arguments": map[string]any{"what": "pending_commands"}}),
		)}
	}

	result, err := cap.WaitForResult(queryID, 20*time.Second)
	if err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrExtTimeout, "Screenshot capture timeout: "+err.Error(), "Ensure the extension is connected and the page has loaded. Try refreshing the page, then retry.", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	var screenshotResult map[string]any
	if err := json.Unmarshal(result, &screenshotResult); err != nil {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrInvalidJSON, "Failed to parse screenshot result: "+err.Error(), "Check extension logs for errors")}
	}

	if errMsg, ok := screenshotResult["error"].(string); ok {
		return mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: mcp.StructuredErrorResponse(mcp.ErrExtError, "Screenshot capture failed: "+errMsg, "Check that the tab is visible and accessible. The extension reported an error.", mcp.WithHint(deps.DiagnosticHintString()))}
	}

	// Extract data_url before building text block to avoid duplicating
	// the large base64 payload in both text and image content blocks.
	var dataURL string
	if du, ok := screenshotResult["data_url"].(string); ok && du != "" {
		dataURL = du
		delete(screenshotResult, "data_url")
	}

	// #386: save_to — copy screenshot to user-specified path
	if params.SaveTo != "" && dataURL != "" {
		if saveErr := saveScreenshotToPath(params.SaveTo, dataURL); saveErr != nil {
			screenshotResult["save_to_error"] = saveErr.Error()
		} else {
			screenshotResult["save_to"] = params.SaveTo
		}
	}

	// Build text response with file path info (backward compatible)
	resp := mcp.Succeed(req, "Screenshot captured", screenshotResult)

	// Append inline image content block if data_url was present
	if dataURL != "" {
		base64Data, mimeType := parseDataURL(dataURL)
		if base64Data != "" {
			resp = mcp.AppendImageToResponse(resp, base64Data, mimeType)
		}
	}

	return resp
}

// parseDataURL extracts the base64 data and MIME type from a data URL.
// Example: "data:image/jpeg;base64,/9j/4AAQ..." -> ("/9j/4AAQ...", "image/jpeg")
// Returns empty strings if the data URL format is invalid.
func parseDataURL(dataURL string) (base64Data, mimeType string) {
	if !strings.HasPrefix(dataURL, "data:") {
		return "", ""
	}
	// Format: data:<mimeType>;base64,<data>
	rest := dataURL[5:] // strip "data:"
	semicolonIdx := strings.Index(rest, ";")
	if semicolonIdx < 0 {
		return "", ""
	}
	mimeType = rest[:semicolonIdx]
	rest = rest[semicolonIdx+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return "", ""
	}
	base64Data = rest[7:] // strip "base64,"
	return base64Data, mimeType
}

// saveScreenshotToPath saves a screenshot data URL to a user-specified file path (#386).
// Creates parent directories if needed. Only allows .png and .jpeg/.jpg extensions.
func saveScreenshotToPath(saveTo string, dataURL string) error {
	// Validate the path is absolute
	absPath, err := filepath.Abs(saveTo)
	if err != nil {
		return fmt.Errorf("invalid save_to path: %w", err)
	}

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(absPath))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return fmt.Errorf("save_to must have .png, .jpg, or .jpeg extension, got %q", ext)
	}

	// Decode the data URL
	b64Data, _ := parseDataURL(dataURL)
	if b64Data == "" {
		return fmt.Errorf("screenshot_save: invalid data URL format. Expected 'data:image/...;base64,...'")
	}

	imageData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return fmt.Errorf("failed to decode image data: %w", err)
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write the file
	// #nosec G306 -- user-specified path for screenshot save
	if err := os.WriteFile(absPath, imageData, 0o644); err != nil {
		return fmt.Errorf("failed to write screenshot: %w", err)
	}

	return nil
}
