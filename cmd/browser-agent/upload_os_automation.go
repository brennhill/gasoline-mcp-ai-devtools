// Purpose: Re-exports OS-level upload automation functions from the uploadhandler sub-package.
// Why: Stage 4 upload escalation requires OS automation gated behind --enable-os-upload-automation.
// Docs: docs/features/feature/file-upload/index.md

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"

// Handler function aliases.
var (
	handleOSAutomationInternal = uploadhandler.HandleOSAutomation
	detectBrowserPID           = uploadhandler.DetectBrowserPID
	dismissFileDialogInternal  = uploadhandler.DismissFileDialog
	executeOSAutomation        = uploadhandler.ExecuteOSAutomation
)

// Validator and sanitizer function aliases.
var (
	validatePathForOSAutomation   = uploadhandler.ValidatePathForOSAutomation
	validateHTTPMethod            = uploadhandler.ValidateHTTPMethod
	validateFormActionURL         = uploadhandler.ValidateFormActionURL
	validateCookieHeader          = uploadhandler.ValidateCookieHeader
	sanitizeForContentDisposition = uploadhandler.SanitizeForContentDisposition
	sanitizeForAppleScript        = uploadhandler.SanitizeForAppleScript
	sanitizeForSendKeys           = uploadhandler.SanitizeForSendKeys
)
