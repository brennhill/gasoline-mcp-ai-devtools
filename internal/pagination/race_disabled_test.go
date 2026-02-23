// Purpose: Validate race_disabled_test.go behavior and guard against regressions.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/pagination/index.md

//go:build !race

package pagination

const raceDetectorEnabled = false
