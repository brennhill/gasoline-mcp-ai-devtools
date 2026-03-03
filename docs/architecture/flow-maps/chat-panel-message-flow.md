---
doc_type: flow_map
scope: chat_panel_message_flow
status: active
last_reviewed: 2026-03-03
owners:
  - Brenn
---

# Chat Panel Message Flow

## Scope

Full round-trip for in-page chat messages: user input through daemon ChatSession, MCP sampling delivery to AI, response interception at the bridge, SSE push back to the browser panel. Includes annotation auto-injection from draw mode.

## Entrypoints

1. **Extension:** `toggleChatPanel()` in `src/content/ui/chat-panel.ts` — opens/closes the side panel
2. **Extension:** `connectChatStream()` in `src/content/ui/chat-panel-sse.ts` — establishes SSE connection to daemon
3. **Go Push Handler:** `handlePushMessage()` in `cmd/dev-console/push_handlers.go` — receives user messages with `conversation_id`
4. **Go SSE Handler:** `handleChatStream()` in `cmd/dev-console/chat_handlers.go` — SSE endpoint for message delivery
5. **Go Response Handler:** `handleChatResponse()` in `cmd/dev-console/chat_handlers.go` — receives bridge-forwarded AI responses
6. **Bridge:** `forwardSamplingResponse()` in `cmd/dev-console/bridge.go` — intercepts sampling responses from stdin

## Primary Flow

### User Message Round-Trip

1. User types message in chat panel and presses Enter or clicks Send
2. Content script sends `GASOLINE_PUSH_CHAT` message to background service worker
3. Background `pushChatMessage()` sends `POST /push/message` with `conversation_id` and message text
4. Daemon `handlePushMessage()` creates or retrieves `ChatSession`, adds user message
5. Daemon builds `sampling/createMessage` JSON-RPC request via `BuildSamplingRequest()`
6. Daemon writes sampling request to stdout, tracking `requestID → conversationID` in `samplingRequests` map
7. AI client receives sampling request, processes it, sends JSON-RPC response to stdin
8. Bridge `bridgeStdioToHTTPFast()` detects response (Method empty + ID present)
9. Bridge `forwardSamplingResponse()` sends `POST /chat/response` to daemon
10. Daemon `handleChatResponse()` extracts assistant text, adds to `ChatSession`
11. `ChatSession.AddMessage()` notifies all SSE subscribers via channel send
12. SSE handler writes `event: message\ndata: {json}\n\n` and flushes
13. Extension `connectChatStream()` parser receives SSE event, calls `onMessage` callback
14. Chat panel renders assistant message bubble

### Annotation Auto-Push

1. User clicks "Draw" in chat panel (or presses Alt+Shift+D)
2. Draw mode activates, user draws annotations, presses Escape
3. `POST /draw-mode/complete` fires (existing flow)
4. `pushDrawModeCompletion()` checks if `s.chatSession != nil`
5. If active: injects annotation data as `ChatMessage{role: "annotation"}` into session
6. Normal message flow continues from step 5 above — annotation is part of sampling context
7. AI sees annotations in conversation, responds with analysis
8. Response streams back to panel via SSE (steps 7-14 above)

### SSE Connection

1. Chat panel opens → `connectChatStream()` sends `GET /chat/stream?conversation_id=...`
2. Daemon sends `event: history\ndata: [...]\n\n` with existing messages as initial burst
3. Daemon subscribes to `ChatSession` and enters long-poll loop
4. On new message: writes SSE event + flushes
5. Heartbeat comment (`: heartbeat\n\n`) every 15 seconds
6. On `r.Context().Done()`: unsubscribes and returns

## Error and Recovery Paths

- **SSE disconnect:** Extension auto-reconnects after 1 second; server replays history on reconnect
- **ChatSession full (100 messages):** oldest message evicted on new add; conversation continues
- **Sampling request timeout:** request ID cleaned from `samplingRequests` map; no response delivered
- **Bridge forwarding failure:** logged; message lost (user can resend)
- **No active chat session for annotation push:** annotation stored normally but not injected into chat
- **Multiple SSE subscribers:** all receive the same messages via independent channels

## State and Contracts

- **ChatSession capacity:** 100 messages max (bounded, FIFO eviction)
- **SSE heartbeat:** 15-second interval
- **Conversation scope:** single active session per daemon (MVP)
- **Message roles:** `"user"`, `"assistant"`, `"annotation"`
- **Wire format:** all JSON fields use `snake_case`
- **Sampling request tracking:** `sync.Map` of `int64 → string` (requestID → conversationID)

## Code Paths

| Component | File |
|-----------|------|
| Chat session data structure | `internal/push/chat_session.go` |
| SSE endpoint + response handler | `cmd/dev-console/chat_handlers.go` |
| Push message handler (enhanced) | `cmd/dev-console/push_handlers.go` |
| Bridge response interception | `cmd/dev-console/bridge.go` |
| Server routes + session field | `cmd/dev-console/server.go`, `cmd/dev-console/server_routes.go` |
| Sampling request builder | `internal/push/sampling.go` |
| Extension chat panel UI | `src/content/ui/chat-panel.ts` |
| Extension SSE client | `src/content/ui/chat-panel-sse.ts` |
| Extension message handler | `src/content/runtime-message-listener.ts` |
| Extension push handler | `src/background/push-handler.ts` |

## Test Paths

| Component | File |
|-----------|------|
| Chat session CRUD, pub/sub, eviction | `internal/push/chat_session_test.go` |
| SSE endpoint, response handling | `cmd/dev-console/chat_handlers_test.go` |
| Bridge response detection, forwarding | `cmd/dev-console/bridge_sampling_test.go` |
| Panel toggle, message rendering, draw | `tests/extension/chat-panel.test.js` |
| SSE parsing, reconnect, close | `tests/extension/chat-panel-sse.test.js` |

## Edit Guardrails

- `ChatSession` must be goroutine-safe — all access guarded by `sync.RWMutex`
- Subscriber channels must be buffered (1) to avoid blocking `AddMessage` on slow readers
- SSE response headers must be set before first `Flush()` — once flushed, headers are locked
- Bridge response detection relies on `Method == ""` + `ID != nil` — do not change JSON-RPC parsing order
- `samplingRequests` cleanup must be deterministic — remove on response receipt or timeout, never rely on GC
- Draw mode annotation injection is best-effort — must not block or error if no active chat session
- Extension chat panel z-index (`2147483643`) must not conflict with existing draw-mode overlay
