// types.go — Re-exports upload request/response wire types from internal/upload.
// Why: Keeps wire type definitions in internal/upload while making them available as short aliases.
// Docs: docs/features/feature/file-upload/index.md

package uploadhandler

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"

// ============================================
// Type Aliases
// ============================================

type FileReadRequest = upload.FileReadRequest
type FileReadResponse = upload.FileReadResponse
type FileDialogInjectRequest = upload.FileDialogInjectRequest
type FormSubmitRequest = upload.FormSubmitRequest
type OSAutomationInjectRequest = upload.OSAutomationInjectRequest
type StageResponse = upload.StageResponse
type ProgressTier = upload.ProgressTier

// ============================================
// Constants
// ============================================

const (
	ProgressTierSimple   = upload.ProgressTierSimple
	ProgressTierPeriodic = upload.ProgressTierPeriodic
	ProgressTierDetailed = upload.ProgressTierDetailed

	MaxBase64FileSize          = upload.MaxBase64FileSize
	DefaultEscalationTimeoutMs = upload.DefaultEscalationTimeoutMs
)

// ============================================
// Function Aliases
// ============================================

var (
	GetProgressTier = upload.GetProgressTier
	DetectMimeType  = upload.DetectMimeType
)
