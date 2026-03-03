---
doc_type: tech-spec
feature_id: feature-playback-engine
status: proposed
last_reviewed: 2026-03-03
---

# Playback Engine Tech Spec

## Architecture
- Core types and orchestration: `internal/recording/playback.go`, `internal/recording/playback_engine_types.go`
- Action execution helpers: `internal/recording/playback_engine_actions.go`
- Session/runtime management: `internal/recording/playback_engine_session.go`
- CLI handler bridge: `cmd/dev-console/recording_handlers_playback.go`

## Constraints
- Playback execution must remain bounded and interruptible.
- Error policy (`continue`, `stop`, dependency skip) must be explicit.
- Replay output should include enough per-step evidence for debugging.
