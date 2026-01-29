# MCP Correctness

Gasoline implements the [Model Context Protocol](https://modelcontextprotocol.io/specification/2025-11-25) specification using SSE transport and custom HTTP bridge.

**Protocol Version:** `2024-11-05` (with version negotiation for `2025-11-25`)

## Compliance Status

**All MUST constraints:** ✅ PASS
**Intentional deviations:** 3 (documented below)
**Transport:** SSE (Server-Sent Events) for MCP spec compliance

## Violations Summary

**Current:** None (all violations fixed)

## Intentional Deviations

| ID | Deviation | Justification |
|----|-----------|---------------|
| L-6, J-6 | Respond to `initialized` notification with `{}` | Some MCP clients (including Claude Code) expect a response. Removing it could break compatibility. Track spec evolution. |
| — | `_meta` field on tools with `data_counts` | Non-standard but uses `_` prefix convention. Provides AI with buffer state without extra tool call. No spec conflict. |
| — | `X-Gasoline-Client` header for multi-client | Not part of MCP spec. Internal transport-layer addition for our `/mcp` HTTP bridge. Does not affect SSE transport. |
| — | Only `type: "text"` content blocks | MCP allows image/resource content types. All our data is textual. No spec violation — we just don't use all content types. |

## Key Implementation Details

**Capabilities declared:**
- `tools` — 5 tools: observe, generate, configure, interact, analyze
- `resources` — 1 resource: `gasoline://guide`

**Error handling:**
- Three-tier model: transport (HTTP status) → protocol (JSON-RPC error) → application (`isError: true`)
- Tool execution failures use `isError: true` (not JSON-RPC errors)
- Rate limiting: 100 tool calls per minute per client

**Security:**
- Server binds `127.0.0.1` only
- Origin validation with strict localhost/extension allowlist
- Sensitive data redaction in all tool outputs
- Cryptographically random session IDs for SSE connections

**Transport:**
- **Primary (MCP clients):** SSE (Server-Sent Events) at `/mcp/sse` with POST to `/mcp/messages/{session-id}`
  - Bidirectional communication: SSE for server→client, HTTP POST for client→server
  - MCP 2024-11-05 compliant SSE transport
  - Session-based routing with UUID session IDs
  - Automatic cleanup of stale connections (1 hour timeout)
- **Secondary (browser extension):** Custom `/mcp` POST endpoint for backward compatibility
  - HTTP bridge for extension telemetry
  - Not part of MCP spec, documented intentional deviation

## SSE Transport Details

**Connection Flow:**
1. Client connects: `GET /mcp/sse`
2. Server sends `endpoint` event with POST URI: `/mcp/messages/{session-id}`
3. Client sends requests: `POST /mcp/messages/{session-id}` with JSON-RPC 2.0
4. Server sends responses/notifications via SSE `message` events

**Event Types:**
- `endpoint` — Initial event with POST URI for client requests
- `message` — JSON-RPC responses and MCP notifications

**SSE Headers:**
```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

**Session Management:**
- UUID-based session IDs (cryptographically random)
- Multi-client isolation via session registry
- Automatic cleanup of idle connections (1 hour)
- Context cancellation for graceful disconnect detection

## Test Coverage

80+ MCP constraints covered by:
- `main_test.go` — JSON-RPC protocol, lifecycle, tools, resources
- `redaction_test.go` — Security sanitization
- `multi_client_test.go` — Concurrent client isolation
- `sse_test.go` — SSE transport, connection lifecycle, thread safety

## References

- [MCP 2024-11-05 Specification](https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/transports/)
- [SSE Transport Documentation](https://modelcontextprotocol.io/legacy/concepts/transports)
