# MCP Resources: Usage Guide

## Status: Approved
## Priority: P0
## Author: Claude + Brenn
## Date: 2025-01-26

---

## Problem Statement

AI assistants connecting to Gasoline via MCP don't know how to use the tools. Users must manually explain "use observe with what: errors" every time they start a new project.

**Goal**: Expose a usage guide as an MCP resource that AI assistants can read on connect.

---

## MCP Resources Protocol

Per the [MCP specification](https://modelcontextprotocol.io/), resources are read-only data that servers expose to clients.

### Required Methods

| Method | Description |
|--------|-------------|
| `resources/list` | Returns available resources with URIs |
| `resources/read` | Returns content for a specific URI |

### Server Capabilities

Server must declare resources capability in `initialize` response:

```json
{
  "capabilities": {
    "tools": {},
    "resources": {}
  }
}
```

---

## Implementation

### Resource: Usage Guide

**URI**: `gasoline://guide`
**MIME Type**: `text/markdown`
**Purpose**: Teach AI assistants how to use Gasoline tools

### resources/list Response

```json
{
  "resources": [
    {
      "uri": "gasoline://guide",
      "name": "Gasoline Usage Guide",
      "description": "How to use Gasoline MCP tools for browser debugging",
      "mimeType": "text/markdown"
    }
  ]
}
```

### resources/read Response

```json
{
  "contents": [
    {
      "uri": "gasoline://guide",
      "mimeType": "text/markdown",
      "text": "<markdown content>"
    }
  ]
}
```

---

## Usage Guide Content

The guide should be concise (AI context is precious) but complete:

```markdown
# Gasoline MCP Tools

Browser observability for AI coding agents. See console errors, network failures, DOM state, and more.

## Quick Reference

| Tool | Purpose | Key Parameter |
|------|---------|---------------|
| `observe` | Get browser state | `what`: errors, logs, network, websocket, actions, vitals, page |
| `analyze` | Analyze data | `target`: performance, accessibility, changes, timeline, api |
| `generate` | Create artifacts | `format`: test, reproduction, pr_summary, sarif, har |
| `configure` | Manage session | `action`: store, noise_rule, dismiss, clear |
| `query_dom` | Query live DOM | `selector`: CSS selector |

## Common Workflows

### See browser errors
```json
{ "tool": "observe", "arguments": { "what": "errors" } }
```

### Check network failures
```json
{ "tool": "observe", "arguments": { "what": "network", "status_min": 400 } }
```

### Run accessibility audit
```json
{ "tool": "analyze", "arguments": { "target": "accessibility" } }
```

### Query DOM element
```json
{ "tool": "query_dom", "arguments": { "selector": ".error-message" } }
```

### Generate Playwright test
```json
{ "tool": "generate", "arguments": { "format": "test", "test_name": "user_login" } }
```

### Check Web Vitals
```json
{ "tool": "observe", "arguments": { "what": "vitals" } }
```

## Tips

- Call `observe` with `what: "errors"` first to see what's broken
- Use `what: "page"` to confirm which page the browser is on
- The extension must show "Connected" for tools to work
- Data comes from the active browser tab
```

---

## Code Changes

### main.go

Add handlers for `resources/list` and `resources/read` in `HandleRequest`:

```go
case "resources/list":
    return h.handleResourcesList(req)
case "resources/read":
    return h.handleResourcesRead(req)
```

### Capabilities

Update `handleInitialize` to declare resources capability:

```go
Capabilities: MCPCapabilities{
    Tools:     MCPToolsCapability{},
    Resources: MCPResourcesCapability{},
},
```

---

## Testing

1. **Unit test**: Verify `resources/list` returns the guide
2. **Unit test**: Verify `resources/read` returns markdown content
3. **Integration test**: Connect MCP client, read resource, verify content

---

## Success Criteria

1. `resources/list` returns usage guide resource
2. `resources/read` returns markdown content
3. AI assistants can discover and read the guide without user intervention
4. Guide content is accurate and covers all 5 composite tools
