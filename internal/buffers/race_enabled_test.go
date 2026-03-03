// Purpose: Race-detector-enabled concurrency tests for ring buffer.
// Why: Prevents silent regressions in critical behavior paths.
// Docs: docs/features/feature/ring-buffer/index.md

//go:build race

package buffers

const raceDetectorEnabled = true
