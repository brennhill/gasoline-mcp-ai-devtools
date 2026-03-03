// Purpose: Tests for shared types correctness.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/normalized-event-schema/index.md

//go:build race

package types

const raceEnabled = true
