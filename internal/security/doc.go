// Package security provides security analysis and policy generation for web applications.
//
// Features:
//   - Content-Security-Policy (CSP) generation from observed network traffic
//   - Subresource Integrity (SRI) hash generation for external scripts
//   - Third-party domain auditing and classification
//   - Security vulnerability flagging (credentials, PII, insecure transport)
//   - Security audit trail with severity levels and remediation hints
//   - Snapshot-based security diffing (before/after comparisons)
//
// The package operates in two modes:
//   - Interactive mode: Can prompt user for configuration changes
//   - MCP mode: Blocked from making config changes, returns errors
//
// All security operations maintain an in-memory audit log accessible via
// GetSecurityAuditLog(). Future versions will persist to disk.
package security
