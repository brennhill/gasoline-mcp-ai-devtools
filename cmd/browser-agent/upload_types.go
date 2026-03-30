// Purpose: Re-exports upload request/response wire types from the uploadhandler sub-package.
// Why: Keeps wire type definitions centralized while making them available as short aliases in cmd.
// Docs: docs/features/feature/file-upload/index.md

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"

// ============================================
// Type Aliases
// ============================================

type FileReadRequest = uploadhandler.FileReadRequest
type FileReadResponse = uploadhandler.FileReadResponse
type FileDialogInjectRequest = uploadhandler.FileDialogInjectRequest
type FormSubmitRequest = uploadhandler.FormSubmitRequest
type OSAutomationInjectRequest = uploadhandler.OSAutomationInjectRequest
type UploadStageResponse = uploadhandler.StageResponse
type ProgressTier = uploadhandler.ProgressTier

// ============================================
// Constants
// ============================================

const (
	ProgressTierSimple   = uploadhandler.ProgressTierSimple
	ProgressTierPeriodic = uploadhandler.ProgressTierPeriodic
	ProgressTierDetailed = uploadhandler.ProgressTierDetailed

	maxBase64FileSize          = uploadhandler.MaxBase64FileSize
	defaultEscalationTimeoutMs = uploadhandler.DefaultEscalationTimeoutMs
)

// ============================================
// Function Aliases
// ============================================

var (
	getProgressTier = uploadhandler.GetProgressTier
	detectMimeType  = uploadhandler.DetectMimeType
)
