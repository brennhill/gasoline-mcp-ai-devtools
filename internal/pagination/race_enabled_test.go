// Purpose: Race-detector-enabled concurrency tests for pagination and cursor.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/pagination/index.md

//go:build race

package pagination

const raceDetectorEnabled = true
