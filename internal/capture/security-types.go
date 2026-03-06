// Purpose: Defines capture-side security flag payloads derived from network analysis.
// Why: Provides a stable threat-flag structure shared with downstream security tooling.
// Docs: docs/features/feature/security-hardening/index.md

package capture

import (
	"time"
)

// SecurityFlag represents a detected security issue detected from network waterfall analysis.
type SecurityFlag struct {
	Type      string    `json:"type"`      // "suspicious_tld", "non_standard_port", etc.
	Severity  string    `json:"severity"`  // "low", "medium", "high", "critical"
	Origin    string    `json:"origin"`    // The flagged origin
	Message   string    `json:"message"`   // Human-readable explanation
	Resource  string    `json:"resource"`  // Specific resource URL (optional)
	PageURL   string    `json:"page_url"`  // Page that loaded this resource
	Timestamp time.Time `json:"timestamp"` // When flagged
}
