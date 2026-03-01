// Purpose: Re-exports upload request/response wire types (FileReadRequest, FileDialogInjectRequest, etc.) from internal/upload.
// Why: Keeps wire type definitions in internal/upload while making them available as short aliases in cmd.
// Docs: docs/features/feature/file-upload/index.md

package main

import "github.com/dev-console/dev-console/internal/upload"

// ============================================
// Type Aliases
// ============================================

type FileReadRequest = upload.FileReadRequest
type FileReadResponse = upload.FileReadResponse
type FileDialogInjectRequest = upload.FileDialogInjectRequest
type FormSubmitRequest = upload.FormSubmitRequest
type OSAutomationInjectRequest = upload.OSAutomationInjectRequest
type UploadStageResponse = upload.StageResponse
type ProgressTier = upload.ProgressTier

// ============================================
// Constants
// ============================================

const (
	ProgressTierSimple   = upload.ProgressTierSimple
	ProgressTierPeriodic = upload.ProgressTierPeriodic
	ProgressTierDetailed = upload.ProgressTierDetailed

	maxBase64FileSize          = upload.MaxBase64FileSize
	defaultEscalationTimeoutMs = upload.DefaultEscalationTimeoutMs
)

// ============================================
// Function Aliases
// ============================================

var (
	getProgressTier = upload.GetProgressTier
	detectMimeType  = upload.DetectMimeType
)
