// Purpose: Implements upload command handling, validation, and OS automation wiring.
// Why: Reduces upload flake by centralizing validation and secure browser-to-OS handoff behavior.
// Docs: docs/features/feature/file-upload/index.md

package main

import "github.com/dev-console/dev-console/internal/upload"

// Handler function aliases.
var (
	handleOSAutomationInternal = upload.HandleOSAutomation
	detectBrowserPID           = upload.DetectBrowserPID
	dismissFileDialogInternal  = upload.DismissFileDialog
	executeOSAutomation        = upload.ExecuteOSAutomation
)

// Validator and sanitizer function aliases.
var (
	validatePathForOSAutomation   = upload.ValidatePathForOSAutomation
	validateHTTPMethod            = upload.ValidateHTTPMethod
	validateFormActionURL         = upload.ValidateFormActionURL
	validateCookieHeader          = upload.ValidateCookieHeader
	sanitizeForContentDisposition = upload.SanitizeForContentDisposition
	sanitizeForAppleScript        = upload.SanitizeForAppleScript
	sanitizeForSendKeys           = upload.SanitizeForSendKeys
)

// handleOSAutomationInternalMethod is the ToolHandler method wrapper for testing.
func (h *ToolHandler) handleOSAutomationInternal(req OSAutomationInjectRequest) UploadStageResponse {
	return handleOSAutomationInternal(req, h.uploadSecurity)
}
