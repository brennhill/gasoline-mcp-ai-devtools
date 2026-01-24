# Roadmap

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

## Next: Reliability (ship-blockers)

Specs: `docs/ai-first/tech-spec-rate-limiting.md`, `docs/ai-first/tech-spec-memory-enforcement.md`

- [ ] **Rate limiting / circuit breaker** — Server responds 429 at >1000 events/sec; extension exponential-backoff (100ms→500ms→2s) with 5-failure circuit break. File: `rate_limit.go`.
- [ ] **Memory enforcement** — Three thresholds (20MB soft, 50MB hard, 100MB critical) with progressive eviction. Minimal mode at critical. File: `memory.go`.
- [ ] **Interception deferral** — Defer v4 intercepts (WS constructor, fetch body capture) until after `load` event + 100ms. Currently `install()` runs immediately.

## Next: AI Features (product differentiation)

Specs: `docs/ai-first/tech-spec-persistent-memory.md`, `docs/ai-first/tech-spec-noise-filtering.md`

- [ ] **Persistent memory** — Cross-session storage (`session_store`, `load_session_context`). Baselines, noise rules, API schemas, error history survive restarts. File: `ai_persistence.go`.
- [ ] **Noise filtering** — Built-in heuristics + agent-configurable rules (`configure_noise`, `dismiss_noise`). Auto-detection from buffer frequency analysis. File: `ai_noise.go`.

## Next: Web Vitals (spec-complete capture)

- [ ] **Web vitals capture** — Implement `startWebVitalsCapture`, `resetWebVitals`, `getWebVitals`, `stopWebVitalsCapture`, `sendWebVitals`. TDD stubs exist in `extension-tests/web-vitals.test.js` (10 tests, all currently failing).

## Performance Intelligence (extends check_performance)

Building on the existing performance budget monitor to provide proactive, contextual performance insights.

- [ ] **Push notification on regression** — AI is alerted without polling. After a reload, if baselines are exceeded, the server proactively signals "your last reload regressed load by 800ms". Requires MCP notifications or a status field in `get_changes_since`.
- [ ] **SPA route measurement** — Observe `pushState`/`replaceState` and `popstate` events. Measure time-to-interactive per route transition. Extend performance snapshots to include per-route metrics.
- [ ] **Causal diffing** — Compare resource lists (scripts, stylesheets, fonts) between baseline and current snapshot. Report what changed: "3 new scripts totaling 400KB added since baseline". File: `performance.go`.
- [ ] **Workflow integration** — Auto-run `check_performance` after each AI-initiated code change. Include performance delta in PR summaries. Integrates with codegen/reproduction scripts.
- [ ] **Budget thresholds as config** — Developer sets constraints ("load < 2s, bundle < 500KB") in a `.gasoline.json` or via MCP tool. AI enforces as hard constraints during development. Persisted via `session_store`.

## Ecosystem Exports

Standard format exports that make Gasoline data consumable by other tools. All are new files with zero overlap on existing code.

### Tier 1: Standard Formats (low effort, high reach)

- [ ] **HAR export** — `export_har` MCP tool. Network bodies + timing → HTTP Archive format. Consumable by Charles Proxy, Fiddler, Chrome DevTools import, Postman. ~100 lines Go. File: `export_har.go`.
- [ ] **SARIF export** — `export_sarif` MCP tool. A11y audit results → Static Analysis Results Interchange Format. Consumable by GitHub Code Scanning, VS Code SARIF Viewer, CI/CD security gates. ~80 lines Go. File: `export_sarif.go`.
- [ ] **Playwright test export** — Formalize existing `generate_test` into proper `.spec.ts` files with network mocking from captured bodies. Enhancement to `codegen.go`.

### Tier 2: Dev Tool Integrations (medium effort, focused audience)

- [ ] **MSW export** — `export_msw` MCP tool. Endpoint catalog + network bodies → Mock Service Worker handlers. "Observe your real API, generate test mocks automatically." ~150 lines Go. File: `export_msw.go`.
- [ ] **OpenAPI skeleton** — `export_openapi` MCP tool. Observed endpoints, methods, status codes, example bodies → OpenAPI 3.0 YAML. Not type inference — just observed facts. ~200 lines Go. File: `export_openapi.go`.
- [ ] **VS Code extension** — Gasoline status in sidebar: error count, recent failures, endpoint list, performance alerts. Uses existing MCP tools. Separate repo.

### Tier 3: Observability Bridge (higher effort, enterprise value)

- [ ] **Prometheus metrics** — `/metrics` endpoint. Performance marks, endpoint latencies, error rates, WS connection counts. Consumable by Grafana, Datadog, any monitoring dashboard. ~200 lines Go. File: `export_prometheus.go`.
- [ ] **Webhook/event stream** — Push captured events to arbitrary URLs. Works without MCP notifications. Consumable by Slack, Discord, PagerDuty, custom dashboards. ~100 lines Go. File: `export_webhook.go`.
- [ ] **Sentry-compatible export** — Format errors for local Sentry/GlitchTip. Stack traces, breadcrumbs (user actions), context. Sentry envelope format. ~200 lines Go. File: `export_sentry.go`.

## Later: Completeness & Polish

- [ ] **API schema inference** — Auto-detect endpoints, request/response shapes, auth patterns from network traffic. Spec: `docs/ai-first/tech-spec-api-schema.md`.
- [ ] **Binary format detection** — Identify protobuf/MessagePack via magic bytes instead of hex dump.
- [ ] **Network body E2E tests** — E2E coverage for body capture (large bodies, binary, header sanitization).
- [ ] **Reproduction script enhancements** — Screenshot insertion, data fixture generation, visual assertions.
- [ ] **Extension health dashboard** — Buffer usage, memory, dropped events, POST success rate.
- [ ] **Selective capture profiles** — Pre-defined configs ("debugging", "performance", "security").

## Parallel Assignment Guide

These features touch different files and can run as simultaneous agents:

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| Rate limiting | `rate_limit.go`, `extension/background.js` | Memory, Persistent, Noise, Web Vitals |
| Memory enforcement | `memory.go`, `queries.go` (move helpers) | Rate, Persistent, Noise, Web Vitals |
| Persistent memory | `ai_persistence.go` (new) | Rate, Memory, Noise, Web Vitals |
| Noise filtering | `ai_noise.go` (new) | Rate, Memory, Persistent, Web Vitals |
| Web vitals | `extension/inject.js` | Rate, Memory, Persistent, Noise |
| Interception deferral | `extension/inject.js`, `extension/content.js` | Rate, Memory, Persistent, Noise |
