// os_automation.go — Re-exports OS-level upload automation functions from internal/upload.
// Why: Stage 4 upload escalation requires OS automation gated behind --enable-os-upload-automation.
// Docs: docs/features/feature/file-upload/index.md

package uploadhandler

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"

// Handler function aliases.
var (
	HandleOSAutomation = upload.HandleOSAutomation
	DetectBrowserPID   = upload.DetectBrowserPID
	DismissFileDialog  = upload.DismissFileDialog
	ExecuteOSAutomation = upload.ExecuteOSAutomation
)

// Validator and sanitizer function aliases.
var (
	ValidatePathForOSAutomation   = upload.ValidatePathForOSAutomation
	ValidateHTTPMethod            = upload.ValidateHTTPMethod
	ValidateFormActionURL         = upload.ValidateFormActionURL
	ValidateCookieHeader          = upload.ValidateCookieHeader
	SanitizeForContentDisposition = upload.SanitizeForContentDisposition
	SanitizeForAppleScript        = upload.SanitizeForAppleScript
	SanitizeForSendKeys           = upload.SanitizeForSendKeys
)
