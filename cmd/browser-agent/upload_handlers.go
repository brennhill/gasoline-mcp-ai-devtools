// Purpose: Delegates upload HTTP routes to the uploadhandler sub-package.
// Why: Keeps Server methods thin; core upload HTTP logic lives in the uploadhandler package.
// Docs: docs/features/feature/file-upload/index.md

package main

import (
	"net/http"

	"github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"
)

// handleFileReadInternal delegates to uploadhandler for file read logic.
// Used by test helpers and the HTTP handler.
var handleFileReadInternal = uploadhandler.HandleFileRead

// handleDialogInjectInternal delegates to uploadhandler for dialog injection logic.
// Used by test helpers and the HTTP handler.
var handleDialogInjectInternal = uploadhandler.HandleDialogInject

// handleFileRead serves stage-1 file metadata reads for upload workflows.
func (s *Server) handleFileRead(w http.ResponseWriter, r *http.Request) {
	uploadhandler.HandleFileReadHTTP(w, r, uploadSecurityConfig, jsonResponse)
}

// handleFileDialogInject serves stage-2 dialog injection preparation.
func (s *Server) handleFileDialogInject(w http.ResponseWriter, r *http.Request) {
	uploadhandler.HandleFileDialogInjectHTTP(w, r, uploadSecurityConfig, jsonResponse)
}

// handleFormSubmit serves stage-3 submit orchestration for upload flows.
func (s *Server) handleFormSubmit(w http.ResponseWriter, r *http.Request) {
	uploadhandler.HandleFormSubmitHTTP(w, r, uploadSecurityConfig, jsonResponse)
}

// handleOSAutomation serves stage-4 OS automation bridge.
func (s *Server) handleOSAutomation(w http.ResponseWriter, r *http.Request, osAutomationEnabled bool) {
	uploadhandler.HandleOSAutomationHTTP(w, r, osAutomationEnabled, uploadSecurityConfig, jsonResponse)
}

// handleOSAutomationDismiss sends Escape to close an orphaned native file dialog.
func (s *Server) handleOSAutomationDismiss(w http.ResponseWriter, r *http.Request, osAutomationEnabled bool) {
	uploadhandler.HandleOSAutomationDismissHTTP(w, r, osAutomationEnabled, jsonResponse)
}
