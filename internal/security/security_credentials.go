// Purpose: Detects credential and secret exposure patterns across URLs, payloads, and console output.
// Why: Reduces high-impact key/token leakage risk by flagging dangerous exposure paths early.
// Docs: docs/features/feature/redaction-patterns/index.md
// Docs: docs/features/feature/security-hardening/index.md

// Layout:
// - security_credentials_patterns.go: credential regex catalog and scan rule metadata
// - security_credentials_scan.go: URL/body/console credential scanning pipeline
// - security_credentials_helpers.go: secret masking and log-entry helpers
// - security_url_helpers.go: shared URL/content-type helper predicates
package security
