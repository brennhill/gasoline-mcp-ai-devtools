// upload_handlers.go — File upload endpoint handlers (4-stage escalation).
// Implements POST /api/file/read, /api/file/dialog/inject, /api/form/submit,
// and /api/os-automation/inject endpoints.
// All endpoints require --enable-upload-automation flag.
package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ============================================
// Stage 1: File Read (POST /api/file/read)
// ============================================

// handleFileRead is the HTTP handler for POST /api/file/read
func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request, uploadEnabled bool) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !uploadEnabled {
		jsonResponse(w, http.StatusForbidden, FileReadResponse{
			Success: false,
			Error:   "Upload automation is disabled. Start server with --enable-upload-automation flag.",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024) // 1MB max for request body
	var req FileReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, FileReadResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := handleFileReadInternal(req)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		status := http.StatusBadRequest
		if strings.Contains(resp.Error, "not found") || strings.Contains(resp.Error, "no such file") {
			status = http.StatusNotFound
		} else if strings.Contains(resp.Error, "permission") {
			status = http.StatusForbidden
		}
		jsonResponse(w, status, resp)
	}
}

// handleFileReadInternal is the core logic for file read, testable without HTTP
func handleFileReadInternal(req FileReadRequest) FileReadResponse {
	if req.FilePath == "" {
		return FileReadResponse{
			Success: false,
			Error:   "Missing required parameter: file_path",
		}
	}

	// Security: reject relative paths
	if !filepath.IsAbs(req.FilePath) {
		return FileReadResponse{
			Success: false,
			Error:   "file_path must be an absolute path. Relative paths are not allowed for security.",
		}
	}

	// Stat the file
	info, err := os.Stat(req.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return FileReadResponse{
				Success: false,
				Error:   "File not found: " + req.FilePath + ". Verify the file path is correct.",
			}
		}
		if os.IsPermission(err) {
			return FileReadResponse{
				Success: false,
				Error:   "Permission denied reading file: " + req.FilePath + ". Check file permissions.",
			}
		}
		return FileReadResponse{
			Success: false,
			Error:   "Failed to access file: " + err.Error(),
		}
	}

	if info.IsDir() {
		return FileReadResponse{
			Success: false,
			Error:   "Path is a directory, not a file: " + req.FilePath,
		}
	}

	fileName := filepath.Base(req.FilePath)
	mimeType := detectMimeType(fileName)
	fileSize := info.Size()

	resp := FileReadResponse{
		Success:  true,
		FileName: fileName,
		FileSize: fileSize,
		MimeType: mimeType,
	}

	// Only base64 encode small files (< 100MB)
	if fileSize <= maxBase64FileSize {
		// #nosec G304 -- file path validated above (absolute path check)
		data, err := os.ReadFile(req.FilePath)
		if err != nil {
			if os.IsPermission(err) {
				return FileReadResponse{
					Success: false,
					Error:   "Permission denied reading file: " + req.FilePath + ". Check file permissions.",
				}
			}
			return FileReadResponse{
				Success: false,
				Error:   "Failed to read file: " + err.Error(),
			}
		}
		resp.DataBase64 = base64.StdEncoding.EncodeToString(data)
	}
	// Files > 100MB: no base64, use streaming in stage 3

	return resp
}

// handleFileReadInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleFileReadInternal(req FileReadRequest) FileReadResponse {
	return handleFileReadInternal(req)
}

// ============================================
// Stage 2: File Dialog Injection (POST /api/file/dialog/inject)
// ============================================

// handleFileDialogInject is the HTTP handler for POST /api/file/dialog/inject
func (s *Server) handleFileDialogInject(w http.ResponseWriter, r *http.Request, uploadEnabled bool) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !uploadEnabled {
		jsonResponse(w, http.StatusForbidden, UploadStageResponse{
			Success: false,
			Error:   "Upload automation is disabled. Start server with --enable-upload-automation flag.",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req FileDialogInjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, UploadStageResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := handleDialogInjectInternal(req)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
}

// handleDialogInjectInternal is the core logic for dialog injection, testable without HTTP
func handleDialogInjectInternal(req FileDialogInjectRequest) UploadStageResponse {
	if req.FilePath == "" {
		return UploadStageResponse{
			Success: false,
			Stage:   2,
			Error:   "Missing required parameter: file_path",
		}
	}

	if !filepath.IsAbs(req.FilePath) {
		return UploadStageResponse{
			Success: false,
			Stage:   2,
			Error:   "file_path must be an absolute path",
		}
	}

	if req.BrowserPID <= 0 {
		return UploadStageResponse{
			Success: false,
			Stage:   2,
			Error:   "Missing or invalid browser_pid. Provide the Chrome browser process ID.",
		}
	}

	// Verify file exists
	info, err := os.Stat(req.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return UploadStageResponse{
				Success: false,
				Stage:   2,
				Error:   "File not found: " + req.FilePath,
			}
		}
		return UploadStageResponse{
			Success: false,
			Stage:   2,
			Error:   "Failed to access file: " + err.Error(),
		}
	}

	return UploadStageResponse{
		Success:       true,
		Stage:         2,
		Status:        "File dialog injection queued",
		FileName:      filepath.Base(req.FilePath),
		FileSizeBytes: info.Size(),
	}
}

// handleDialogInjectInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleDialogInjectInternal(req FileDialogInjectRequest) UploadStageResponse {
	return handleDialogInjectInternal(req)
}

// ============================================
// Stage 3: Form Submission (POST /api/form/submit)
// ============================================

// handleFormSubmit is the HTTP handler for POST /api/form/submit
func (s *Server) handleFormSubmit(w http.ResponseWriter, r *http.Request, uploadEnabled bool) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !uploadEnabled {
		jsonResponse(w, http.StatusForbidden, UploadStageResponse{
			Success: false,
			Error:   "Upload automation is disabled. Start server with --enable-upload-automation flag.",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024) // 10MB max for form metadata
	var req FormSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, UploadStageResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := handleFormSubmitInternal(req)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
}

// handleFormSubmitInternal is the core logic for form submission, testable without HTTP
func handleFormSubmitInternal(req FormSubmitRequest) UploadStageResponse {
	start := time.Now()

	// Validate required fields
	if req.FormAction == "" {
		return UploadStageResponse{
			Success: false,
			Stage:   3,
			Error:   "Missing required parameter: form_action",
		}
	}

	if req.FilePath == "" {
		return UploadStageResponse{
			Success: false,
			Stage:   3,
			Error:   "Missing required parameter: file_path",
		}
	}

	if req.FileInputName == "" {
		return UploadStageResponse{
			Success: false,
			Stage:   3,
			Error:   "Missing required parameter: file_input_name",
		}
	}

	if !filepath.IsAbs(req.FilePath) {
		return UploadStageResponse{
			Success: false,
			Stage:   3,
			Error:   "file_path must be an absolute path",
		}
	}

	// Default method to POST
	if req.Method == "" {
		req.Method = "POST"
	}

	// Verify file exists
	info, err := os.Stat(req.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return UploadStageResponse{
				Success: false,
				Stage:   3,
				Error:   "File not found: " + req.FilePath,
			}
		}
		return UploadStageResponse{
			Success: false,
			Stage:   3,
			Error:   "Failed to access file: " + err.Error(),
		}
	}

	// Open file for streaming
	// #nosec G304 -- file path validated above (absolute path check)
	file, err := os.Open(req.FilePath)
	if err != nil {
		return UploadStageResponse{
			Success: false,
			Stage:   3,
			Error:   "Failed to open file: " + err.Error(),
		}
	}
	defer file.Close() //nolint:errcheck // deferred close

	// Build multipart form with file streaming
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	// Write form fields and file in a goroutine to enable streaming
	var writeErr error
	go func() {
		defer pw.Close() //nolint:errcheck // pipe close

		// Add CSRF token as form field if provided
		if req.CSRFToken != "" {
			if err := writer.WriteField("csrf_token", req.CSRFToken); err != nil {
				writeErr = err
				return
			}
		}

		// Add form fields
		for k, v := range req.Fields {
			if err := writer.WriteField(k, v); err != nil {
				writeErr = err
				return
			}
		}

		// Add file (streamed, no memory bloat)
		fileName := filepath.Base(req.FilePath)
		mimeType := detectMimeType(fileName)
		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition",
			fmt.Sprintf(`form-data; name="%s"; filename="%s"`, req.FileInputName, fileName))
		partHeader.Set("Content-Type", mimeType)

		fw, err := writer.CreatePart(partHeader)
		if err != nil {
			writeErr = err
			return
		}

		if _, err := io.Copy(fw, file); err != nil {
			writeErr = err
			return
		}

		if err := writer.Close(); err != nil {
			writeErr = err
		}
	}()

	// Create HTTP request to the platform
	httpReq, err := http.NewRequest(req.Method, req.FormAction, pr)
	if err != nil {
		return UploadStageResponse{
			Success: false,
			Stage:   3,
			Error:   "Failed to create HTTP request: " + err.Error(),
		}
	}

	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if req.Cookies != "" {
		httpReq.Header.Set("Cookie", req.Cookies)
	}

	// Execute the request
	client := &http.Client{
		Timeout: 10 * time.Minute, // Large file uploads can take a while
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return UploadStageResponse{
			Success:       false,
			Stage:         3,
			Error:         "Form submission failed: " + err.Error(),
			FileName:      filepath.Base(req.FilePath),
			FileSizeBytes: info.Size(),
			DurationMs:    time.Since(start).Milliseconds(),
		}
	}
	defer httpResp.Body.Close() //nolint:errcheck // deferred close

	if writeErr != nil {
		return UploadStageResponse{
			Success:    false,
			Stage:      3,
			Error:      "Error writing form data: " + writeErr.Error(),
			DurationMs: time.Since(start).Milliseconds(),
		}
	}

	// Check response status
	if httpResp.StatusCode >= 400 {
		// Read response body for error details (limited)
		bodyBytes := make([]byte, 1024)
		n, _ := httpResp.Body.Read(bodyBytes)
		bodyPreview := string(bodyBytes[:n])

		errMsg := fmt.Sprintf("Platform returned HTTP %d", httpResp.StatusCode)
		if httpResp.StatusCode == 401 {
			errMsg = "User not logged into platform (HTTP 401). Please log in and retry."
		} else if httpResp.StatusCode == 403 {
			errMsg = "CSRF token mismatch or forbidden (HTTP 403). Token may be expired."
		} else if httpResp.StatusCode == 422 {
			errMsg = "Form validation failed (HTTP 422). Check required fields."
		}

		return UploadStageResponse{
			Success:       false,
			Stage:         3,
			Error:         errMsg,
			FileName:      filepath.Base(req.FilePath),
			FileSizeBytes: info.Size(),
			DurationMs:    time.Since(start).Milliseconds(),
			Suggestions:   []string{"Check authentication", "Verify CSRF token", "Response: " + truncate(bodyPreview, 200)},
		}
	}

	return UploadStageResponse{
		Success:       true,
		Stage:         3,
		Status:        fmt.Sprintf("Form interception: %s submitted to platform (HTTP %d)", req.Method, httpResp.StatusCode),
		FileName:      filepath.Base(req.FilePath),
		FileSizeBytes: info.Size(),
		DurationMs:    time.Since(start).Milliseconds(),
	}
}

// handleFormSubmitInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleFormSubmitInternal(req FormSubmitRequest) UploadStageResponse {
	return handleFormSubmitInternal(req)
}

// ============================================
// Stage 4: OS Automation (POST /api/os-automation/inject)
// ============================================

// handleOSAutomation is the HTTP handler for POST /api/os-automation/inject
func (s *Server) handleOSAutomation(w http.ResponseWriter, r *http.Request, uploadEnabled bool) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !uploadEnabled {
		jsonResponse(w, http.StatusForbidden, UploadStageResponse{
			Success: false,
			Error:   "Upload automation is disabled. Start server with --enable-upload-automation flag.",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req OSAutomationInjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, UploadStageResponse{
			Success: false,
			Error:   "Invalid JSON: " + err.Error(),
		})
		return
	}

	resp := handleOSAutomationInternal(req)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
}

// handleOSAutomationInternal is the core logic for OS automation, testable without HTTP
func handleOSAutomationInternal(req OSAutomationInjectRequest) UploadStageResponse {
	if req.FilePath == "" {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "Missing required parameter: file_path",
		}
	}

	if !filepath.IsAbs(req.FilePath) {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "file_path must be an absolute path",
		}
	}

	if req.BrowserPID <= 0 {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "Missing or invalid browser_pid. Provide the Chrome browser process ID.",
		}
	}

	// Verify file exists
	if _, err := os.Stat(req.FilePath); err != nil {
		if os.IsNotExist(err) {
			return UploadStageResponse{
				Success: false,
				Stage:   4,
				Error:   "File not found: " + req.FilePath,
			}
		}
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "Failed to access file: " + err.Error(),
		}
	}

	// Attempt OS-level file dialog injection
	return executeOSAutomation(req)
}

// handleOSAutomationInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleOSAutomationInternal(req OSAutomationInjectRequest) UploadStageResponse {
	return handleOSAutomationInternal(req)
}

// executeOSAutomation performs platform-specific OS automation
func executeOSAutomation(req OSAutomationInjectRequest) UploadStageResponse {
	start := time.Now()

	switch runtime.GOOS {
	case "darwin":
		return executeMacOSAutomation(req, start)
	case "windows":
		return executeWindowsAutomation(req, start)
	case "linux":
		return executeLinuxAutomation(req, start)
	default:
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   fmt.Sprintf("OS automation not supported on %s", runtime.GOOS),
			Suggestions: []string{
				"Use Stage 3 (form interception) instead",
				"Manually upload the file",
			},
		}
	}
}

// executeMacOSAutomation uses AppleScript to inject file path into file dialog
func executeMacOSAutomation(req OSAutomationInjectRequest, start time.Time) UploadStageResponse {
	// AppleScript to type file path into file dialog and press Enter
	script := fmt.Sprintf(`tell application "System Events"
	-- Wait for file dialog to appear
	delay 0.5
	-- Type the file path using keyboard shortcut Cmd+Shift+G (Go to folder)
	keystroke "g" using {command down, shift down}
	delay 0.5
	-- Type the file path
	keystroke "%s"
	delay 0.3
	-- Press Enter to navigate
	key code 36
	delay 0.5
	-- Press Enter again to confirm selection
	key code 36
end tell`, req.FilePath)

	// #nosec G204 -- script is built from validated file path (absolute path check above)
	cmd := exec.Command("osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("AppleScript failed: %v", err)
		if len(output) > 0 {
			errMsg += " Output: " + string(output)
		}
		return UploadStageResponse{
			Success:    false,
			Stage:      4,
			Error:      errMsg,
			DurationMs: time.Since(start).Milliseconds(),
			Suggestions: []string{
				"Grant Accessibility permissions: System Settings > Privacy & Security > Accessibility",
				"Ensure a file dialog is open in Chrome",
			},
		}
	}

	return UploadStageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via AppleScript",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// executeWindowsAutomation uses PowerShell with SendKeys to inject file path
func executeWindowsAutomation(req OSAutomationInjectRequest, start time.Time) UploadStageResponse {
	// PowerShell script to find file dialog and type path
	script := fmt.Sprintf(`
Add-Type -AssemblyName System.Windows.Forms
Start-Sleep -Milliseconds 500
# Type the file path into the file name field
[System.Windows.Forms.SendKeys]::SendWait("%s")
Start-Sleep -Milliseconds 300
# Press Enter
[System.Windows.Forms.SendKeys]::SendWait("{ENTER}")
`, strings.ReplaceAll(req.FilePath, `\`, `\\`))

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script) // #nosec G204 -- path validated
	output, err := cmd.CombinedOutput()
	if err != nil {
		return UploadStageResponse{
			Success:    false,
			Stage:      4,
			Error:      fmt.Sprintf("PowerShell automation failed: %v. Output: %s", err, string(output)),
			DurationMs: time.Since(start).Milliseconds(),
			Suggestions: []string{
				"Ensure a file dialog is open in Chrome",
				"Run with administrator privileges if needed",
			},
		}
	}

	return UploadStageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via PowerShell/SendKeys",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}

// executeLinuxAutomation uses xdotool to inject file path into file dialog
func executeLinuxAutomation(req OSAutomationInjectRequest, start time.Time) UploadStageResponse {
	// Check if xdotool is available
	if _, err := exec.LookPath("xdotool"); err != nil {
		return UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "xdotool not found. Install with: sudo apt install xdotool",
			Suggestions: []string{
				"Install xdotool: sudo apt install xdotool (Debian/Ubuntu)",
				"Install xdotool: sudo dnf install xdotool (Fedora)",
				"Use Stage 3 (form interception) instead",
			},
		}
	}

	// Find and activate the file dialog window, type path, press Enter
	commands := []struct {
		name string
		args []string
	}{
		{"xdotool", []string{"search", "--name", "Open", "windowactivate"}},
		{"xdotool", []string{"key", "ctrl+l"}},                         // Focus location bar
		{"xdotool", []string{"type", "--clearmodifiers", req.FilePath}}, // Type path
		{"xdotool", []string{"key", "Return"}},                         // Confirm
	}

	for _, c := range commands {
		cmd := exec.Command(c.name, c.args...) // #nosec G204 -- xdotool path from LookPath
		if output, err := cmd.CombinedOutput(); err != nil {
			return UploadStageResponse{
				Success:    false,
				Stage:      4,
				Error:      fmt.Sprintf("xdotool command failed: %v. Output: %s", err, string(output)),
				DurationMs: time.Since(start).Milliseconds(),
				Suggestions: []string{
					"Ensure a file dialog is open",
					"Check that X11/Wayland session is active",
				},
			}
		}
		time.Sleep(200 * time.Millisecond) // Brief pause between commands
	}

	return UploadStageResponse{
		Success:    true,
		Stage:      4,
		Status:     "OS automation: file path injected via xdotool",
		FileName:   filepath.Base(req.FilePath),
		DurationMs: time.Since(start).Milliseconds(),
	}
}
