// Purpose: Defines the dedicated interact upload sub-handler.
// Why: Narrows ToolHandler responsibilities by isolating upload-specific validation/queueing.
// Docs: docs/features/feature/file-upload/index.md

package main

type uploadInteractHandler struct {
	parent *ToolHandler
}

func newUploadInteractHandler(parent *ToolHandler) *uploadInteractHandler {
	return &uploadInteractHandler{parent: parent}
}
