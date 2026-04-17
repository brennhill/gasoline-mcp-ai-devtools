// form_submit.go — Re-exports form submission handlers and file validation functions from internal/upload.
// Why: Keeps form submit logic in internal/upload while maintaining backward-compatible function references.
// Docs: docs/features/feature/file-upload/index.md

package uploadhandler

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"

// Function aliases for form submission handlers.
var (
	HandleFormSubmit    = upload.HandleFormSubmit
	HandleFormSubmitCtx = upload.HandleFormSubmitCtx
	ValidateFormSubmitFields = upload.ValidateFormSubmitFields
	OpenAndValidateFile      = upload.OpenAndValidateFile
	StreamMultipartForm      = upload.StreamMultipartForm
	ExecuteFormSubmit        = upload.ExecuteFormSubmit
)
