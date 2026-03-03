---
doc_type: feature_index
feature_id: feature-chat-panel
status: implementation
feature_type: feature
owners: []
last_reviewed: 2026-03-03
code_paths:
  - internal/push/chat_session.go
  - cmd/dev-console/chat_handlers.go
  - cmd/dev-console/push_handlers.go
  - cmd/dev-console/bridge.go
  - cmd/dev-console/server.go
  - cmd/dev-console/server_routes.go
  - internal/push/sampling.go
  - src/content/ui/chat-panel.ts
  - src/content/ui/chat-panel-sse.ts
  - src/content/runtime-message-listener.ts
  - src/background/push-handler.ts
test_paths:
  - internal/push/chat_session_test.go
  - cmd/dev-console/chat_handlers_test.go
  - cmd/dev-console/bridge_sampling_test.go
  - tests/extension/chat-panel.test.js
  - tests/extension/chat-panel-sse.test.js
---

# Chat Panel

Conversational side panel on any tracked page. User types messages, annotations auto-push into the conversation, and AI responses stream back via SSE.

## TL;DR

- Status: implementation
- Tool: internal (push delivery, SSE streaming)
- Mode/Action: MCP sampling round-trip, SSE response delivery
- Shortcuts: Alt+Shift+C (toggle panel), Draw button (in-panel draw mode trigger)
- Location: `docs/features/feature/chat-panel`

## Specs

- Flow Map: [flow-map.md](./flow-map.md)

## Requirement IDs

- CHAT_001 — User message round-trip (browser → daemon → AI → SSE → browser)
- CHAT_002 — SSE streaming endpoint with history replay
- CHAT_003 — Bridge sampling response interception
- CHAT_004 — Annotation auto-injection from draw mode
- CHAT_005 — Chat panel UI with dark theme

## Code and Tests

### Go (daemon)

| File | Purpose | Tests |
|------|---------|-------|
| `internal/push/chat_session.go` | Bounded message store with pub/sub | `chat_session_test.go` |
| `cmd/dev-console/chat_handlers.go` | SSE endpoint + response handler | `chat_handlers_test.go` |
| `cmd/dev-console/push_handlers.go` | Enhanced push message with `conversation_id` | `push_handlers_test.go` |
| `cmd/dev-console/bridge.go` | Sampling response detection + forwarding | `bridge_sampling_test.go` |
| `cmd/dev-console/server.go` | ChatSession + samplingRequests fields | — |
| `cmd/dev-console/server_routes.go` | Route registration | — |
| `internal/push/sampling.go` | Request ID tracking | `sampling_test.go` |

### TypeScript (extension)

| File | Purpose |
|------|---------|
| `src/content/ui/chat-panel.ts` | Side panel UI: messages, input, draw button |
| `src/content/ui/chat-panel-sse.ts` | SSE client using fetch + ReadableStream |
| `src/content/runtime-message-listener.ts` | `GASOLINE_TOGGLE_CHAT` handler |
| `src/background/push-handler.ts` | `pushChatMessage()` with conversation_id |

### Extension Tests

| File | Coverage |
|------|----------|
| `tests/extension/chat-panel.test.js` | Panel toggle, message rendering, draw button, annotation cards |
| `tests/extension/chat-panel-sse.test.js` | SSE parsing, reconnect, close cleanup |
