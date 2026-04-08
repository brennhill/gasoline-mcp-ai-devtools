// toolconst.go — Shared constants across tool sub-packages.
// Purpose: Single source of truth for constants used by multiple tool packages.
// Why: Eliminates duplicated constants between toolconfigure and toolinteract.

package toolconst

const (
	// MaxSequenceSteps is the maximum number of steps in a batch or sequence.
	MaxSequenceSteps = 50

	// DefaultStepTimeout is the default per-step timeout in milliseconds.
	DefaultStepTimeout = 10_000
)
