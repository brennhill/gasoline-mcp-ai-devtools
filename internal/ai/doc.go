// Purpose: Package ai — checkpoint-based diffing, noise filtering, and persistence for AI-facing telemetry.
// Why: Provides incremental change summaries and noise-reduced signals so AI agents can debug efficiently.
// Docs: docs/features/feature/push-alerts/index.md

/*
Package ai provides checkpoint-based state diffing, noise filtering, and session
persistence for AI-facing browser telemetry.

Key types:
  - CheckpointManager: captures named checkpoints and computes diffs across console, network, WebSocket, and action buffers.
  - NoiseRuleSet: compiles and applies regex-based noise rules to suppress irrelevant log entries.
  - PersistenceManager: saves, loads, and lists AI session context to disk.

Key functions:
  - NewCheckpointManager: creates a checkpoint manager backed by a CaptureStateReader.
  - AutoDetectNoise: proposes noise rules from captured log patterns with confidence scoring.
*/
package ai
