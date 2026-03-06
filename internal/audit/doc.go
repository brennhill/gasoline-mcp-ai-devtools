// Purpose: Package audit — bounded concurrent audit trail with parameter redaction.
// Why: Provides compliance-grade traceability for tool calls while minimizing sensitive-data exposure.
// Docs: docs/features/feature/enterprise-audit/index.md

/*
Package audit implements a bounded, concurrent audit trail for MCP tool invocations.

Key types:
  - AuditEntry: a single tool invocation record with timestamp, client ID, parameters, and duration.
  - AuditTrail: thread-safe, capacity-bounded store with automatic parameter redaction.

Key functions:
  - NewAuditTrail: creates an audit trail with configurable max entries.
  - Record: appends an audit entry, redacting sensitive parameter values.
  - Query: retrieves entries filtered by tool name, session ID, or time range.
*/
package audit
