// upload_os_automation.go — Stage 4 OS automation and validators delegating to internal/upload.
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
