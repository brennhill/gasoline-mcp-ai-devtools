// Purpose: Provides non-stuttering type aliases for the redaction package.
// Why: Improves call-site readability while preserving backward compatibility for existing API names.

package redaction

type (
	Pattern = RedactionPattern
	Config  = RedactionConfig
	Engine  = RedactionEngine
)

func NewEngine(configPath string) *Engine { return NewRedactionEngine(configPath) }
