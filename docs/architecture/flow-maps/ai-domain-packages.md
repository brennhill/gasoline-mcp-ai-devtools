---
doc_type: flow_map
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
---

# Checkpoint, Noise, and Persistence Split

## Scope

Refactor the former `internal/ai` mixed domain into focused packages:

- `internal/checkpoint` for checkpoint diffing and alert delivery state
- `internal/noise` for rule matching and auto-detection
- `internal/persistence` for session store I/O and validation
- `internal/ai` retained only as a compatibility facade

## Entrypoints

1. `cmd/dev-console/tools_core_constructor.go:NewToolHandler`
2. `cmd/dev-console/tools_configure_noise_actions.go`
3. `cmd/dev-console/tools_configure_state_impl.go`
4. `cmd/dev-console/tools_analyze_visual.go`

## Primary Flow

1. Startup initializes `persistence.NewSessionStore(...)` when project path is available.
2. Startup wires `noise.NewNoiseConfigWithStore(store)` (or `noise.NewNoiseConfig()` fallback).
3. `configure(what='noise_rule')` actions read/write noise rules via `internal/noise`.
4. `configure(what='store'|'load')` actions use `internal/persistence` session store APIs.
5. Checkpoint diffing and alert lifecycle execute through `internal/checkpoint` types/managers.
6. Legacy `internal/ai` imports are served by alias wrappers for backward compatibility.

## Error and Recovery Paths

1. Session store init failure: handler continues with nil store and in-memory-only noise config.
2. Corrupt persisted noise payload: `internal/noise` logs warning and continues with built-ins.
3. Missing store for configure store/load: tool returns structured `not_initialized`.
4. Missing checkpoint name/timestamp mismatch: checkpoint resolver falls back to current snapshot.

## State and Contracts

1. Noise rule cap remains `maxNoiseRules = 100` in `internal/noise`.
2. Session storage namespaces/keys remain validated by `internal/persistence`.
3. Checkpoint response schema (`DiffResponse`) remains unchanged via package aliases.
4. `internal/ai` facade must not add new behavior; it only forwards to focused packages.

## Code Paths

- `internal/checkpoint/*.go`
- `internal/noise/*.go`
- `internal/persistence/*.go`
- `internal/ai/aliases.go`
- `cmd/dev-console/tools_core_constructor.go`
- `cmd/dev-console/tools_core.go`
- `cmd/dev-console/tools_configure_noise_actions.go`
- `cmd/dev-console/tools_configure_state_impl.go`
- `cmd/dev-console/tools_analyze_visual.go`
- `cmd/dev-console/noise_autorun.go`

## Test Paths

- `internal/checkpoint/*_test.go`
- `internal/noise/*_test.go`
- `internal/persistence/*_test.go`
- `cmd/dev-console/tools_configure_noise_actions_test.go`
- `cmd/dev-console/tools_configure_sequence_test.go`

## Edit Guardrails

1. New checkpoint logic goes under `internal/checkpoint`, not `internal/ai`.
2. New noise matching/proposal logic goes under `internal/noise`.
3. New session-store features go under `internal/persistence`.
4. Keep `internal/ai` as compatibility-only until all downstream imports are removed.
5. If API shape changes, update aliases and run quick-go gate before merge.
