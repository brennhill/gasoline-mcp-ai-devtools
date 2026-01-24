# Roadmap

Thesis: AI reads browser state → AI writes code → human verifies. Every feature is judged by whether it makes the AI smarter, the feedback loop tighter, or human verification easier.

## Completed

- [x] **Query result cleanup** — Delete on read + periodic expiry sweep.
- [x] **Connection duration** — `"duration": "5m02s"` in `get_websocket_status`.
- [x] **Message rate calculation** — Rolling 5-second window, recalculated every second.
- [x] **Age formatting** — `"age": "0.2s"` in last message preview.
- [x] **A11y audit result caching** — 30s TTL, force_refresh, concurrent dedup, navigation invalidation.
- [x] **Compressed state diffs** — `get_changes_since` with checkpoint IDs for token-efficient polling.
- [x] **Performance budget monitor** — `check_performance` with snapshots, baselines, FCP/LCP/CLS regression detection.
- [x] **File split refactor** — Monolithic v4.go split into 8 domain files.
- [x] **Multi-agent infrastructure** — Lefthook hooks, advisory lock file, branching strategy.
- [x] **Schema variant tracking** — WebSocket connections track message variants (e.g., "message 89%, typing 8%").
- [x] **WebSocket payload spec compliance** — Added `type: "websocket"` field to all WS event payloads per spec log format.
- [x] **Contract-first testing process** — Shape tests enforce spec field presence before behavioral tests. Rule codified in `.claude/docs/tdd.md`.
- [x] **Rate limiting / circuit breaker** — Server 429 at >1000 events/sec; extension exponential-backoff (100→500→2000ms) with 5-failure circuit break, 30s pause. Files: `rate_limit.go`, `extension-tests/rate-limit.test.js`.
- [x] **Memory enforcement** — Three thresholds (20MB soft, 50MB hard, 100MB critical) with progressive eviction. Extension memory estimation + pressure checking. Files: `memory.go`, `extension-tests/memory.test.js`.
- [x] **Interception deferral** — Phase 1 (immediate: API, PerfObservers) / Phase 2 (deferred: console, fetch, WS, errors, actions). Trigger: load + 100ms, 10s timeout fallback. File: `inject.js`, `extension-tests/interception-deferral.test.js`.

---

## P1: AI Intelligence (the AI gets smarter)

These make the AI a competent collaborator instead of a stateless tool.

Specs: `docs/ai-first/tech-spec-persistent-memory.md`, `docs/ai-first/tech-spec-noise-filtering.md`

- [x] **Persistent memory** — Cross-session storage (`session_store`, `load_session_context`). Baselines, noise rules, API schemas, error history survive restarts. File: `ai_persistence.go`.
- [x] **Noise filtering** — Built-in heuristics + agent-configurable rules (`configure_noise`, `dismiss_noise`). Auto-detection from buffer frequency analysis. File: `ai_noise.go`.
- [x] **API schema inference** — Auto-detect endpoints, request/response shapes, auth patterns from network traffic. The AI understands your API without documentation. File: `api_schema.go`. Spec: `docs/ai-first/tech-spec-api-schema.md`.

## P2: Feedback Loop (AI learns from its own changes)

The core thesis loop: AI changes code → Gasoline detects impact → AI adjusts.

- [x] **Push notification on regression** — AI is alerted without polling. After a reload, if baselines are exceeded, the server signals "your last reload regressed load by 800ms" via a status field in `get_changes_since`.
- [x] **Causal diffing** — Compare resource lists (scripts, stylesheets, fonts) between baseline and current. Report what changed: "3 new scripts totaling 400KB added since baseline". File: `performance.go`.
- [x] **Workflow integration** — Auto-run `check_performance` after each AI-initiated code change. Include performance delta in PR summaries. Integrates with codegen/reproduction scripts.

## P3: Measurement & Verification (human audits AI work)

Help the human verify what the AI observed and decided.

- [x] **Web vitals capture** — FCP, LCP, CLS, INP via extension. Standardized performance data the AI can reference. TDD stubs exist in `extension-tests/web-vitals.test.js`.
- [x] **SARIF export** — `export_sarif` MCP tool. A11y audit results → GitHub Code Scanning format. Human sees AI-detected issues in PR review. File: `export_sarif.go`.
- [x] **HAR export** — `export_har` MCP tool. Network bodies + timing → HTTP Archive format. Human can inspect in Charles Proxy / DevTools. File: `export_har.go`.

## Next: AI-First Tool Surface (architectural refactor)

The tool surface has grown to 24 tools — past the threshold where AI models degrade in tool selection accuracy. This refactor consolidates to 5 composite tools, adds dynamic exposure, and introduces push-based alerts. This is the highest-priority work because it directly affects every AI consumer's ability to use Gasoline effectively.

### Phase 1: Composite Tools (24 → 5)

Collapse granular tools into smart composite tools with mode parameters.

- [x] **`observe`** — Replaces: `get_browser_errors`, `get_browser_logs`, `get_websocket_events`, `get_websocket_status`, `get_network_bodies`, `get_enhanced_actions`, `get_web_vitals`, `get_page_info`. Mode parameter: `what: "errors"|"logs"|"network"|"websocket_events"|"websocket_status"|"actions"|"vitals"|"page"`.
- [x] **`analyze`** — Replaces: `check_performance`, `get_api_schema`, `run_accessibility_audit`, `get_changes_since`, `get_session_timeline`. Mode parameter: `target: "performance"|"api"|"accessibility"|"changes"|"timeline"`.
- [x] **`generate`** — Replaces: `get_reproduction_script`, `generate_test`, `generate_pr_summary`, `export_sarif`, `export_har`. Mode parameter: `format: "reproduction"|"test"|"pr_summary"|"sarif"|"har"`.
- [x] **`configure`** — Replaces: `session_store`, `load_session_context`, `configure_noise`, `dismiss_noise`, `clear_browser_logs`. Mode parameter: `action: "store"|"load"|"noise_rule"|"dismiss"|"clear"`.
- [x] **`query_dom`** — Stays standalone (interactive, requires CSS selector input).

### Phase 2: Metadata Annotations

Tools list includes `_meta.data_counts` so AI consumers know what data is available without changing the schema. Dynamic tool exposure was rejected (AI models work best with stable schemas; MCP clients cache `tools/list` and don't re-poll reliably).

- [x] **`_meta` data counts** — Each tool in `tools/list` includes `_meta.data_counts` showing current buffer sizes per mode. AI uses this as a hint for which modes to call first. File: `tools.go`.
- ~~**State-aware tool list**~~ — Rejected: dynamic enum filtering confuses AI models.
- ~~**Progressive disclosure**~~ — Rejected: same reason (schema instability).

### Phase 3: Push-Based Alerts

Server attaches unsolicited context to responses instead of requiring separate tool calls.

- [x] **Alert piggyback** — Every `observe` response includes an alerts block with regressions, anomalies, and noise rule triggers detected since last call. Alerts are drained after delivery. File: `alerts.go`.
- [x] **Situation synthesis** — Server-side triage: deduplication, priority ordering (error > warning > info), correlation (regression + anomaly within 5s → compound alert), summary prefix (4+ alerts). File: `alerts.go`.
- [x] **CI/CD webhook receiver** — `POST /ci-result` endpoint. Build failures, test results, and deploy status surface through alerts without a dedicated tool. Idempotent, capped at 10 results. File: `alerts.go`, route in `main.go`.
- [x] **Anomaly detection** — Error frequency spike (>3x rolling average in 10s window) generates anomaly alert. File: `alerts.go`.

### Phase 4: Deeper Browser Intelligence

Server-side intelligence that makes the AI smarter about browser state it can't get any other way.

- [x] **Anomaly detection** — Error frequency spike detection (>3x rolling average) surfaces in alerts.
- [ ] **AI capture control** — AI adjusts extension settings via `configure(action: "capture", settings: {...})`. Session-scoped (resets on restart). Settings: log_level, ws_mode, network_bodies, screenshot_on_error, action_replay. Changes emit info-level alert + structured audit log (`~/.gasoline/audit.jsonl`, rotating JSONL, fluentbit-compatible).
- [ ] **Error clustering** — Group related errors across sessions: "These 4 stack traces share a common root cause." Reduces noise, surfaces root causes.
- [ ] **Cross-session temporal graph** — Persistent memory v2: not just key-value but "error X first appeared at time T, correlated with change Y, resolved by fix Z." Browser state history that survives context resets.
- [ ] **SPA route measurement** — `pushState`/`popstate` observation, per-route time-to-interactive. Real browser state the AI can't get without instrumentation.
- [ ] **Budget thresholds as config** — Developer sets "load < 2s, bundle < 500KB" in `.gasoline.json`, AI enforces. Clear pass/fail criteria tighten the feedback loop.

### Files Modified
- `cmd/dev-console/tools.go` — Composite dispatchers, `_meta` data counts, alert piggyback on observe
- `cmd/dev-console/main.go` — `MCPTool.Meta` field, CI webhook route
- `cmd/dev-console/alerts.go` — New: Alert struct, buffer, dedup, correlation, CI webhook, anomaly detection
- `cmd/dev-console/api_schema.go` — `EndpointCount()` method
- `cmd/dev-console/composite_tools_test.go` — Phase 2 metadata tests
- `cmd/dev-console/alerts_test.go` — New: 16 Phase 3 tests
- `cmd/dev-console/testdata/mcp-tools-list.golden.json` — Updated with `_meta` fields
- `docs/ai-first/tech-spec-dynamic-exposure.md` — Phase 2 spec
- `docs/ai-first/tech-spec-push-alerts.md` — Phase 3 spec

---

## Phase 5: Security & Analysis

Analysis and security hardening tools that operate on data Gasoline already captures. No new browser capture mechanisms needed.

### Enterprise Adoption Blockers (ship first)

Without these, security teams will block the tool in enterprise environments.

- [ ] **Configurable Redaction Patterns** — User-defined regex patterns for redacting sensitive data from tool responses (tokens, SSNs, card numbers, custom patterns). Security teams won't approve a tool that passes raw tokens/PII to AI context.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 2.4
  - Branch: `feature/redaction-patterns`

- [ ] **Tool Invocation Log (`get_audit_log`)** — Ring-buffer log of every MCP tool call with timestamp, tool name, parameters, response size, duration, and client identity.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 1.1
  - Branch: `feature/audit-log`

- [ ] **Client Identification** — Identify which AI client (Claude Code, Cursor, Windsurf, etc.) is connected via MCP, recorded on every audit entry.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 1.2
  - Branch: `feature/client-id`

- [ ] **Session ID Assignment** — Unique session ID per MCP connection, correlating all tool calls within a session.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 1.3
  - Branch: `feature/session-id`

- [ ] **TTL-Based Retention** — Configurable time-to-live for all captured data; buffers automatically evict entries older than TTL.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 2.1
  - Branch: `feature/ttl-retention`

- [ ] **API Key Authentication** — Optional shared-secret authentication for the HTTP API, preventing unauthorized tools from connecting.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 3.1
  - Branch: `feature/api-key-auth`

### Market Differentiators (ship second)

Features no other tool provides — these justify Gasoline's existence beyond "just another MCP server."

- [ ] **CSP Generator (`generate_csp`)** — Generate a Content-Security-Policy from observed resource origins. No other tool generates CSP passively from browser observation.
  - Spec: ai-first/tech-spec-security-hardening.md § Tool 1
  - Branch: `feature/generate-csp`

- [ ] **Third-Party Risk Audit (`audit_third_parties`)** — Map all external domains, classify by risk level, domain reputation scoring, enterprise custom lists. Bundled reputation lists (Disconnect.me, Tranco 10K, curated CDNs), domain heuristics, optional external enrichment (RDAP, CT, Safe Browsing).
  - Spec: ai-first/tech-spec-security-hardening.md § Tool 2
  - Branch: `feature/audit-third-parties`

- [ ] **Security Scanner (`security_audit`)** — Detect exposed credentials, missing auth, PII leaks, insecure transport, missing security headers (incl. CSP analysis), insecure cookies. Proactive: context streaming pushes alerts for credential exposure, CSP violations, insecure cookies, and missing security headers as they are observed.
  - Spec: v6-specification.md § Feature 1 (checks 1-6)
  - Branch: `feature/security-audit`

- [ ] **Security Regression Detection (`diff_security`)** — Compare security posture before/after code changes. Removed headers, lost cookie flags, dropped auth requirements flagged with severity.
  - Spec: ai-first/tech-spec-security-hardening.md § Tool 3
  - Branch: `feature/diff-security`
  - Depends on: `diff_sessions`

### Operational Intelligence (ship third)

- [ ] **API Contract Validation (`validate_api`)** — Track response shapes, detect contract violations.
  - Spec: v6-specification.md § Feature 4
  - Branch: `feature/validate-api`

- [ ] **Configuration Profiles** — Named configuration bundles (short-lived, restricted, paranoid) that set TTL, redaction, and rate limits to common security postures.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 2.2
  - Branch: `feature/config-profiles`

- [ ] **Per-Tool Rate Limits** — Configurable rate limits per MCP tool (e.g., `query_dom` limited to 10/min) to prevent runaway AI loops.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 3.2
  - Branch: `feature/per-tool-rate-limits`

- [ ] **Data Export** — MCP tool to export current buffer state and audit entries as JSON Lines for offline retention.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 2.3
  - Branch: `feature/data-export`

- [ ] **Verification Loop (`verify_fix`)** — Before/after session comparison for fix verification.
  - Spec: v6-specification.md § Feature 2
  - Branch: `feature/verify-fix`

- [ ] **Session Comparison (`diff_sessions`)** — Named snapshot storage and comparison.
  - Spec: v6-specification.md § Feature 3
  - Branch: `feature/diff-sessions`

- [ ] **Performance Budget Monitor (`check_performance`)** — Baseline regression detection with budget thresholds.
  - Spec: performance-budget-spec.md
  - Branch: `feature/performance-budget-monitor`

### Scale & Polish (ship last)

- [ ] **Context Streaming** — Push significant events to AI via MCP notifications. Depends on MCP client notification support maturing.
  - Spec: v6-specification.md § Feature 5
  - Branch: `feature/context-streaming`

- [ ] **Test Generation v2 (`generate_test`)** — DOM assertions, fixtures, visual snapshots.
  - Spec: generate-test-v2.md
  - Branch: `feature/generate-test-v2`

- [ ] **SRI Hash Generator (`generate_sri`)** — Generate Subresource Integrity hashes for third-party resources.
  - Spec: ai-first/tech-spec-security-hardening.md § Tool 4
  - Branch: `feature/generate-sri`

- [ ] **Redaction Audit Log** — Log every time data is redacted (what pattern matched, what field, what tool response), without storing the redacted content itself.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 1.4
  - Branch: `feature/redaction-audit`

- [ ] **Configurable Thresholds** — All server limits (buffer sizes, memory caps, rate limits) configurable via CLI flags or config file.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 3.3
  - Branch: `feature/configurable-thresholds`

- [ ] **Health & SLA Metrics (`get_health`)** — MCP tool exposing server uptime, buffer utilization, memory usage, request counts, and error rates.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 3.4
  - Branch: `feature/health-metrics`

- [ ] **Project Isolation** — Multiple isolated capture contexts (projects) on a single server, each with independent buffers and configuration.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 4.1
  - Branch: `feature/project-isolation`

- [ ] **Read-Only Mode** — Server mode that accepts capture data but disables all mutation tools (clear, dismiss, checkpoint delete).
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 4.2
  - Branch: `feature/read-only-mode`

- [ ] **Tool Allowlisting** — Configuration to restrict which MCP tools are available, hiding sensitive tools from untrusted clients.
  - Spec: ai-first/tech-spec-enterprise-audit.md § Tier 4.3
  - Branch: `feature/tool-allowlist`

### Internal Quality: Fuzz Tests

Go fuzz tests for protocol parsing and input handling. Not user-facing — improves Gasoline's own resilience.

- [ ] **FuzzJSONRPCParse** — Fuzz the MCP JSON-RPC message parser. Goal: No panics, no unbounded allocations.
- [ ] **FuzzHTTPBodyParse** — Fuzz the `/logs` and `/network-body` HTTP endpoints. Goal: All malformed bodies return 400, never panic.
- [ ] **FuzzSecurityPatterns** — Fuzz the credential/PII regex patterns. Goal: No catastrophic backtracking.
- [ ] **FuzzWebSocketFrame** — Fuzz WebSocket message handling. Goal: Malformed frames handled gracefully.
- [ ] **FuzzNetworkBodyStorage** — Fuzz large/malformed network body storage. Goal: Memory limits enforced, no OOM.

**When to run:** Fuzz tests run in CI with `-fuzztime=30s`. Extended fuzzing (`-fuzztime=5m`) runs as part of the release PR skill.

### Dependencies

- Enterprise Adoption Blockers can be implemented in parallel (no cross-feature deps)
- Client ID + Session ID are prerequisites for meaningful audit logs
- Configuration Profiles depend on TTL + Redaction being implemented first
- `diff_security` depends on `diff_sessions`
- Context Streaming depends on MCP notification support in clients

---

## Tech Debt: Consolidation (code quality)

Codebase grew feature-by-feature without periodic pattern extraction. Locally consistent, globally drifting. Two parallel streams since Go and Extension files don't overlap.

### Stream A: Go Server

Sequential (each touches overlapping files):

- [x] **MCP response helper** — Extract `mcpTextResponse(text)` to eliminate 25+ identical response constructions across tool handlers. File: `tools.go`.
- [x] **Decompose Capture struct** — Extract `A11yCache`, `PerformanceStore`, `MemoryState` from the God Object. Files: `types.go`, all domain files.
- [x] **Remove Go dead code** — Delete `RecordEventReceived()`, legacy `eventCount`/`rateResetTime` fields, unused `initialized` field. Files: `queries.go`, `types.go`, `main.go`.
- [x] **Add request body limits** — `http.MaxBytesReader` on all POST handlers to prevent unbounded reads. Files: `websocket.go`, `network.go`, `actions.go`, `performance.go`, `queries.go`, `main.go`.
- [x] **Deduplicate utilities** — Consolidate `extractPath`/`ExtractURLPath`, `removeFromSlice`. File: `helpers.go`.

### Stream B: Extension + Tests

Sequential (message naming cascades into test assertions):

- [x] **Normalize message naming** — Replace `DEV_CONSOLE_*` with `GASOLINE_*` throughout. Files: `inject.js`, `content.js`, `background.js`, docs.
- [x] **Fix truncateArg** — Replace dangerous `JSON.parse(sliced + '"} [truncated]')` with safe truncation. File: `background.js`.
- [x] **Replace setInterval with chrome.alarms** — `setInterval` is unreliable in MV3 service workers. File: `background.js`.
- [x] **Remove JS dead code** — Delete `_TEXT_CONTENT_TYPES`, no-op references in popup.js. Files: `inject.js`, `popup.js`.
- [x] **Shared test infrastructure** — Extract `createMockWindow()`, `createMockChrome()`, `createMockDocument()` into `extension-tests/helpers.js`.

## P5: Nice-to-have (someday, maybe)

Useful but not thesis-critical. Only if there's nothing higher to work on.

- [ ] **Binary format detection** — Identify protobuf/MessagePack via magic bytes instead of hex dump.
- [ ] **Network body E2E tests** — E2E coverage for body capture (large bodies, binary, header sanitization).
- [ ] **Reproduction script enhancements** — Screenshot insertion, data fixture generation, visual assertions.

## Discarded

These don't serve the thesis. The AI IS the interface — exporting to other tools assumes a non-AI workflow. Features that duplicate what the AI can already do natively, or that aren't browser state, don't belong in Gasoline.

- ~~**Spec ingestion**~~ — AI already reads markdown files natively. Gasoline captures browser state, not documents.
- ~~**Git context**~~ — AI already has `git log`, `git blame`, `gh pr`. Not browser state.
- ~~**Production error bridge**~~ — Not browser state. Requires external infra pointing at localhost. AI can query Sentry/Datadog APIs directly.
- ~~**Predictive warnings**~~ — Gasoline sees browser errors, not code patterns. Can't meaningfully say "this code pattern caused issues" without seeing code.
- ~~**Agent-to-agent channel**~~ — MCP already shares server state across clients. Premature abstraction for a workflow that doesn't exist yet.
- ~~**Prometheus metrics**~~ — Enterprise observability for a dev-time tool. Nobody runs Grafana dashboards for their debugger.
- ~~**Webhook/event stream**~~ — Slack alerts for localhost console errors. The AI is already watching in real-time.
- ~~**Sentry-compatible export**~~ — Sentry is for production. Dev-time errors are transient and the AI handles them directly.
- ~~**MSW export**~~ — Generates mock handlers. Useful developer tool, but the AI already has the raw data.
- ~~**OpenAPI skeleton**~~ — Exports observed endpoints to YAML. Redundant when API schema inference gives the AI this directly.
- ~~**VS Code extension**~~ — Status sidebar. The AI is the interface, not a widget.
- ~~**Playwright test export**~~ — `generate_test` already works. Prettier output isn't a new capability.
- ~~**Extension health dashboard**~~ — Meta-monitoring of the tool itself.
- ~~**Selective capture profiles**~~ — Pre-configs. Convenience, not capability.

---

## Parallel Assignment Guide

### Phase 4 Features

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| AI capture control | `tools.go`, `main.go`, `audit.go` (new) | All others |
| Error clustering | `clustering.go` (new) | All others |
| Cross-session temporal graph | `ai_persistence.go` (extend) | All others |
| SPA route measurement | `inject.js`, `performance.go` | All except Budgets |
| Budget thresholds | `performance.go`, config loader (new) | All except SPA routes |

### Phase 5: Enterprise Adoption Blockers

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| Redaction patterns | `redaction.go` (new), `tools.go` | Audit log, TTL, API key |
| Audit log | `audit.go` (new), `tools.go` | Redaction, TTL, API key |
| Client identification | `audit.go`, `main.go` | All (pairs with Session ID) |
| Session ID | `audit.go`, `main.go` | All (pairs with Client ID) |
| TTL retention | `memory.go` (extend), `types.go` | Redaction, Audit log, API key |
| API key auth | `main.go`, `auth.go` (new) | Redaction, Audit log, TTL |

### Phase 5: Market Differentiators

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| CSP generator | `security_csp.go` (new), `tools.go` | All others |
| Third-party audit | `security_thirdparty.go` (new), `tools.go` | All others |
| Security scanner | `security.go` (new), `tools.go` | All except diff_security |
| Security regression | `security_diff.go` (new), `tools.go` | Needs diff_sessions first |

### Phase 5: Operational Intelligence

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| API contract validation | `contracts.go` (new), `tools.go` | All others |
| Configuration profiles | `config.go` (new), `main.go` | Needs TTL + Redaction first |
| Per-tool rate limits | `rate_limit.go` (extend), `tools.go` | All others |
| Data export | `export.go` (new), `tools.go` | All others |
| Verify fix | `verify.go` (new), `tools.go` | All others |
| Session comparison | `sessions.go` (new), `tools.go` | All others |
| Performance budget | `performance.go` (extend), `tools.go` | All others |

### Phase 5: Scale & Polish

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| Context streaming | `streaming.go` (new), `main.go` | All others |
| Test generation v2 | `codegen.go` (extend) | All others |
| SRI generator | `security_sri.go` (new), `tools.go` | All others |
| Redaction audit log | `audit.go` (extend) | Needs Redaction + Audit log first |
| Configurable thresholds | `config.go` (extend), `main.go` | All others |
| Health metrics | `health.go` (new), `tools.go` | All others |
| Project isolation | `projects.go` (new), all domain files | Standalone (large scope) |
| Read-only mode | `main.go`, `tools.go` | All others |
| Tool allowlisting | `tools.go`, `config.go` | All others |
