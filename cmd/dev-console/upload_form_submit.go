// upload_form_submit.go — Stage 3 form submission delegates to internal/upload.
package main

import "github.com/dev-console/dev-console/internal/upload"

// Function aliases for form submission handlers.
var (
	handleFormSubmitInternal    = upload.HandleFormSubmit
	handleFormSubmitInternalCtx = upload.HandleFormSubmitCtx
	validateFormSubmitFields    = upload.ValidateFormSubmitFields
	openAndValidateFile         = upload.OpenAndValidateFile
	streamMultipartForm         = upload.StreamMultipartForm
	executeFormSubmit           = upload.ExecuteFormSubmit
)

// handleFormSubmitInternalMethod is the ToolHandler method wrapper for testing.
func (h *ToolHandler) handleFormSubmitInternal(req FormSubmitRequest) UploadStageResponse {
	return handleFormSubmitInternal(req, h.uploadSecurity)
}
