// Purpose: Provides non-stuttering type aliases for the recording package.
// Why: Improves call-site readability while preserving backward compatibility for existing API names.

package recording

type (
	Action   = RecordingAction
	Item     = Recording
	Metadata = RecordingMetadata
	Manager  = RecordingManager
)
