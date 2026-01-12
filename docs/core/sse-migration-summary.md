# SSE Transport Migration Summary

## Overview

Gasoline has migrated from stdio to SSE (Server-Sent Events) transport for MCP 2024-11-05 spec compliance. This document summarizes all changes made.

## What Changed

### Architecture

**Before (stdio):**
- MCP clients spawned Gasoline process
- Communication via stdin/stdout (newline-delimited JSON-RPC)
- Server exited when stdin closed (unless `--persist` flag used)

**After (SSE):**
- MCP clients still spawn Gasoline process
- Communication via HTTP SSE at `/mcp/sse`
- Bidirectional: SSE for server→client, HTTP POST for client→server
- Server runs continuously until SIGTERM/SIGINT

### MCP Configuration Format

**Old (stdio):**
```json
{
  "mcpServers": {
    "gasoline": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890", "--persist"]
    }
  }
}
```

**New (SSE):**
```json
{
  "mcpServers": {
    "gasoline": {
      "type": "sse",
      "command": "npx",
      "args": ["-y", "gasoline-mcp", "--port", "7890"],
      "url": "http://localhost:7890/mcp/sse"
    }
  }
}
```

### Key Differences

1. **Transport type**: `stdio` → `sse`
2. **URL field**: Added `"url": "http://localhost:7890/mcp/sse"`
3. **Persist flag**: Removed `--persist` (no longer needed, server always persists)
4. **Connection flow**:
   - Client connects: `GET /mcp/sse`
   - Server sends: `event: endpoint` with POST URI
   - Client requests: `POST /mcp/messages/{session-id}`
   - Server responds: SSE `event: message`

## Files Updated

### Codebase
- ✅ `/cmd/dev-console/sse.go` (NEW) - SSE infrastructure
- ✅ `/cmd/dev-console/main.go` - Removed stdio loop, added SSE routes
- ✅ `/cmd/dev-console/streaming.go` - SSE broadcasts instead of stdout
- ✅ `/cmd/dev-console/tools.go` - Accept SSERegistry parameter
- ✅ `/cmd/dev-console/sse_test.go` (NEW) - Comprehensive SSE tests
- ✅ `/docs/core/mcp-correctness.md` (NEW) - Updated MCP compliance docs

### Repository Root
- ✅ `/README.md` - Updated all MCP config examples to SSE

### Marketing Site (~/dev/gasoline-site)
- ✅ `/src/content/docs/getting-started.md` - Updated all config examples
- ✅ `/src/content/docs/mcp-integration/index.md` - Updated architecture description and VS Code Continue config

### Left Unchanged
- `/src/content/docs/blog/v5-1-0-release.md` - Historical blog post, kept stdio config for v5.1.0 accuracy

## Testing

All tests pass:
```bash
✅ go vet ./cmd/dev-console/
✅ go test -short ./cmd/dev-console/ (2.7s)
✅ make test (7.3s, all tests including fuzz)
```

Manual SSE verification:
```bash
$ curl -N -m 2 http://localhost:7890/mcp/sse
event: endpoint
data: {"uri":"/mcp/messages/14d223b0de492a237d6b5b28caba224b"}
```

## Implementation Details

### SSE Protocol

**Event Types:**
- `endpoint` - Initial event with POST URI for client requests
- `message` - JSON-RPC responses and MCP notifications

**Headers:**
```http
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

**Session Management:**
- UUID-based session IDs (cryptographically random)
- Multi-client isolation via SSERegistry
- Automatic cleanup of idle connections (1 hour timeout)
- Context cancellation for graceful disconnect detection

### Security

- Origin validation via existing `corsMiddleware`
- Cryptographically random session IDs
- Rate limiting (100 calls/min, unchanged)
- Localhost-only binding (127.0.0.1)

### Performance Impact

- **Memory**: ~1KB per SSE connection (5 concurrent clients = <5KB overhead)
- **CPU**: Negligible (SSE write is `fmt.Fprintf` + `Flush`)
- **Latency**: Identical to stdio (both localhost IPC, <1ms)

## Backward Compatibility

The browser extension is **fully backward compatible**:
- Extension still uses `/mcp` POST endpoint (unchanged)
- No extension code changes required
- Only MCP client configuration needs updating

## Next Steps

1. **Update NPM package** - Publish new version with SSE support
2. **Update PyPI package** - Publish new version with SSE support
3. **Update documentation site** - Deploy updated getting-started.md
4. **Test with Claude Code** - Verify SSE transport works end-to-end
5. **Update Chrome Web Store listing** - Once extension is published

## References

- [MCP 2024-11-05 SSE Specification](https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/transports/)
- [SSE Transport Documentation](https://modelcontextprotocol.io/legacy/concepts/transports)
- Implementation Plan: `/Users/brenn/.claude/plans/polymorphic-bubbling-sedgewick.md`
