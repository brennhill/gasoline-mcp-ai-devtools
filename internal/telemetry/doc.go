// Package telemetry implements the Kaboom anonymous-telemetry pipeline:
// install ID derivation + persistence, session lifecycle, beacon
// formatting, and ingest-side wire contract enforcement. The full
// surface map lives in docs/core/app-metrics.md.
//
// CONSTRAINT — no t.Parallel() in package telemetry tests
//
// Several files own package-level globals that the production
// pipeline mutates singly (kaboomDir, installIDLoadInFlight,
// cachedInstallIDPtr, secondaryDirOverride, userHomeDirFn,
// session, endpoint). Tests gate these via the withXxxState helpers
// in helpers_test.go (lockBudgetMu, homeDirFnMu, secondaryDirStateMu);
// the helpers serialize ROTATION but assume tests run sequentially.
//
// A test that called t.Parallel() would run its t.Cleanup
// concurrently with another test's body, racing the global-state
// restore against the global-state mutation. The race detector would
// catch it, but the test would already be wrong.
//
// New tests in this package must NOT call t.Parallel().
//
// Subtests inside a single test (t.Run) are fine. The constraint is
// solely about parallel execution between sibling top-level tests.
//
// If a future requirement makes parallelism necessary (e.g., a per-
// install_id fixture isolation), introduce an installIDStore
// interface so each test can own its instance instead of mutating
// package globals.
package telemetry
