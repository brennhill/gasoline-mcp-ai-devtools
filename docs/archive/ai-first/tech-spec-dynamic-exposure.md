# Tech Spec: Dynamic Tool Exposure (Phase 2)

## Purpose

The MCP `tools/list` response should change based on what data is currently available in the server. This reduces cognitive load on AI consumers by only showing tools and modes that are actionable right now. An AI seeing `analyze(target: "accessibility")` when no a11y audit has been cached is wasted schema space.

## How It Works

### State-Aware Tool List

The `toolsList()` method on `ToolHandler` inspects the current state of `Server` (log entries), `Capture` (network bodies, WebSocket events, actions, performance snapshots, a11y cache, API schema store), and `SessionStore` (persistent data) to determine which tool modes are available.

For each composite tool, the enum values in the mode parameter are filtered to only include modes that have data. If a tool has zero available modes, it is omitted entirely from the response.

### Availability Rules

**`observe` tool — `what` enum filtering:**

| Mode | Available when |
|------|----------------|
| `errors` | Always (errors may arrive at any time) |
| `logs` | Always (same reason) |
| `network` | `capture.networkBodies` is non-empty |
| `websocket_events` | `capture.wsEvents` is non-empty |
| `websocket_status` | `capture.connections` is non-empty |
| `actions` | `capture.enhancedActions` is non-empty |
| `vitals` | `capture.perf.snapshots` is non-empty |
| `page` | Always (live query, data is on-demand) |

**`analyze` tool — `target` enum filtering:**

| Mode | Available when |
|------|----------------|
| `performance` | `capture.perf.snapshots` is non-empty |
| `api` | `capture.schemaStore` has observed endpoints |
| `accessibility` | Always (triggers a live audit) |
| `changes` | `server.entries` or any capture buffer is non-empty |
| `timeline` | `capture.enhancedActions` is non-empty |

**`generate` tool — `format` enum filtering:**

| Mode | Available when |
|------|----------------|
| `reproduction` | `capture.enhancedActions` is non-empty |
| `test` | `capture.enhancedActions` is non-empty |
| `pr_summary` | Any data exists (entries, actions, network, perf) |
| `sarif` | Always (triggers a live audit) |
| `har` | `capture.networkBodies` is non-empty |

**`configure` tool:**

Always available with all modes. Configuration is always valid.

**`query_dom` tool:**

Always available. It's a live query.

### Progressive Disclosure

The server tracks whether the AI has made at least one successful `observe` call (any mode). Until that first observation:

- Only `observe` and `query_dom` are exposed
- `analyze`, `generate`, and `configure` are hidden

After the first successful observation, all tools that have available data are exposed. This prevents AI models from calling `generate(format: "reproduction")` before any actions have been captured.

The "first observation" flag is stored as a boolean on `ToolHandler` and resets when the server restarts. It is NOT persisted.

### Capability Annotations

Each tool in the `tools/list` response includes an optional `_capabilities` annotation in its description or as a top-level field. This is a structured hint for AI consumers:

```
{
  "name": "observe",
  "description": "...",
  "_meta": {
    "available_modes": ["errors", "logs", "network", "actions"],
    "data_counts": {
      "errors": 3,
      "logs": 47,
      "network": 12,
      "actions": 8
    }
  }
}
```

The `_meta` field uses the MCP convention for server-to-client metadata. It is informational — AI consumers may ignore it, but capable consumers use it to prioritize which mode to call first.

### Data Count Computation

Counts are computed at `tools/list` time by reading current buffer lengths:

- `errors`: count of entries where `level == "error"`
- `logs`: total `server.entries` count
- `network`: `len(capture.networkBodies)`
- `websocket_events`: `len(capture.wsEvents)`
- `websocket_status`: `len(capture.connections)`
- `actions`: `len(capture.enhancedActions)`
- `vitals`: `len(capture.perf.snapshots)`
- `performance`: same as vitals
- `api`: `capture.schemaStore.EndpointCount()`
- `timeline`: same as actions
- `reproduction`/`test`: same as actions
- `har`: same as network

### Concurrency

The `toolsList()` method acquires read locks on both `server.mu` and `capture.mu` to compute availability. Since `tools/list` is called infrequently (typically once at session start, occasionally on reconnect), the lock contention is negligible.

## Edge Cases

- If all observe modes are empty (fresh start), `observe` still shows `errors`, `logs`, and `page` (always-available modes).
- If progressive disclosure is active (no observation yet), `tools/list` returns exactly 2 tools.
- A `tools/list` call itself does NOT count as an observation.
- The `_meta` field is omitted entirely for tools with no meaningful counts (like `configure`).

## Performance Constraints

- `tools/list` must respond in under 1ms (just reading buffer lengths under read locks).
- No allocations beyond the response construction itself.

## Test Scenarios

1. Fresh server: `tools/list` returns only `observe` (with errors, logs, page) and `query_dom`.
2. After adding network bodies: `observe` gains "network" mode; if first observation made, `generate` appears with "har".
3. After first `observe` call: `analyze`, `generate`, `configure` become visible (progressive disclosure lifted).
4. After adding actions: `observe` gains "actions"; `analyze` gains "timeline"; `generate` gains "reproduction" and "test".
5. After adding performance snapshot: `observe` gains "vitals"; `analyze` gains "performance".
6. `configure` always shows all modes once progressive disclosure is lifted.
7. `_meta.data_counts` reflects current buffer sizes accurately.
8. Enum filtering: if network bodies exist but actions don't, `observe.what` enum has "network" but not "actions".
