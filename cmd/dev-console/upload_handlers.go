// upload_handlers.go — HTTP handlers for file upload stages 1-4.
// Stage 4 requires --enable-os-upload-automation flag.
package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dev-console/dev-console/internal/upload"
)

// handleFileReadInternal is the core logic for file read, delegating to internal/upload.
var handleFileReadInternal = upload.HandleFileRead

// handleDialogInjectInternal is the core logic for dialog injection, delegating to internal/upload.
var handleDialogInjectInternal = upload.HandleDialogInject

// ============================================
// Stage 1: File Read (POST /api/file/read)
// ============================================

// handleFileRead is the HTTP handler for POST /api/file/read
func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
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

// handleFileReadInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleFileReadInternal(req FileReadRequest) FileReadResponse {
	return handleFileReadInternal(req, h.uploadSecurity, false)
}

// ============================================
// Stage 2: File Dialog Injection (POST /api/file/dialog/inject)
// ============================================

// handleFileDialogInject is the HTTP handler for POST /api/file/dialog/inject
func (s *Server) handleFileDialogInject(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
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

// handleDialogInjectInternalMethod is the ToolHandler method wrapper for testing
func (h *ToolHandler) handleDialogInjectInternal(req FileDialogInjectRequest) UploadStageResponse {
	return handleDialogInjectInternal(req, h.uploadSecurity)
}

// ============================================
// Stage 3: Form Submission (POST /api/form/submit)
// ============================================

// handleFormSubmit is the HTTP handler for POST /api/form/submit
func (s *Server) handleFormSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
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
func (s *Server) handleOSAutomation(w http.ResponseWriter, r *http.Request, osAutomationEnabled bool) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !osAutomationEnabled {
		jsonResponse(w, http.StatusForbidden, UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "OS-level upload automation is disabled. Start server with --enable-os-upload-automation flag.",
		})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
	var req OSAutomationInjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, UploadStageResponse{
			Success: false,
			Stage:   4,
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

// handleOSAutomationDismiss sends Escape to close a dangling native file dialog.
func (s *Server) handleOSAutomationDismiss(w http.ResponseWriter, r *http.Request, osAutomationEnabled bool) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "Method not allowed"})
		return
	}

	if !osAutomationEnabled {
		jsonResponse(w, http.StatusForbidden, UploadStageResponse{
			Success: false,
			Stage:   4,
			Error:   "OS automation is disabled.",
		})
		return
	}

	resp := dismissFileDialogInternal()
	if resp.Success {
		jsonResponse(w, http.StatusOK, resp)
	} else {
		jsonResponse(w, http.StatusInternalServerError, resp)
	}
}
