// Purpose: Concurrency tests for pagination and cursor without race detector overhead.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/pagination/index.md

//go:build !race

package pagination

const raceDetectorEnabled = false
