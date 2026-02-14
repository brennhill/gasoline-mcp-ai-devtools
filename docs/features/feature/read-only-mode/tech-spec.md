---
feature: read-only-mode
status: proposed
---

# Tech Spec: Read-Only Mode

> Plain language only. No code. Describes HOW the implementation works at a high level.

## Architecture Overview

Read-only mode is a server-wide boolean flag set at startup via CLI flag or environment variable. The flag is immutable after server initialization. All MCP tool handlers check this flag before executing mutations. If read-only is true, mutation operations return error immediately without creating pending queries or touching browser state.

## Key Components

- **Server config**: Add ReadOnlyMode boolean field to server config struct
- **CLI flag**: Add --read-only flag to main.go
- **Environment variable**: Check GASOLINE_READ_ONLY env var
- **Tool handler guards**: Each mutation operation checks ReadOnlyMode before execution
- **Error response**: Return standard error format with "read_only_mode_enabled" code
- **Observe integration**: Include read_only_mode in server_config observe mode

## Data Flows

```
Server startup: gasoline --read-only
  → Parse CLI flags, read environment
  → Set config.ReadOnlyMode = true
  → Log: "Read-only mode enabled, mutation tools disabled"

Agent calls interact tool:
  → Server receives MCP request
  → Handler checks: if config.ReadOnlyMode { return error }
  → Return: {error: "read_only_mode_enabled", message: "..."}
  → Agent receives error, does not attempt mutation

Agent calls observe tool:
  → Server receives MCP request
  → Handler checks: observe is always allowed
  → Execute normally, return data

Agent calls configure({action: "clear"}):
  → Server receives MCP request
  → Handler checks action: "clear" is mutation
  → If read-only, return error
  → Else execute normally
```

## Implementation Strategy

### Mutation detection:
- Categorize all tool actions as observation (allowed) or mutation (blocked)
- Observation: observe (all modes), generate (all types), configure (query_dom, health, streaming status)
- Mutation: interact (all actions), configure (store, load, clear, dismiss, noise_rule, diff_sessions)

### Flag enforcement:
1. Initialize ReadOnlyMode from CLI flag or env var at server start
2. Make field immutable (no API to toggle at runtime)
3. In each tool handler, add guard: if readOnly && isMutation { return error }
4. Return consistent error response with actionable message

### Error format:
```json
{
  "error": "read_only_mode_enabled",
  "message": "Interactive features are disabled in read-only mode. Only observation and analysis tools are available. Restart server without --read-only to enable mutations."
}
```

### Server config exposure:
Add "server_config" mode to observe tool to expose current configuration:
```json
observe({what: "server_config"})
// Returns:
{
  "read_only_mode": true,
  "port": 7890,
  "persist": true,
  "network_waterfall_capacity": 1000
}
```

## Edge Cases & Assumptions

- **Edge Case 1**: Agent doesn't check read-only status, tries mutation → **Handling**: Return error with clear message
- **Edge Case 2**: CLI flag conflicts with env var → **Handling**: CLI flag takes precedence
- **Edge Case 3**: User wants to toggle read-only without restart → **Handling**: Not supported, document as limitation
- **Edge Case 4**: Extension doesn't know about read-only mode → **Handling**: Server never creates pending queries for mutations, extension never receives them
- **Assumption 1**: Read-only is binary (all or nothing), no granular permissions
- **Assumption 2**: Clients respect error responses, don't retry mutations

## Risks & Mitigations

- **Risk 1**: Agent bypasses check by calling extension HTTP directly → **Mitigation**: Extension requires server to create pending queries, cannot be bypassed
- **Risk 2**: Read-only mode too restrictive, blocks needed operations → **Mitigation**: Use tool-allowlisting for granular control
- **Risk 3**: Error message unclear, agent doesn't understand → **Mitigation**: Use explicit error code and actionable message
- **Risk 4**: Agent retries mutation repeatedly → **Mitigation**: Return same error every time, agent should detect pattern

## Dependencies

- Server config struct
- MCP tool handlers (observe, generate, configure, interact)
- CLI flag parsing

## Performance Considerations

- Read-only check is O(1) boolean comparison (negligible overhead)
- No performance impact on observation operations
- Prevents expensive mutation operations from executing

## Security Considerations

- **Immutable after start**: Cannot be toggled at runtime, prevents privilege escalation
- **Server-side enforcement**: Client cannot bypass, enforced at MCP handler level
- **Clear audit trail**: All mutation attempts logged as errors
- **Defense in depth**: Complements other security features (tool allowlisting, project isolation)
- **Production safety**: Primary use case is production environments where mutation is prohibited
