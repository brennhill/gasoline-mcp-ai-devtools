// Purpose: Re-exports OS-level upload automation functions (browser PID detection, file dialog dismiss, native input) from internal/upload.
// Why: Stage 4 upload escalation requires OS automation gated behind --enable-os-upload-automation.
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
