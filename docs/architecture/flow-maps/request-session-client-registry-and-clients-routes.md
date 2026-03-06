---
doc_type: flow_map
flow_id: request-session-client-registry-and-clients-routes
status: active
last_reviewed: 2026-03-05
owners:
  - Brenn
entrypoints:
  - cmd/dev-console/main_connection_mcp_bootstrap.go:initCapture
  - cmd/dev-console/server_routes_clients.go:handleClientsList
  - cmd/dev-console/server_routes_clients.go:handleClientByID
code_paths:
  - cmd/dev-console/client_registry_adapter.go
  - cmd/dev-console/main_connection_mcp_bootstrap.go
  - cmd/dev-console/server_routes_clients.go
  - internal/capture/interfaces.go
  - internal/capture/client_registry_setter.go
  - internal/session/client_registry.go
test_paths:
  - cmd/dev-console/server_routes_clients_test.go
  - internal/session/client_registry_test.go
last_verified_version: 0.7.12
last_verified_date: 2026-03-05
---

# Request Session Client Registry and `/clients` Routes

## Scope

Covers startup wiring of the concrete session registry into capture and the HTTP `/clients` route behavior for listing, registering, reading, and deleting connected MCP clients.

## Entrypoints

1. `initCapture(...)` injects the registry adapter at daemon bootstrap.
2. `handleClientsList(...)` serves `GET /clients` and `POST /clients`.
3. `handleClientByID(...)` serves `GET /clients/{id}` and `DELETE /clients/{id}`.

## Primary Flow

1. Daemon startup creates `session.NewClientRegistry()`.
2. `newSessionClientRegistryAdapter(...)` bridges concrete session types to capture’s interface boundary without introducing package cycles.
3. `cap.SetClientRegistry(...)` stores the adapter for route-time lookup.
4. Route handlers call `resolveClientRegistry(...)`; if unavailable, they return `503 client_registry_unavailable`.
5. `POST /clients` decodes JSON `{cwd}`, registers the client, and returns the created client state.
6. `GET /clients` returns current client list and count.
7. `GET /clients/{id}` returns the client state or `404`.
8. `DELETE /clients/{id}` unregisters and returns `{"unregistered": true}` or `404` when missing.

## Error and Recovery Paths

1. Missing registry injection returns `503` with a structured error payload.
2. Invalid request body JSON returns `400 Invalid JSON`.
3. Unknown client IDs on read/delete return `404 Client not found`.

## State and Contracts

1. `internal/capture/interfaces.go` defines the `ClientRegistry` abstraction boundary.
2. `internal/session/client_registry.go` remains the concrete lock-owning registry implementation.
3. Adapter methods keep concrete result values (`*session.ClientState`, `[]session.ClientInfo`) while satisfying the capture interface return shape.

## Code Paths

- `cmd/dev-console/client_registry_adapter.go`
- `cmd/dev-console/main_connection_mcp_bootstrap.go`
- `cmd/dev-console/server_routes_clients.go`
- `internal/capture/interfaces.go`
- `internal/capture/client_registry_setter.go`
- `internal/session/client_registry.go`

## Test Paths

- `cmd/dev-console/server_routes_clients_test.go`
- `internal/session/client_registry_test.go`

## Edit Guardrails

1. Keep `session` -> `capture` dependency direction; do not import `session` from `internal/capture`.
2. Preserve explicit `404` behavior for missing `DELETE /clients/{id}` to avoid false-positive success responses.
3. Keep registry availability checks centralized in `resolveClientRegistry(...)` to avoid divergent route error behavior.
