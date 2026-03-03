// Purpose: Generates cryptographically random audit entry and session IDs.
// Why: Centralizes ID generation so audit trail and session creation share a single scheme.
package audit

import (
	"crypto/rand"
	"encoding/hex"
)

// generateAuditID creates a unique audit entry ID (16 hex chars).
func generateAuditID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b) // #nosec G104 -- best-effort randomness for non-security audit ID
	return hex.EncodeToString(b)
}

// generateAuditSessionID creates a unique session ID (32 hex chars from 16 random bytes).
func generateAuditSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b) // #nosec G104 -- best-effort randomness for non-security session ID
	return hex.EncodeToString(b)
}
