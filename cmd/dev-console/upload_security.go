// upload_security.go — Security type aliases and function delegates to internal/upload.
package main

import "github.com/dev-console/dev-console/internal/upload"

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
