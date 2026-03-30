// security.go — Re-exports cross-platform upload security types from internal/upload.
// Why: Surfaces upload security validation as package-level aliases for use by upload HTTP handlers.
// Docs: docs/features/feature/file-upload/index.md

package uploadhandler

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/upload"

// ============================================
// Type Aliases
// ============================================

type Security = upload.Security
type PathValidationResult = upload.PathValidationResult
type PathDeniedError = upload.PathDeniedError
type UploadDirRequiredError = upload.UploadDirRequiredError

// ============================================
// Function and Variable Aliases
// ============================================

// ValidateUploadDir validates the --upload-dir flag at startup.
var ValidateUploadDir = upload.ValidateUploadDir

var (
	MatchesDenylist     = upload.MatchesDenylist
	MatchesUserDenylist = upload.MatchesUserDenylist
	IsWithinDir         = upload.IsWithinDir
	PathsEqualFold      = upload.PathsEqualFold
	PathHasPrefixFold   = upload.PathHasPrefixFold
)

// IsPrivateIP re-exports SSRF-safe IP validation.
var IsPrivateIP = upload.IsPrivateIP

// NewSecurity creates a new upload security configuration.
var NewSecurity = upload.NewSecurity

// SetSkipSSRFCheck enables/disables SSRF check bypass (for testing only).
var SetSkipSSRFCheck = upload.SetSkipSSRFCheck

// SetSSRFAllowedHosts configures the SSRF allowed-hosts list.
var SetSSRFAllowedHosts = upload.SetSSRFAllowedHosts

// NewSSRFSafeTransport creates an HTTP transport with SSRF protection.
var NewSSRFSafeTransport = upload.NewSSRFSafeTransport
