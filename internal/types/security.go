// Purpose: Owns security.go runtime behavior and integration logic.
// Docs: docs/features/feature/observe/index.md

// security.go — Security and threat detection types.
// Contains types for flagging suspicious network activity and security issues.
// Zero dependencies - foundational types used by capture and security packages.
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
