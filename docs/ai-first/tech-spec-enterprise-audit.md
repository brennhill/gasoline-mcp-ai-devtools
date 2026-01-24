# Technical Spec: Enterprise Audit & Governance

## Purpose

Gasoline captures sensitive browser state — console logs, network bodies, WebSocket payloads, DOM content, and accessibility trees. In enterprise environments, AI tools accessing this data create compliance and security concerns: Who called which tool? What data was exposed? How long is it retained? Can runaway AI agents be constrained?

This specification defines four tiers of enterprise-readiness features that give security teams visibility, control, and auditability over how AI agents interact with captured browser data through Gasoline's MCP interface.

---

## Tier 1: AI Audit Trail

### 1.1 Tool Invocation Log

Every MCP tool call produces an audit entry in an append-only log. The log captures what was called, by whom, when, and the outcome — without storing the full response content (which may be large and sensitive).

#### How It Works

When the MCP dispatcher receives a tool call, it records an entry before executing the tool. After execution, it updates the entry with the result metadata (duration, response size, error if any). The log is a fixed-size ring buffer (configurable, default 10,000 entries) stored in memory, with optional file persistence.

Each audit entry contains:
- Timestamp (ISO 8601 with millisecond precision)
- Session ID (see 1.3)
- Client identity (see 1.2)
- Tool name (e.g., `observe`, `query_dom`, `generate`)
- Parameter summary (tool name and key parameters, not full request body)
- Response size in bytes
- Duration in milliseconds
- Status (success, error, rate-limited, redacted)
- Redaction count (how many fields were redacted in the response, see 1.4)

The log is queryable via a new MCP tool `get_audit_log` that accepts filters: time range, tool name, session ID, status. The tool returns entries in reverse chronological order with pagination.

#### What the Log Does NOT Store

- Full request parameters (may contain sensitive queries)
- Full response bodies (may contain captured PII)
- Authentication credentials
- Raw network bodies or DOM content

The log stores metadata about access, not the accessed data itself. This keeps the audit log small and non-sensitive while still providing a complete access trail.

#### Persistence

By default, the audit log lives in memory and is lost on server restart. When file persistence is enabled (via `--audit-file` flag), entries are appended to a JSON Lines file. The file is append-only — entries are never modified or deleted. File rotation is the operator's responsibility (standard logrotate works).

---

### 1.2 Client Identification

Each MCP connection identifies the connecting client. This information is recorded on every audit entry and available in health metrics.

#### How It Works

When an MCP client connects (over stdio), Gasoline identifies it through a handshake mechanism. During the MCP `initialize` request, the client sends its `clientInfo` object (part of the MCP specification). Gasoline extracts:

- Client name (e.g., "claude-code", "cursor", "windsurf", "continue")
- Client version
- Process ID of the connecting client (from the stdio parent process)

If the client doesn't send `clientInfo` (older clients), Gasoline labels the connection as "unknown" with the process ID.

For HTTP API connections (extension, direct API calls), Gasoline uses the `User-Agent` header and optionally a custom `X-Gasoline-Client` header for explicit identification.

#### Storage

Client identity is stored per-session (see 1.3) and referenced by all audit entries for that session. It is not separately persisted — it lives as a field on the session record.

---

### 1.3 Session ID Assignment

Each MCP connection receives a unique session ID that correlates all tool calls within that connection's lifetime.

#### How It Works

When an MCP client sends the `initialize` request, Gasoline generates a session ID using a compact format: a base-36 timestamp prefix plus 4 random characters (e.g., `s_m5kq2a_7f3x`). This provides chronological sortability, uniqueness, and human-readability.

The session ID is:
- Returned in the `initialize` response as a server capability extension
- Included in every audit log entry for that connection
- Available via the `get_health` tool for active session listing
- Logged on connection close with total tool call count and duration

Sessions end when the stdio pipe closes (MCP client disconnects or process dies). The server detects this via EOF on stdin.

#### Multiple Concurrent Sessions

The server supports multiple simultaneous MCP sessions (e.g., two Claude Code instances connected in parallel). Each gets its own session ID. The audit log distinguishes which session made which call. Rate limits (Tier 3) apply per-session independently.

---

### 1.4 Redaction Audit Log

When data is redacted from a tool response (by configured redaction patterns, see Tier 2.4), the redaction itself is logged. This creates an audit trail proving that sensitive data was protected, without storing the sensitive data.

#### How It Works

Each time a redaction pattern matches content in a tool response, a redaction event is recorded containing:
- Timestamp
- Session ID
- Tool name that produced the response
- Field path where the match occurred (e.g., `entries[3].message`, `body.response`)
- Pattern name that triggered the redaction (e.g., "credit-card", "bearer-token")
- Number of characters redacted (length only, not content)

Redaction events are included in the audit log as a sub-entry of the tool invocation that triggered them. They are also separately queryable via `get_audit_log` with a `type: "redaction"` filter.

#### What Is NOT Stored

The redacted content itself is never stored anywhere — not in the audit log, not in a side channel, nowhere. The audit log proves "a credit card pattern was found and removed from field X" without revealing the card number.

---

## Tier 2: Data Governance

### 2.1 TTL-Based Retention

All captured data (console logs, network entries, WebSocket events, actions, network bodies) has a configurable time-to-live. Entries older than the TTL are automatically evicted during normal buffer operations.

#### How It Works

The server maintains a TTL value (default: unlimited, meaning data lives until buffer rotation evicts it). When TTL is set (via `--ttl` flag or config), every buffer read and write operation includes an age check. Entries with timestamps older than `now - TTL` are skipped on read and proactively evicted during periodic maintenance.

The eviction runs on a timer (every 30 seconds) and also piggybacks on buffer writes. This ensures stale data doesn't persist even if no new data arrives.

TTL values:
- Minimum: 1 minute
- Default: unlimited (existing behavior — buffer size limits still apply)
- Recommended for compliance: 1 hour for development, 15 minutes for sensitive environments

TTL applies to:
- Console log entries
- Network request/response metadata
- Network body storage
- WebSocket event buffer
- Action buffer
- Performance snapshots
- Checkpoint data

TTL does NOT apply to:
- Audit log entries (those have their own retention, governed by buffer size)
- Noise rules (configuration, not captured data)
- Server configuration state

#### Behavior on TTL Change

When TTL is reduced at runtime (via a future config reload mechanism), the next eviction pass removes all entries that exceed the new TTL. This is an immediate effect — there's no grace period for existing data.

---

### 2.2 Compliance Presets

Named configuration bundles that set multiple enterprise features to values appropriate for specific compliance frameworks. Presets are a convenience — they set the same flags that operators could set individually.

#### Available Presets

**`soc2`** — SOC 2 Type II audit requirements:
- TTL: 4 hours (retain enough for a work session, not overnight)
- Audit log: enabled, file persistence on
- Redaction: bearer tokens, API keys, session cookies
- Rate limits: default (no additional restriction)
- Client ID: required (reject connections without `clientInfo`)

**`hipaa`** — HIPAA covered entity requirements:
- TTL: 15 minutes (minimize PHI exposure window)
- Audit log: enabled, file persistence on
- Redaction: SSN, MRN, date-of-birth, email, phone, plus all `soc2` patterns
- Rate limits: `query_dom` limited to 5/min (DOM may contain PHI)
- Client ID: required

**`pci-dss`** — PCI DSS cardholder data environment:
- TTL: 30 minutes
- Audit log: enabled, file persistence on
- Redaction: credit card (Luhn-validated), CVV, cardholder name patterns, plus all `soc2` patterns
- Rate limits: `get_network_bodies` limited to 10/min (bodies may contain card data)
- Client ID: required
- Network body storage: disabled entirely (cardholder data must never be stored)

**`strict`** — Maximum restriction (custom environments):
- TTL: 5 minutes
- Audit log: enabled, file persistence on
- Redaction: all known patterns enabled
- Rate limits: 30 calls/min per tool (global)
- Client ID: required
- Read-only mode: enabled (no mutation tools)

#### How Presets Work

Presets are applied at server start via `--compliance-preset=soc2`. They set defaults for all related flags. Individual flags can still override preset values (e.g., `--compliance-preset=hipaa --ttl=30m` uses HIPAA defaults but overrides TTL to 30 minutes).

Presets are not runtime-changeable — they're a startup configuration. The active preset name is exposed in the health endpoint and audit log header.

---

### 2.3 Data Export & Archive

Operators can export captured data and audit logs to structured file formats for offline retention, SIEM ingestion, or compliance archives.

#### How It Works

A new MCP tool `export_data` generates an export file containing the requested data scope. The tool accepts:
- Scope: `audit`, `captures`, `all`
- Format: `jsonl` (JSON Lines), `csv`
- Time range: start/end timestamps (optional, defaults to all available data)
- Output: returns the file path where the export was written

Exports are written to a configurable directory (`--export-dir`, default: `./exports/`). Each export file is named with a timestamp and scope: `gasoline-export-audit-2026-01-24T10-30-00.jsonl`.

For `captures` scope, the export includes all buffer contents (console, network, WebSocket, actions, performance) serialized as JSON objects, one per line. Each line includes a `type` field identifying the data category.

For `audit` scope, the export includes all audit log entries.

The export is a point-in-time snapshot — data continues to be captured and evicted normally during the export.

#### SIEM Integration

The JSON Lines format is chosen specifically for SIEM compatibility. Each line is a self-contained JSON object that can be ingested by Splunk, Elastic, Datadog, or any log aggregator without custom parsing. Field names follow ECS (Elastic Common Schema) conventions where applicable.

---

### 2.4 Configurable Redaction Patterns

Operators define regex patterns that automatically redact sensitive data from MCP tool responses before they reach the AI client. Redaction happens at the response boundary — the server stores unredacted data internally but never exposes it through MCP.

#### How It Works

Redaction patterns are configured via a JSON configuration file (`--redaction-config`) or CLI flags. Each pattern has:
- Name: human-readable identifier (e.g., "credit-card", "bearer-token")
- Pattern: regex applied to string fields in tool responses
- Replacement: what to substitute (default: `[REDACTED:{name}]`)
- Scope: which tool responses to apply to (`all`, or specific tool names)

Patterns are applied after the tool generates its response but before serialization to the MCP client. The server walks all string fields in the response JSON, applying each active pattern. Matches are replaced with the configured replacement string.

#### Built-In Patterns

The server ships with these patterns (disabled by default, enabled by compliance presets or explicit config):

- `bearer-token`: `Bearer [A-Za-z0-9\-._~+/]+=*` — OAuth bearer tokens
- `api-key`: `(?i)(api[_-]?key|apikey|secret[_-]?key)\s*[:=]\s*\S+` — API keys in various formats
- `credit-card`: `\b[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}[- ]?[0-9]{4}\b` — Credit card numbers (with optional Luhn validation)
- `ssn`: `\b[0-9]{3}-[0-9]{2}-[0-9]{4}\b` — US Social Security Numbers
- `email`: `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z]{2,}\b` — Email addresses
- `jwt`: `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*` — JSON Web Tokens
- `session-cookie`: `(?i)(session|sid|token)\s*=\s*[A-Za-z0-9+/=_-]{16,}` — Session identifiers

#### Custom Patterns

Operators add custom patterns in the config file:

```json
{
  "patterns": [
    {"name": "internal-id", "pattern": "CUST-[0-9]{8}", "replacement": "[REDACTED:customer-id]"},
    {"name": "mrn", "pattern": "MRN[0-9]{7,10}", "replacement": "[REDACTED:medical-record]"}
  ]
}
```

#### Performance

Redaction adds latency to tool responses. With the built-in patterns on a typical response (2KB of text), the overhead should be under 1ms. Large responses (network bodies, full DOM trees) may see 2-5ms overhead. This is acceptable given the security benefit, but is bounded by only scanning string fields (not binary data or numeric arrays).

---

## Tier 3: Operational Safety

### 3.1 API Key Authentication

Optional authentication for the HTTP API that prevents unauthorized clients from connecting to the Gasoline server. When enabled, all HTTP requests (from extension, direct API) must include the key.

#### How It Works

When started with `--api-key=<secret>` or `GASOLINE_API_KEY` environment variable, the server requires an `Authorization: Bearer <secret>` header on all HTTP requests. Requests without the header or with the wrong key receive 401 Unauthorized.

The key is a shared secret — the same key is configured in the extension settings and the server. This is not intended for multi-user authentication (see Tier 4 for multi-tenant), but for preventing accidental connections from other tools on localhost.

MCP connections (stdio) are not affected by API key auth — stdio inherently authenticates via the process model (only the parent process can connect).

#### Extension Configuration

The extension's options page gains a "Server API Key" field. When set, the extension includes the key in all POST requests to the server. The key is stored in `chrome.storage.local` (encrypted at rest by Chrome).

#### Key Rotation

To rotate the key, restart the server with the new key and update the extension setting. During the brief window between server restart and extension update, the extension will receive 401s and enter its normal backoff behavior.

---

### 3.2 Per-Tool Rate Limits

Configurable rate limits that apply per MCP tool per session. This prevents a malfunctioning or malicious AI agent from hammering expensive tools (like `query_dom` or `get_network_bodies`) in a tight loop.

#### How It Works

Each tool has a configurable maximum calls per minute per session. When exceeded, the tool responds with an MCP error containing a retry-after hint. The rate limit resets every 60 seconds on a sliding window.

Default limits (can be overridden):
- `observe`: 60/min (high — common read operation)
- `query_dom`: 20/min (expensive — triggers round-trip to browser)
- `generate`: 30/min (moderate — codegen is CPU-bound)
- `analyze`: 30/min (moderate)
- `get_audit_log`: 10/min (low — audit queries shouldn't be in hot loops)
- `export_data`: 2/min (very low — exports are expensive and produce files)

Limits are configured via `--rate-limits` flag or config file:
```
--rate-limits="query_dom=10,generate=5,observe=120"
```

#### Rate Limit Response

When rate-limited, the tool returns a standard MCP error response:
```json
{
  "error": {
    "code": -32029,
    "message": "Rate limit exceeded for tool 'query_dom': 20/min. Retry after 45s.",
    "data": {"tool": "query_dom", "limit": 20, "window": "1m", "retry_after_seconds": 45}
  }
}
```

The error code -32029 is in the MCP application error range. Well-behaved AI clients should respect the retry hint.

---

### 3.3 Configurable Thresholds

All hardcoded server limits become configurable via CLI flags, environment variables, or a config file. This replaces the current approach of fixed constants in source code.

#### Configurable Values

| Parameter | Default | Flag | Env |
|-----------|---------|------|-----|
| Console buffer size | 1000 entries | `--buffer-console` | `GASOLINE_BUFFER_CONSOLE` |
| Network buffer size | 500 entries | `--buffer-network` | `GASOLINE_BUFFER_NETWORK` |
| WebSocket buffer size | 1000 events | `--buffer-websocket` | `GASOLINE_BUFFER_WEBSOCKET` |
| Action buffer size | 200 actions | `--buffer-actions` | `GASOLINE_BUFFER_ACTIONS` |
| Network body max size | 100KB | `--body-max-size` | `GASOLINE_BODY_MAX_SIZE` |
| Network body store count | 50 bodies | `--body-max-count` | `GASOLINE_BODY_MAX_COUNT` |
| Memory soft limit | 20MB | `--memory-soft` | `GASOLINE_MEMORY_SOFT` |
| Memory hard limit | 50MB | `--memory-hard` | `GASOLINE_MEMORY_HARD` |
| Global rate limit | 1000 events/sec | `--rate-limit` | `GASOLINE_RATE_LIMIT` |
| Server port | 7890 | `--port` | `GASOLINE_PORT` |
| TTL | unlimited | `--ttl` | `GASOLINE_TTL` |
| Audit log size | 10000 entries | `--audit-size` | `GASOLINE_AUDIT_SIZE` |

#### Config File

As an alternative to flags, a TOML config file can be specified via `--config`:

```toml
[buffers]
console = 2000
network = 1000
websocket = 5000
actions = 500

[memory]
soft_limit = "40MB"
hard_limit = "100MB"

[retention]
ttl = "2h"
audit_size = 50000

[auth]
api_key = "sk-gasoline-prod-abc123"

[compliance]
preset = "soc2"
```

Priority order: CLI flags > environment variables > config file > defaults.

---

### 3.4 Health & SLA Metrics

A new MCP tool `get_health` exposes server operational state for monitoring and alerting. This is distinct from the existing `/health` HTTP endpoint (which only returns circuit breaker state) — it provides comprehensive server metrics.

#### What It Reports

The tool returns:
- **Server**: version, uptime, PID, platform, go version
- **Memory**: current allocation, buffer breakdown by category, percent of hard limit used
- **Buffers**: entries/capacity for each buffer type, eviction counts since startup
- **Rate limiting**: current event rate, circuit breaker state, total 429s issued
- **Sessions**: active MCP session count, list of session IDs with client identity and connection duration
- **Audit**: total tool calls since startup, calls per tool breakdown, error count
- **Compliance**: active preset name (if any), TTL value, redaction patterns active count
- **Retention**: oldest entry timestamp per buffer, next eviction scheduled

#### Use Cases

- **Monitoring dashboards**: Poll `get_health` to track memory and rate trends
- **Alerting**: Trigger on memory > 80% of hard limit, circuit breaker open, or high error rate
- **Debugging**: When an AI agent reports missing data, check buffer utilization and eviction counts
- **Compliance reporting**: Verify TTL is enforced, redaction is active, audit log is persisting

---

## Tier 4: Multi-Tenant & Access Control

### 4.1 Project Isolation

Multiple isolated capture contexts on a single Gasoline server. Each project has independent buffers, configuration, and access. This supports scenarios where multiple developers or applications share a single server instance.

#### How It Works

Projects are created via a new HTTP endpoint `POST /projects` with a name and optional configuration overrides. Each project gets:
- Independent buffer sets (console, network, WebSocket, actions, performance)
- Independent checkpoint storage
- Independent noise rules
- Optional per-project TTL and rate limits

The extension specifies which project to send data to via a project ID in its configuration (options page). MCP clients specify the project via a parameter on the `initialize` request.

Default behavior (no project specified) uses a "default" project, maintaining backward compatibility with existing single-project deployments.

#### Resource Limits

Each project consumes memory from the global pool. The server enforces:
- Maximum number of projects: configurable (default 10)
- Per-project memory cap: global hard limit / max projects (fair share)
- Projects exceeding their share trigger eviction in that project only

#### Project Lifecycle

Projects persist in memory for the server's lifetime. They can be deleted via `DELETE /projects/{id}`, which immediately frees all buffers. There is no persistence across server restarts — projects are recreated on next connection.

---

### 4.2 Read-Only Mode

A server mode that accepts captured data from the extension but disables all MCP tools that modify server state. This creates a "observe but don't touch" deployment suitable for shared environments or untrusted AI clients.

#### What Is Disabled

In read-only mode, these operations return an error:
- `configure` with action `clear` (clearing buffers)
- `configure` with action `dismiss` (modifying noise rules)
- `configure` with action `noise_rule` (adding/removing noise rules)
- Checkpoint deletion
- Any future mutation tool

#### What Remains Active

- All observe tools (reading logs, network, WebSocket events, actions)
- All analyze tools (performance, API schema, accessibility, changes, timeline)
- All generate tools (reproduction scripts, tests, PR summaries, SARIF, HAR)
- Query DOM (read-only browser query)
- Audit log reads
- Health metrics

#### Activation

Read-only mode is set at server start via `--read-only` flag. It cannot be toggled at runtime — this prevents an AI agent from disabling read-only mode via a tool call.

---

### 4.3 Tool Allowlisting

Restrict which MCP tools are available to connected clients. Tools not on the allowlist are completely hidden — they don't appear in the `tools/list` response and calls to them return "unknown tool" errors.

#### How It Works

An allowlist is configured via `--tools-allow` flag or config file. Only the listed tools are exposed:

```
--tools-allow="observe,analyze,get_health,get_audit_log"
```

If no allowlist is set, all tools are available (default behavior).

There is also a blocklist (`--tools-block`) for the inverse case — allow everything except specific tools:

```
--tools-block="configure,generate,query_dom"
```

If both are specified, the allowlist takes priority (blocklist is ignored).

#### Tool Visibility

Hidden tools don't appear in the MCP `tools/list` response. An AI client that doesn't see a tool in the list won't try to call it. If somehow called directly, the server responds with the standard MCP "method not found" error — it doesn't reveal that the tool exists but is blocked.

#### Per-Session Overrides

Future extension: allowlists could be set per-client identity (e.g., "Cursor gets all tools, but the CI integration only gets observe and analyze"). This is not in the initial implementation but the data model supports it.

---

## Configuration Summary

All enterprise features are opt-in. A default Gasoline installation behaves identically to today — no audit log, no TTL, no redaction, no auth. Enterprise features activate via explicit configuration.

### Minimal Enterprise Setup

```bash
gasoline --audit-file=./audit.jsonl --api-key=$GASOLINE_KEY --ttl=1h
```

### Full Compliance Setup

```bash
gasoline \
  --compliance-preset=soc2 \
  --audit-file=/var/log/gasoline/audit.jsonl \
  --api-key=$GASOLINE_KEY \
  --export-dir=/var/log/gasoline/exports/ \
  --redaction-config=./redaction-rules.json \
  --config=./gasoline.toml
```

---

## Performance Constraints

Enterprise features must not violate existing SLOs:
- Audit logging adds < 0.1ms per tool call (append to in-memory buffer)
- Redaction adds < 5ms worst case per tool response (regex scan of string fields)
- TTL eviction runs on a background timer, never blocking request processing
- Rate limit checks add < 0.01ms per tool call (atomic counter read)
- Config file parsing happens once at startup (not on every request)
- Health metrics are computed on-demand when the tool is called (no background polling overhead)

Total overhead for all Tier 1-3 features combined: < 6ms per tool call worst case.

---

## Test Scenarios

### Tier 1
- Verify audit log records every tool call with correct metadata
- Verify client identity is extracted from MCP `initialize` request
- Verify session IDs are unique across concurrent connections
- Verify redaction events are logged without storing redacted content
- Verify audit log respects buffer size limit (oldest entries evicted)
- Verify file persistence writes valid JSON Lines
- Verify `get_audit_log` filters work (time range, tool name, session)

### Tier 2
- Verify TTL eviction removes entries older than configured TTL
- Verify TTL reduction takes immediate effect
- Verify compliance presets set all expected values
- Verify preset values can be overridden by explicit flags
- Verify export produces valid JSON Lines with all buffer types
- Verify redaction patterns match and replace correctly
- Verify redacted responses don't contain original sensitive data
- Verify custom redaction patterns load from config file
- Verify Luhn validation on credit card pattern (reduces false positives)

### Tier 3
- Verify API key auth rejects requests without key
- Verify API key auth accepts requests with correct key
- Verify MCP stdio connections bypass API key auth
- Verify per-tool rate limits enforce configured thresholds
- Verify rate limit error response includes retry hint
- Verify configurable thresholds override defaults
- Verify config file parsing with all supported values
- Verify priority order: CLI > env > config file > defaults
- Verify health metrics report accurate values for all categories

### Tier 4
- Verify project isolation (data in project A not visible in project B)
- Verify per-project memory caps are enforced
- Verify read-only mode blocks all mutation operations
- Verify read-only mode allows all read/analyze/generate operations
- Verify tool allowlisting hides tools from `tools/list`
- Verify hidden tools return "method not found" on direct call
- Verify blocklist hides specified tools
- Verify default behavior (no enterprise config) is unchanged

---

## Zero-Dependency Constraint

All enterprise features are implemented using Go stdlib only. Specific implications:
- Regex: `regexp` package (no PCRE, no lookahead — patterns must be RE2-compatible)
- JSON: `encoding/json` for config parsing and export
- File I/O: `os` package for audit log persistence
- Crypto: `crypto/rand` for session ID generation
- Config: custom TOML parser (simple key-value, no nested tables beyond one level) or flag-based only
- No external auth libraries — API key is a simple string comparison

The TOML config file support is the only architectural decision that may benefit from a library. If RE2-compatible TOML parsing proves too complex with stdlib alone, fall back to a simpler INI-style format or JSON config file. Never add a dependency.

---

## Migration & Backward Compatibility

All features are additive. No existing behavior changes without explicit opt-in. Specifically:
- No audit log is written unless `--audit-file` is set
- No TTL is enforced unless `--ttl` is set
- No redaction occurs unless patterns are configured
- No auth is required unless `--api-key` is set
- No rate limits apply unless configured
- All tools remain available unless allowlisting is configured
- The extension works unchanged — new auth header is only sent if configured

The MCP protocol version does not change. New tools (`get_audit_log`, `get_health`, `export_data`) are added; no existing tools change their response format.
