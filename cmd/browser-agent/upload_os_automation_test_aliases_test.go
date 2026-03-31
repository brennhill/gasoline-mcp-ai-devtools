package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"

var (
	handleOSAutomationInternal = uploadhandler.HandleOSAutomation
	detectBrowserPID           = uploadhandler.DetectBrowserPID
	dismissFileDialogInternal  = uploadhandler.DismissFileDialog
	executeOSAutomation        = uploadhandler.ExecuteOSAutomation
)

var (
	validatePathForOSAutomation   = uploadhandler.ValidatePathForOSAutomation
	validateHTTPMethod            = uploadhandler.ValidateHTTPMethod
	validateFormActionURL         = uploadhandler.ValidateFormActionURL
	validateCookieHeader          = uploadhandler.ValidateCookieHeader
	sanitizeForContentDisposition = uploadhandler.SanitizeForContentDisposition
	sanitizeForAppleScript        = uploadhandler.SanitizeForAppleScript
	sanitizeForSendKeys           = uploadhandler.SanitizeForSendKeys
)
