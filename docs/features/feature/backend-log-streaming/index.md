---
doc_type: feature_index
feature_id: feature-backend-log-streaming
status: proposed
feature_type: feature
owners: []
last_reviewed: 2026-02-16
code_paths:
  - internal/capture/accessor.go
  - internal/capture/buffer_clear.go
  - internal/capture/buffer-types.go
  - internal/capture/capture-struct.go
  - internal/capture/circuit_breaker.go
  - internal/capture/constants.go
  - internal/capture/debug_logger.go
  - internal/capture/debug.go
  - internal/capture/enhanced_actions.go
  - internal/capture/enhanced-actions-types.go
  - internal/capture/extension_log_redaction.go
  - internal/capture/extension_logs.go
  - internal/capture/extension_state.go
  - internal/capture/extension-logging-types.go
  - internal/capture/handlers.go
  - internal/capture/helpers.go
  - internal/capture/http_debug_redaction.go
  - internal/capture/interfaces.go
  - internal/capture/internal-types.go
  - internal/capture/log-diff.go
  - internal/capture/memory.go
  - internal/capture/network_bodies.go
  - internal/capture/network_waterfall.go
  - internal/capture/network-types.go
  - internal/capture/playback.go
  - internal/capture/queries.go
  - internal/capture/query_dispatcher.go
  - internal/capture/rate_limit.go
  - internal/capture/recording_manager.go
  - internal/capture/recording.go
  - internal/capture/security-types.go
  - internal/capture/session-types.go
  - internal/capture/settings.go
  - internal/capture/status.go
  - internal/capture/sync.go
  - internal/capture/test_helpers.go
  - internal/capture/testhelpers.go
  - internal/capture/ttl.go
  - internal/capture/type-aliases.go
  - internal/capture/types.go
  - internal/capture/websocket-types.go
  - internal/capture/websocket.go
  - src/lib/network.ts
  - src/lib/websocket.ts
test_paths: []
---

# Backend Log Streaming

## TL;DR

- Status: proposed
- Tool: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical tool enums.
- Mode/Action: See feature contract and `docs/core/mcp-command-option-matrix.md` for canonical `what`/`action`/`format` enums.
- Location: `docs/features/feature/backend-log-streaming`

## Specs

- Product Spec: [product-spec.md](./product-spec.md)
- Tech Spec: [tech-spec.md](./tech-spec.md)
- QA Plan: [qa-plan.md](./qa-plan.md)

## Requirement IDs

- FEATURE_BACKEND_LOG_STREAMING_001
- FEATURE_BACKEND_LOG_STREAMING_002
- FEATURE_BACKEND_LOG_STREAMING_003

## Code and Tests

Add concrete implementation and test links here as this feature evolves.
