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

---

## P0: Reliability (ship-blockers)

Without these, the server crashes under real-world load. Nothing else matters until these ship.

Specs: `docs/ai-first/tech-spec-rate-limiting.md`, `docs/ai-first/tech-spec-memory-enforcement.md`

- [ ] **Rate limiting / circuit breaker** — Server responds 429 at >1000 events/sec; extension exponential-backoff (100ms→500ms→2s) with 5-failure circuit break. File: `rate_limit.go`.
- [ ] **Memory enforcement** — Three thresholds (20MB soft, 50MB hard, 100MB critical) with progressive eviction. Minimal mode at critical. File: `memory.go`.
- [ ] **Interception deferral** — Defer intercepts (WS constructor, fetch body capture) until after `load` event + 100ms. Gasoline must not slow the app it debugs.

## P1: AI Intelligence (the AI gets smarter)

These make the AI a competent collaborator instead of a stateless tool.

Specs: `docs/ai-first/tech-spec-persistent-memory.md`, `docs/ai-first/tech-spec-noise-filtering.md`

- [ ] **Persistent memory** — Cross-session storage (`session_store`, `load_session_context`). Baselines, noise rules, API schemas, error history survive restarts. File: `ai_persistence.go`.
- [ ] **Noise filtering** — Built-in heuristics + agent-configurable rules (`configure_noise`, `dismiss_noise`). Auto-detection from buffer frequency analysis. File: `ai_noise.go`.
- [ ] **API schema inference** — Auto-detect endpoints, request/response shapes, auth patterns from network traffic. The AI understands your API without documentation. File: `api_schema.go`. Spec: `docs/ai-first/tech-spec-api-schema.md`.

## P2: Feedback Loop (AI learns from its own changes)

The core thesis loop: AI changes code → Gasoline detects impact → AI adjusts.

- [ ] **Push notification on regression** — AI is alerted without polling. After a reload, if baselines are exceeded, the server signals "your last reload regressed load by 800ms" via a status field in `get_changes_since`.
- [ ] **Causal diffing** — Compare resource lists (scripts, stylesheets, fonts) between baseline and current. Report what changed: "3 new scripts totaling 400KB added since baseline". File: `performance.go`.
- [ ] **Workflow integration** — Auto-run `check_performance` after each AI-initiated code change. Include performance delta in PR summaries. Integrates with codegen/reproduction scripts.

## P3: Measurement & Verification (human audits AI work)

Help the human verify what the AI observed and decided.

- [ ] **Web vitals capture** — FCP, LCP, CLS, INP via extension. Standardized performance data the AI can reference. TDD stubs exist in `extension-tests/web-vitals.test.js`.
- [ ] **SARIF export** — `export_sarif` MCP tool. A11y audit results → GitHub Code Scanning format. Human sees AI-detected issues in PR review. File: `export_sarif.go`.
- [ ] **HAR export** — `export_har` MCP tool. Network bodies + timing → HTTP Archive format. Human can inspect in Charles Proxy / DevTools. File: `export_har.go`.

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

P0 and P1 features all touch different files and can run as simultaneous agents:

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| Rate limiting | `rate_limit.go`, `extension/background.js` | Memory, Persistent, Noise |
| Memory enforcement | `memory.go`, `queries.go` | Rate, Persistent, Noise |
| Persistent memory | `ai_persistence.go` (new) | Rate, Memory, Noise |
| Noise filtering | `ai_noise.go` (new) | Rate, Memory, Persistent |
| API schema inference | `api_schema.go` (new) | All of the above |
| Interception deferral | `extension/inject.js`, `extension/content.js` | All server-side features |
