// Purpose: Defines shared security-flag payload structures produced by security analysis routines.
// Why: Keeps security findings transportable across scanners, auditors, and response formatters.
// Docs: docs/features/feature/security-hardening/index.md

package types

import "time"

// ============================================
// Security Threat Flagging
// ============================================

// SecurityFlag represents a detected security issue detected from network waterfall analysis.
// Flags suspicious patterns like suspicious TLDs, non-standard ports, etc.
type SecurityFlag struct {
	Type      string    `json:"type"`      // "suspicious_tld", "non_standard_port", etc.
	Severity  string    `json:"severity"`  // "low", "medium", "high", "critical"
	Origin    string    `json:"origin"`    // The flagged origin
	Message   string    `json:"message"`   // Human-readable explanation
	Resource  string    `json:"resource"`  // Specific resource URL (optional)
	PageURL   string    `json:"page_url"`  // Page that loaded this resource
	Timestamp time.Time `json:"timestamp"` // When flagged
}
