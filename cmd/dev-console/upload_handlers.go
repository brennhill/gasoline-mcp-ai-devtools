// upload_handlers.go — HTTP handlers for file upload stages 1-4.
// All endpoints require --enable-upload-automation flag.
package main

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

	resp := handleFileReadInternal(req, uploadSecurityConfig, false)
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

// handleFileReadInternal is the core logic for file read, testable without HTTP.
// Opens the file first, then fstats the open handle to avoid TOCTOU races.
// #lizard forgives
func handleFileReadInternal(req FileReadRequest, sec *UploadSecurity, requireUploadDir bool) FileReadResponse {
	if req.FilePath == "" {
		return FileReadResponse{
			Success: false,
			Error:   "Missing required parameter: file_path",
		}
	}

	// Security: full validation chain (Clean → IsAbs → EvalSymlinks → denylist → upload-dir)
	result, err := sec.ValidateFilePath(req.FilePath, requireUploadDir)
	if err != nil {
		return FileReadResponse{
			Success: false,
			Error:   err.Error(),
		}
	}

	// Open the resolved path (symlink-free, TOCTOU safe)
	// #nosec G304 -- file path validated by UploadSecurity chain
	file, err := os.Open(result.ResolvedPath)
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
			Error:   "Failed to access file: " + req.FilePath,
		}
	}
	defer file.Close() //nolint:errcheck // deferred close

	info, err := file.Stat()
	if err != nil {
		return FileReadResponse{
			Success: false,
			Error:   "Failed to stat file: " + req.FilePath,
		}
	}

	if info.IsDir() {
		return FileReadResponse{
			Success: false,
			Error:   "Path is a directory, not a file: " + req.FilePath,
		}
	}

	if err := checkHardlink(info); err != nil {
		return FileReadResponse{
			Success: false,
			Error:   err.Error(),
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

	// Only base64 encode files <= 100MB. Files above this threshold
	// return metadata only; use Stage 3 streaming for the actual upload.
	// Note: a 100MB file peaks at ~366MB RAM (raw + base64 + JSON buffer).
	if fileSize <= maxBase64FileSize {
		data, err := io.ReadAll(io.LimitReader(file, maxBase64FileSize+1))
		if err != nil {
			return FileReadResponse{
				Success: false,
				Error:   "Failed to read file: " + err.Error(),
			}
		}
		resp.DataBase64 = base64.StdEncoding.EncodeToString(data)
	}

	return resp
}

// handleFileReadInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleFileReadInternal(req FileReadRequest) FileReadResponse {
	return handleFileReadInternal(req, h.uploadSecurity, false)
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

	resp := handleDialogInjectInternal(req, uploadSecurityConfig)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
}

// handleDialogInjectInternal is the core logic for dialog injection, testable without HTTP.
// Stage 2 requires --upload-dir.
// #lizard forgives
func handleDialogInjectInternal(req FileDialogInjectRequest, sec *UploadSecurity) UploadStageResponse {
	if req.FilePath == "" {
		return UploadStageResponse{
			Success: false,
			Stage:   2,
			Error:   "Missing required parameter: file_path",
		}
	}

	if req.BrowserPID <= 0 {
		return UploadStageResponse{
			Success: false,
			Stage:   2,
			Error:   "Missing or invalid browser_pid. Provide the Chrome browser process ID.",
		}
	}

	// Security: full validation chain (requires upload-dir for Stage 2)
	result, err := sec.ValidateFilePath(req.FilePath, true)
	if err != nil {
		return UploadStageResponse{
			Success: false,
			Stage:   2,
			Error:   err.Error(),
		}
	}

	// Verify file exists via stat on resolved path
	info, err := os.Stat(result.ResolvedPath)
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
			Error:   "Failed to access file: " + req.FilePath,
		}
	}

	return UploadStageResponse{
		Success:       true,
		Stage:         2,
		Status:        "File dialog injection queued",
		FileName:      filepath.Base(result.ResolvedPath),
		FileSizeBytes: info.Size(),
	}
}

// handleDialogInjectInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleDialogInjectInternal(req FileDialogInjectRequest) UploadStageResponse {
	return handleDialogInjectInternal(req, h.uploadSecurity)
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

	resp := handleFormSubmitInternal(req, uploadSecurityConfig)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
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

	resp := handleOSAutomationInternal(req, uploadSecurityConfig)
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusBadRequest, resp)
	}
}
