// Purpose: Provides diff verdict classification and pixel-level math utilities for image comparison.
// Why: Separates numerical operations from grid construction, region detection, and rendering.
package analyze

func DiffVerdict(pct float64) string {
	switch {
	case pct == 0:
		return "identical"
	case pct < 5:
		return "minor_changes"
	case pct < 25:
		return "major_changes"
	default:
		return "completely_different"
	}
}

func absDiff16(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

