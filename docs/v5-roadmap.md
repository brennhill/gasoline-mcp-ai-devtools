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

### Phase 4: External Signal Ingestion

Broader context sources beyond the browser, all surfacing through the existing 5-tool interface.

- [ ] **Spec ingestion** — `configure(action: "ingest_spec")` parses markdown specs into structured requirements. AI checks runtime behavior against spec.
- [ ] **Production error bridge** — Sentry/Datadog webhook receiver. Production errors correlated with code the AI is modifying surface in `observe` alerts.
- [ ] **Git context** — `analyze(target: "git")` shows recent commits, blame info, and PR history for files involved in current errors.
- [ ] **Cross-session temporal graph** — Persistent memory v2: not just key-value but "error X first appeared at time T, correlated with change Y, resolved by fix Z."

### Phase 5: Autonomous AI Behavior

Server-side intelligence that acts without the AI needing to poll.

- [x] **Anomaly detection** — Error frequency spike detection (>3x rolling average) surfaces in alerts. Future: background thread for metrics beyond error counts.
- [ ] **Error clustering** — Group related errors across sessions: "These 4 stack traces share a common root cause."
- [ ] **Predictive warnings** — Pattern matching against known failure modes: "This code pattern caused issues in 3 prior sessions."
- [ ] **Agent-to-agent channel** — `configure(action: "post_observation")` / `observe(what: "observations")` for multi-agent handoff.

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

## P4: Nice-to-have (someday, maybe)

Useful but not thesis-critical. Only if there's nothing higher to work on.

- [ ] **SPA route measurement** — `pushState`/`popstate` observation, per-route time-to-interactive.
- [ ] **Budget thresholds as config** — Developer sets "load < 2s, bundle < 500KB" in `.gasoline.json`, AI enforces.
- [ ] **Binary format detection** — Identify protobuf/MessagePack via magic bytes instead of hex dump.
- [ ] **Network body E2E tests** — E2E coverage for body capture (large bodies, binary, header sanitization).
- [ ] **Reproduction script enhancements** — Screenshot insertion, data fixture generation, visual assertions.

## Discarded

These don't serve the thesis. The AI IS the interface — exporting to other tools assumes a non-AI workflow.

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

### Feature work (P1)

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| Persistent memory | `ai_persistence.go` (new) | Noise, API schema |
| Noise filtering | `ai_noise.go` (new) | Persistent, API schema |
| API schema inference | `api_schema.go` (new) | Persistent, Noise |
