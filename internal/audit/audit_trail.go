// Purpose: Implements bounded concurrent audit-trail storage with parameter redaction and session metadata.
// Why: Provides compliance-grade traceability for tool calls while minimizing sensitive-data exposure.
// Docs: docs/features/feature/enterprise-audit/index.md

// Layout:
// - audit_types.go: model types and shared constants
// - audit_recording.go: constructor and append/record paths
// - audit_query.go: querying, clear, and MCP handler response assembly
// - audit_session.go: client normalization and session lifecycle
// - audit_redaction.go: parameter redaction rules
// - audit_ids.go: audit/session identifier generation
package audit
