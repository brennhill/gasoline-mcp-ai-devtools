// Purpose: Package testgen — test generation, failure classification, and selector healing.
// Why: Accelerates regression coverage by turning observed failures into repeatable tests.
// Docs: docs/features/feature/test-generation/index.md

/*
Package testgen provides test generation from captured browser state, test failure
classification, and broken selector healing.

Key types:
  - GeneratedTest: output of test generation with framework, content, selectors, and coverage metadata.
  - FailureClassification: categorized test failure with confidence, evidence, and suggested fix.
  - HealResult: selector healing output with healed/unhealed counts and auto-apply tracking.
  - DataProvider: interface abstracting access to captured logs, actions, and network bodies.

Key functions:
  - GenerateTestFromError: generates a test reproducing a specific console error.
  - GenerateTestFromInteraction: generates a test from recorded user interactions.
  - ClassifyFailure: categorizes a test failure (selector_broken, timing_flaky, real_bug, etc.).
  - RepairSelectors: attempts to heal broken CSS selectors with confidence scoring.
  - HealTestBatch: heals selectors across multiple test files in a directory.
*/
package testgen
