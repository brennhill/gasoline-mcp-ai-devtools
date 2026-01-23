# Roadmap

## Completed

- [x] **Query result cleanup** — Delete on read + periodic expiry sweep.
- [x] **Connection duration** — `"duration": "5m02s"` in `get_websocket_status`.
- [x] **Message rate calculation** — Rolling 5-second window, recalculated every second.
- [x] **Age formatting** — `"age": "0.2s"` in last message preview.
- [x] **A11y audit result caching** — 30s TTL, force_refresh, concurrent dedup, navigation invalidation.
- [x] **Compressed state diffs** — `get_changes_since` with checkpoint IDs for token-efficient polling.
- [x] **Performance budget monitor** — `check_performance` with snapshots, baselines, and regression detection.
- [x] **File split refactor** — Monolithic v4.go split into 8 domain files.
- [x] **Multi-agent infrastructure** — Lefthook hooks, advisory lock file, branching strategy.

## Next: Reliability (ship-blockers)

Specs: `docs/ai-first/tech-spec-rate-limiting.md`, `docs/ai-first/tech-spec-memory-enforcement.md`

- [ ] **Rate limiting / circuit breaker** — Server responds 429 at >1000 events/sec; extension exponential-backoff (100ms→500ms→2s) with 5-failure circuit break. File: `rate_limit.go`.
- [ ] **Memory enforcement** — Three thresholds (20MB soft, 50MB hard, 100MB critical) with progressive eviction. Minimal mode at critical. File: `memory.go`.

## Next: AI Features (product differentiation)

Specs: `docs/ai-first/tech-spec-persistent-memory.md`, `docs/ai-first/tech-spec-noise-filtering.md`

- [ ] **Persistent memory** — Cross-session storage (`session_store`, `load_session_context`). Baselines, noise rules, API schemas, error history survive restarts. File: `ai_persistence.go`.
- [ ] **Noise filtering** — Built-in heuristics + agent-configurable rules (`configure_noise`, `dismiss_noise`). Auto-detection from buffer frequency analysis. File: `ai_noise.go`.

## Later: Completeness & Polish

- [ ] **API schema inference** — Auto-detect endpoints, request/response shapes, auth patterns from network traffic. Spec: `docs/ai-first/tech-spec-api-schema.md`.
- [ ] **Interception deferral** — Defer v4 intercepts until after `load` event + 100ms (spec compliance).
- [ ] **Schema variant tracking** — WebSocket connections track message variants (e.g., "message 89%, typing 8%").
- [ ] **Binary format detection** — Identify protobuf/MessagePack via magic bytes instead of hex dump.
- [ ] **Network body E2E tests** — E2E coverage for body capture (large bodies, binary, header sanitization).
- [ ] **Reproduction script enhancements** — Screenshot insertion, data fixture generation, visual assertions.
- [ ] **Extension health dashboard** — Buffer usage, memory, dropped events, POST success rate.
- [ ] **Selective capture profiles** — Pre-defined configs ("debugging", "performance", "security").

## Parallel Assignment Guide

These features touch different files and can run as simultaneous agents:

| Feature | Primary files | Can parallel with |
|---------|--------------|-------------------|
| Rate limiting | `rate_limit.go`, `extension/background.js` | Memory, Persistent, Noise |
| Memory enforcement | `memory.go`, `queries.go` (move helpers) | Rate, Persistent, Noise |
| Persistent memory | `ai_persistence.go` (new) | Rate, Memory, Noise |
| Noise filtering | `ai_noise.go` (new) | Rate, Memory, Persistent |
