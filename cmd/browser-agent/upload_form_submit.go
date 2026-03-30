// Purpose: Re-exports form submission handlers from the uploadhandler sub-package.
// Why: Keeps form submit logic centralized while maintaining backward-compatible function references in cmd.
// Docs: docs/features/feature/file-upload/index.md

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"

// Function aliases for form submission handlers.
var (
	handleFormSubmitInternal    = uploadhandler.HandleFormSubmit
	handleFormSubmitInternalCtx = uploadhandler.HandleFormSubmitCtx
	validateFormSubmitFields    = uploadhandler.ValidateFormSubmitFields
	openAndValidateFile         = uploadhandler.OpenAndValidateFile
	streamMultipartForm         = uploadhandler.StreamMultipartForm
	executeFormSubmit           = uploadhandler.ExecuteFormSubmit
)
