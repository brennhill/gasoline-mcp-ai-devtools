// Purpose: Re-exports cross-platform upload security types (UploadSecurity, PathValidationResult, PathDeniedError) from internal/upload.
// Why: Surfaces upload security validation as package-level aliases for use by interact upload handlers.
// Docs: docs/features/feature/file-upload/index.md

package main

import "github.com/brennhill/gasoline-agentic-browser-devtools-mcp/internal/upload"

// ============================================
// Type Aliases
// ============================================

type UploadSecurity = upload.Security
type PathValidationResult = upload.PathValidationResult
type PathDeniedError = upload.PathDeniedError
type UploadDirRequiredError = upload.UploadDirRequiredError

// ============================================
// Function and Variable Aliases
// ============================================

// ValidateUploadDir validates the --upload-dir flag at startup.
var ValidateUploadDir = upload.ValidateUploadDir

// Unexported aliases used by remaining integration tests and HTTP handlers.
var (
	matchesDenylist     = upload.MatchesDenylist
	matchesUserDenylist = upload.MatchesUserDenylist
	isWithinDir         = upload.IsWithinDir
	pathsEqualFold      = upload.PathsEqualFold
	pathHasPrefixFold   = upload.PathHasPrefixFold
)
