// Purpose: Re-exports cross-platform upload security types from the uploadhandler sub-package.
// Why: Surfaces upload security validation as package-level aliases for use by interact upload handlers.
// Docs: docs/features/feature/file-upload/index.md

package main

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/cmd/browser-agent/internal/uploadhandler"

// ============================================
// Type Aliases
// ============================================

type UploadSecurity = uploadhandler.Security
type PathValidationResult = uploadhandler.PathValidationResult
type PathDeniedError = uploadhandler.PathDeniedError
type UploadDirRequiredError = uploadhandler.UploadDirRequiredError

// ============================================
// Function and Variable Aliases
// ============================================

// ValidateUploadDir validates the --upload-dir flag at startup.
var ValidateUploadDir = uploadhandler.ValidateUploadDir

// Unexported aliases used by remaining integration tests and HTTP handlers.
var (
	matchesDenylist     = uploadhandler.MatchesDenylist
	matchesUserDenylist = uploadhandler.MatchesUserDenylist
	isWithinDir         = uploadhandler.IsWithinDir
	pathsEqualFold      = uploadhandler.PathsEqualFold
	pathHasPrefixFold   = uploadhandler.PathHasPrefixFold
)
