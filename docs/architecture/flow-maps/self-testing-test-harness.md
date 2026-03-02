---
doc_type: flow_map
flow_id: self-testing-test-harness
status: active
last_reviewed: 2026-03-02
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/server_routes.go:registerRoutes
  - cmd/dev-console/testpages_http.go:handleTestPages
  - cmd/dev-console/testpages_websocket.go:handleTestHarnessWS
code_paths:
  - cmd/dev-console/server_routes.go
  - cmd/dev-console/testpages_http.go
  - cmd/dev-console/testpages_websocket.go
test_paths:
  - cmd/dev-console/testpages_test.go
---

# Self-Testing Test Harness

## Scope

Covers deterministic `/tests/*` HTTP fixtures and the `/tests/ws` WebSocket echo harness used by smoke/regression checks.

## Entrypoints

- `registerRoutes` wires `/tests/` and `/tests/ws`.
- `handleTestPages` serves static embedded pages and deterministic fixture endpoints.
- `handleTestHarnessWS` performs WebSocket upgrade + echo loop.

## Primary Flow

1. `registerRoutes` binds `handleTestPages()` and `handleTestHarnessWS`.
2. `handleTestPages` serves embedded page assets from `testpages/`.
3. Special fixture endpoints return deterministic responses:
4. `/tests/404` returns 404 JSON.
5. `/tests/500` returns 500 JSON.
6. `/tests/cors-test` returns 200 JSON with no CORS headers.
7. `/tests/slow` sleeps 3 seconds then returns 200 JSON.
8. `handleTestHarnessWS` validates upgrade headers, hijacks the connection, and emits the RFC 6455 handshake.
9. `wsEchoLoop` processes control/data frames and returns text/binary echoes.

## Error and Recovery Paths

- Non-GET requests to `/tests/*` return 405 JSON.
- Missing upgrade headers on `/tests/ws` return 400 JSON.
- Non-hijack-capable writers return 500 JSON.
- Oversized WebSocket payloads are rejected by `wsReadFrame` before allocation.

## State and Contracts

- `maxWSPayload` bounds frame payload size to 1 MiB.
- `wsIdleTimeout` enforces idle timeout behavior on active connections.
- Embedded assets are sourced from `//go:embed testpages`.

## Code Paths

- `cmd/dev-console/testpages_http.go`
- `cmd/dev-console/testpages_websocket.go`
- `cmd/dev-console/server_routes.go`

## Test Paths

- `cmd/dev-console/testpages_test.go`

## Edit Guardrails

- Keep HTTP fixture routing in `testpages_http.go`; avoid mixing protocol logic back in.
- Keep WebSocket frame parsing and echo behavior in `testpages_websocket.go`.
- Any endpoint behavior or frame contract changes must update this map and `testpages_test.go`.
