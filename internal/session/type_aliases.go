// Purpose: Provides non-stuttering type aliases for the session package.
// Why: Improves call-site readability while preserving backward compatibility for existing API names.

package session

type (
	Manager       = SessionManager
	DiffResult    = SessionDiffResult
	NetworkDiff   = SessionNetworkDiff
	NetworkChange = SessionNetworkChange
)
