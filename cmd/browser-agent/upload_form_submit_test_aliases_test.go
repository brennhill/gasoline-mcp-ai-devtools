package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"

var (
	handleFormSubmitInternal    = uploadhandler.HandleFormSubmit
	handleFormSubmitInternalCtx = uploadhandler.HandleFormSubmitCtx
	validateFormSubmitFields    = uploadhandler.ValidateFormSubmitFields
	openAndValidateFile         = uploadhandler.OpenAndValidateFile
	streamMultipartForm         = uploadhandler.StreamMultipartForm
	executeFormSubmit           = uploadhandler.ExecuteFormSubmit
)
