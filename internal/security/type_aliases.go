// Purpose: Provides non-stuttering type aliases for the security package.
// Why: Improves call-site readability while preserving backward compatibility for existing API names.

package security

import "github.com/brennhill/Kaboom-Browser-AI-Devtools-MCP/internal/capture"

// NetworkBody re-exports capture.NetworkBody so test files and helpers can use
// the unqualified name.
type NetworkBody = capture.NetworkBody

type (
	Finding    = SecurityFinding
	ScanInput  = SecurityScanInput
	ScanResult = SecurityScanResult
	Scanner    = SecurityScanner
)
