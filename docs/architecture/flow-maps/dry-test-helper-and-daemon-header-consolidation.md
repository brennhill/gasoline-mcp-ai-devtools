---
doc_type: flow_map
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
feature_ids:
  - feature-pagination
  - feature-security-hardening
  - feature-backend-log-streaming
  - feature-observe
  - feature-terminal
  - feature-browser-extension-enhancement
---

# DRY Test Helpers and Daemon Header Consolidation

## Scope

Consolidate repeated test setup/assertion flows and enforce one daemon-header contract on the extension options surface.

## Entrypoints

- Pagination cursor tests:
  - `internal/pagination/pagination_actions_test.go`
  - `internal/pagination/pagination_websocket_test.go`
- Security diff tests:
  - `internal/security/security_diff_test.go`
- Capture sync tests:
  - `internal/capture/sync_test.go`
- Observe storage tests:
  - `internal/tools/observe/storage_test.go`
- PTY session tests:
  - `internal/pty/session_test.go`
- Extension options daemon requests:
  - `src/options.ts`

## Primary Flow

1. Pagination suites call shared cursor-case runners in `internal/pagination/test_helpers_test.go`.
2. Security diff cases call shared snapshot helpers (`mustTakeSnapshot`, `mustCompareSnapshots`) before assertions.
3. Sync tests call shared transport helpers (`runSyncRequest`, `runSyncRawRequest`, `decodeSyncResponse`) from `internal/capture/sync_test_helpers_test.go`.
4. Observe storage tests call shared summary assertions (`assertSummaryShape`, `assertSummaryTotalBytes`).
5. PTY tests reuse `readUntilContains` for bounded read loops and timeout handling.
6. `src/options.ts` delegates daemon request headers/body init to `src/lib/daemon-http.ts`.

## Error and Recovery Paths

- Helper wrappers fail fast with contextual test messages to avoid silent setup drift.
- PTY read helper centralizes timeout and partial-output diagnostics.
- Options daemon requests stay best-effort where already intended (`syncDevRootToDaemon`, active codebase preload).

## State and Contracts

- Daemon client identity header is normalized via:
  - `buildDaemonHeaders`
  - `buildDaemonJSONRequestInit`
- Cursor metadata assertions are normalized through shared pagination helper calls.
- Security snapshot compare naming contract (`before`/`after`) is normalized in a single helper.

## Code Paths

- `internal/pagination/test_helpers_test.go`
- `internal/pagination/pagination_actions_test.go`
- `internal/pagination/pagination_websocket_test.go`
- `internal/pagination/pagination_test.go`
- `internal/security/security_diff_test.go`
- `internal/capture/sync_test_helpers_test.go`
- `internal/capture/sync_test.go`
- `internal/capture/settings_path_test.go`
- `internal/capture/coverage_gaps_part2_test.go`
- `internal/capture/api_contract_test.go`
- `internal/tools/observe/storage_test.go`
- `internal/pty/session_test.go`
- `internal/session/network_diff_test.go`
- `src/options.ts`
- `src/lib/daemon-http.ts`

## Test Paths

- `internal/pagination/pagination_actions_test.go`
- `internal/pagination/pagination_websocket_test.go`
- `internal/pagination/pagination_test.go`
- `internal/security/security_diff_test.go`
- `internal/capture/sync_test.go`
- `internal/capture/settings_path_test.go`
- `internal/capture/coverage_gaps_part2_test.go`
- `internal/capture/api_contract_test.go`
- `internal/tools/observe/storage_test.go`
- `internal/pty/session_test.go`
- `internal/session/network_diff_test.go`

## Edit Guardrails

- Extend shared helpers before adding new one-off setup blocks in related suites.
- Keep options-page daemon requests on shared header/request builders; do not reintroduce inline header literals.
- Preserve current behavior and assertions; this map only covers structure/DRY unification.

## Feature Links

- Pagination: `docs/features/feature/pagination/flow-map.md`
- Security Hardening: `docs/features/feature/security-hardening/flow-map.md`
- Backend Log Streaming: `docs/features/feature/backend-log-streaming/flow-map.md`
- Observe: `docs/features/feature/observe/flow-map.md`
- Terminal: `docs/features/feature/terminal/flow-map.md`
- Browser Extension Enhancement: `docs/features/feature/browser-extension-enhancement/flow-map.md`
