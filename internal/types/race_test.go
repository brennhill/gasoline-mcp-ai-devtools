// Purpose: Validate race_test.go behavior and guard against regressions.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/observe/index.md

//go:build race

package types

const raceEnabled = true
