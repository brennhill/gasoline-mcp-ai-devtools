---
doc_type: flow_map
flow_id: shared-extraction-and-contract-normalization
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - src/background/commands/analyze.ts:resolveAnalyzeFrameSelection
  - src/background/dom-dispatch.ts:resolveExecutionTarget
  - src/background/commands/interact-content.ts:contentExtractorCommand
  - src/background/context-menus.ts:installContextMenus
  - src/background/keyboard-shortcuts.ts:installDrawModeCommandListener
  - internal/session/verify_actions.go:VerificationManager.Watch
  - internal/tools/configure/rewrite.go:RewriteNoiseRuleArgs
code_paths:
  - src/background/frame-targeting.ts
  - src/background/dom-frame-probe.ts
  - src/background/commands/analyze.ts
  - src/background/dom-dispatch.ts
  - src/background/content-fallback-scripts.ts
  - src/background/commands/interact-content.ts
  - src/background/commands/interact-explore.ts
  - src/background/draw-mode-toggle.ts
  - src/background/context-menus.ts
  - src/background/keyboard-shortcuts.ts
  - src/background/recording-utils.ts
  - src/background/recording-listeners.ts
  - src/inject/observers.ts
  - src/lib/network.ts
  - internal/types/snapshot.go
  - internal/session/types.go
  - internal/session/verify_actions.go
  - internal/mcp/response_json.go
  - internal/tools/configure/boundaries.go
  - internal/tools/configure/rewrite.go
  - internal/tools/observe/handlers_extension_logs.go
  - internal/schema/generate.go
test_paths:
  - extension/background/__tests__/dom-dispatch-structured.test.js
  - tests/extension/interact-content-fallback.test.js
  - internal/session/verify_test.go
  - internal/mcp/response_test.go
  - internal/tools/configure/boundaries_test.go
  - internal/tools/configure/rewrite_test.go
  - internal/tools/observe/analysis_test.go
  - internal/schema/interact_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Shared Extraction and Contract Normalization

## Scope

Covers shared helper extraction and contract normalization refactors applied across interact/analyze/observe/session/configure/generate code paths to reduce drift and duplicated logic.

## Entrypoints

1. Frame-targeted analyze/interact commands now route through shared frame normalization/probing helpers.
2. Content extraction fallbacks are centralized and reused by both `interact-content` and `explore_page`.
3. Draw-mode toggle behavior is centralized and reused by keyboard shortcut and context-menu entrypoints.
4. Session verification actions reuse shared active-session validation.
5. Configure rewrite/boundary parsers reuse shared argument parsing helpers.

## Primary Flow

1. Normalize input once in a shared helper (`frame-targeting`, configure parsers, response builders).
2. Reuse a single utility for execution-path-specific behavior (frame ID resolution, draw-mode toggling, recording slug generation, extension log shaping).
3. Keep caller paths thin and focused on entrypoint-specific concerns (toast/log wording, command registration, response routing).
4. Preserve existing wire contracts while removing internal duplicate branches.

## Error and Recovery Paths

1. Shared frame helpers throw stable `invalid_frame` and `frame_not_found` errors for all consumers.
2. Draw-mode helper falls back to start when state query fails, then propagates unreachable errors to caller-specific UX.
3. Configure parsing helpers return consistent structured errors for invalid JSON and missing required params.
4. Response JSON helpers consistently use safe marshal fallbacks for both success and error payloads.

## State and Contracts

1. Snapshot schema is canonicalized in `internal/types/snapshot.go`; session package uses aliases to avoid contract drift.
2. Shared fallback extractors preserve existing response fields and `fallback: true` flag.
3. Header sanitization in fetch error/success paths now uses shared `sanitizeHeaders`.
4. Failure schema fragments in generate schema are sourced from one helper to avoid divergent field definitions.

## Code Paths

- `src/background/frame-targeting.ts`
- `src/background/dom-frame-probe.ts`
- `src/background/commands/analyze.ts`
- `src/background/dom-dispatch.ts`
- `src/background/content-fallback-scripts.ts`
- `src/background/commands/interact-content.ts`
- `src/background/commands/interact-explore.ts`
- `src/background/draw-mode-toggle.ts`
- `src/background/context-menus.ts`
- `src/background/keyboard-shortcuts.ts`
- `src/background/recording-utils.ts`
- `src/background/recording-listeners.ts`
- `src/inject/observers.ts`
- `internal/types/snapshot.go`
- `internal/session/types.go`
- `internal/session/verify_actions.go`
- `internal/mcp/response_json.go`
- `internal/tools/configure/boundaries.go`
- `internal/tools/configure/rewrite.go`
- `internal/tools/observe/handlers_extension_logs.go`
- `internal/schema/generate.go`

## Test Paths

- `extension/background/__tests__/dom-dispatch-structured.test.js`
- `tests/extension/interact-content-fallback.test.js`
- `internal/session/verify_test.go`
- `internal/mcp/response_test.go`
- `internal/tools/configure/boundaries_test.go`
- `internal/tools/configure/rewrite_test.go`
- `internal/tools/observe/analysis_test.go`
- `internal/schema/interact_test.go`

## Edit Guardrails

1. Add new frame-targeted actions by reusing `normalizeFrameArg` and `resolveMatchedFrameIds`.
2. Keep executeScript fallback functions self-contained; add new fallbacks in `content-fallback-scripts.ts` first.
3. Do not reintroduce per-caller draw-mode or recording-slug logic.
4. Keep snapshot wire fields canonical in `internal/types`; use aliases in feature packages.
5. Keep feature docs in sync:
   - `docs/features/feature/interact-explore/flow-map.md`
   - `docs/features/feature/analyze-tool/flow-map.md`
   - `docs/features/feature/flow-recording/flow-map.md`
   - `docs/features/feature/observe/flow-map.md`
   - `docs/features/feature/request-session-correlation/flow-map.md`
   - `docs/features/feature/historical-snapshots/flow-map.md`
   - `docs/features/feature/config-profiles/flow-map.md`
   - `docs/features/feature/test-generation/flow-map.md`
   - `docs/features/feature/query-service/flow-map.md`
