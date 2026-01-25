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
- [x] **Contract-first testing process** — Shape tests enforce spec field presence before behavioral tests. Rule codified in `.claude/docs/testing.md`.
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

### Suggestions from Gemini
Feature Area,Current Standard (MCP Basics),"Gasoline ""Supercharged"" Future"
Vision,See DOM / Accessibility Tree,Visual Screenshots + Network Logs + Console Errors
Action,Click / Type,Inject State (Cookies) + Mock API Responses
Output,"""I finished the task""","""Here is a Playwright test to prevent this bug from returning"""
Memory,Stateless (Context window only),Semantic Cache (Remembering how to navigate your specific app)

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

## AI-First Tool Surface (complete)

Consolidated 24 granular tools into 5 composite tools with mode parameters, metadata annotations, and push-based alerts.

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

---

## Security & Analysis (complete)

- [x] **Configurable Redaction Patterns** — User-defined regex patterns for redacting sensitive data from tool responses. File: `redaction.go`.
- [x] **Tool Invocation Log (`get_audit_log`)** — Ring-buffer log of every MCP tool call. File: `audit.go`.
- [x] **Client Identification** — AI client identification recorded on every audit entry. File: `audit.go`.
- [x] **Session ID Assignment** — Unique session ID per MCP connection. File: `audit.go`.
- [x] **TTL-Based Retention** — Time-to-live eviction for all captured data buffers. File: `memory.go`.
- [x] **CSP Generator (`generate_csp`)** — CSP from observed resource origins. File: `csp.go`.
- [x] **Third-Party Risk Audit (`audit_third_parties`)** — Risk classification, DGA detection, CDN recognition. File: `thirdparty.go`.
- [x] **Security Scanner (`security_audit`)** — Credentials, PII, headers, cookies, transport, auth checks. File: `security.go`.
- [x] **Security Regression Detection (`diff_security`)** — Snapshot/compare security posture. File: `security_diff.go`.

### Internal Quality: Fuzz Tests (complete)

- [x] **FuzzMCPRequest** — MCP JSON-RPC message parser.
- [x] **FuzzPostLogs** — `/logs` HTTP endpoint.
- [x] **FuzzNetworkBodies** — Network body ingest.
- [x] **FuzzWebSocketEvents** — WebSocket event handling.
- [x] **FuzzEnhancedActions** — Action ingest including password redaction.
- [x] **FuzzValidateLogEntry** — Log entry validation.
- [x] **FuzzScreenshotEndpoint** — Screenshot endpoint with malformed multipart.
- [x] **FuzzSecurityPatterns** — Credential/PII regex patterns. No catastrophic backtracking.

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

## P4: Bi-Directional Actions (capture → interact)

Gasoline is capture-only by design. These features break that rule intentionally — they let the AI act on the page, not just observe it. Useful for human verification and faster reproduction.

- [ ] **Highlight element (`highlight_element`)** — Inject a red overlay on a DOM element so AI can point at things on screen. Helps human verify "this is the button I'm talking about." Extension injects `#gasoline-highlighter` div, positions via `getBoundingClientRect()`. MCP tool accepts selector + duration.
- [ ] **Browser state snapshots (`manage_state`)** — Save/restore `localStorage`, `sessionStorage`, `document.cookie`. Lets AI checkpoint "cart full" state and restore it instantly instead of clicking through the flow again. MCP tool with `action: "save"|"load"|"list"` + `snapshot_name`.
- [ ] **Execute JavaScript (`execute_javascript`)** — Run arbitrary JS in browser context and return JSON-serialized result. Lets AI inspect Redux/Zustand stores, check globals (`window.__NEXT_DATA__`), test expressions before writing code. Localhost-only (already enforced). MCP tool accepts `script` string, returns serialized result or error.

## P5: Nice-to-have (someday, maybe)

Useful but not thesis-critical. Only if there's nothing higher to work on.

- [ ] **Binary format detection** — Identify protobuf/MessagePack via magic bytes instead of hex dump.
- [ ] **Network body E2E tests** — E2E coverage for body capture (large bodies, binary, header sanitization).
- [ ] **Reproduction script enhancements** — Screenshot insertion, data fixture generation, visual assertions.

## Discarded

These don't serve the thesis. The AI IS the interface — exporting to other tools assumes a non-AI workflow. Features that duplicate what the AI can already do natively, aren't browser state, or are redundant with existing capabilities don't belong in Gasoline.

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
- ~~**Error clustering**~~ — The AI itself is better at pattern-matching across stack traces than a zero-dep Go heuristic. Solving the problem at the wrong layer.
- ~~**Cross-session temporal graph**~~ — Over-engineered. Persistent memory v1 (key-value) covers 90% of the need. Developers fix bugs within a session; a full temporal graph adds significant complexity for marginal value.
- ~~**Verification loop (`verify_fix`)**~~ — Redundant. Checkpoints + `get_changes_since` + `check_performance` already give the AI before/after comparison without a dedicated tool.
- ~~**Session comparison (`diff_sessions`)**~~ — Abstraction layer with no remaining consumers. `diff_security` can use the existing checkpoint system.
- ~~**Performance Budget Monitor**~~ — Already implemented (checked off in P2). Duplicate roadmap entry.
- ~~**API Contract Validation (`validate_api`)**~~ — Redundant with API schema inference (`api_schema.go`). Validation is a natural extension of the existing schema, not a separate tool.
- ~~**Configuration Profiles**~~ — Over-abstraction. Named bundles wrapping TTL + redaction settings add indirection for a dev tool where most users have one configuration.
- ~~**API Key Authentication**~~ — Security theater for localhost. The HTTP API is only accessed by the local browser extension; the MCP connection is stdio (process-isolated). The actual threat model is thin.
- ~~**Context Streaming**~~ — Externally blocked. MCP clients don't reliably support push notifications yet. Can't ship what the ecosystem doesn't support.
- ~~**Health & SLA Metrics (`get_health`)**~~ — Same concept as the discarded "Extension health dashboard" repackaged as an MCP tool. Nobody needs SLA metrics for a localhost debugger.
- ~~**Project Isolation**~~ — High complexity (touches every domain file) for a rare use case (debugging multiple apps on one Gasoline instance). Developers use separate terminal sessions.
- ~~**AI capture control**~~ — Over-engineering. The extension already captures everything useful; selectively toggling capture from AI adds complexity for marginal value.
- ~~**SPA route measurement**~~ — pushState observation is fragile across frameworks and the AI can already detect route changes from network/console patterns.
- ~~**Budget thresholds as config**~~ — The AI already compares against baselines via check_performance. A static config file adds indirection.
- ~~**Per-tool rate limits**~~ — Enterprise checkbox for localhost. The existing global rate limiter prevents runaway loops.
- ~~**Data export**~~ — JSONL export for a localhost dev tool. The AI already has the data via MCP tools.
- ~~**Test generation v2**~~ — DOM assertions and visual snapshots. Current test generation covers the 90% case.
- ~~**SRI hash generator**~~ — Niche security feature. CDN integrity is important but rarely the debugging bottleneck.
- ~~**Redaction audit log**~~ — Logging what was redacted (without content) is security theater for a localhost tool.
- ~~**Configurable thresholds**~~ — CLI flags for buffer sizes. Hardcoded defaults work for the dev-tool use case.
- ~~**Read-only mode**~~ — Unnecessary access control for a single-user localhost tool.
- ~~**Tool allowlisting**~~ — Same reasoning as read-only mode. No untrusted clients on localhost.

