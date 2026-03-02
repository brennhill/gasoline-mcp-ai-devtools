// Purpose: Provides non-stuttering type aliases for the audit package.
// Why: Improves call-site readability while preserving backward compatibility for existing API names.

package audit

type (
	Entry          = AuditEntry
	Trail          = AuditTrail
	SessionInfo    = AuditSessionInfo
	Filter         = AuditFilter
	Config         = AuditConfig
	RedactionEntry = RedactionEvent
)

func NewTrail(config Config) *Trail { return NewAuditTrail(config) }
