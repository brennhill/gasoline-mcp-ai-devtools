// Purpose: Validate race_disabled_test.go behavior and guard against regressions.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/observe/index.md

//go:build !race

package buffers

const raceDetectorEnabled = false
