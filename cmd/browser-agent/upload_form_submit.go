// Purpose: Re-exports form submission handlers and file validation functions from internal/upload for interact upload stages.
// Why: Keeps form submit logic in internal/upload while maintaining backward-compatible function references in cmd.
// Docs: docs/features/feature/file-upload/index.md

package main

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/upload"

// Function aliases for form submission handlers.
var (
	handleFormSubmitInternal    = upload.HandleFormSubmit
	handleFormSubmitInternalCtx = upload.HandleFormSubmitCtx
	validateFormSubmitFields    = upload.ValidateFormSubmitFields
	openAndValidateFile         = upload.OpenAndValidateFile
	streamMultipartForm         = upload.StreamMultipartForm
	executeFormSubmit           = upload.ExecuteFormSubmit
)

