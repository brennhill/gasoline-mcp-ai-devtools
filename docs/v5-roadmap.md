# v5 Roadmap

## Quick Wins (High Impact, Low Effort)

- [x] **1. Query result cleanup** — `queryResults` map in v4.go grows unbounded; results are never deleted after retrieval. Fix: delete on read + periodic expiry sweep.
- [x] **2. Connection duration** — `get_websocket_status` doesn't calculate/format duration for active connections. Spec: `"duration": "5m02s"`.
- [x] **3. Message rate calculation** — `perSecond` field exists in types but is never computed. Spec: rolling 5-second window, recalculated every second.
- [x] **4. Age formatting** — WebSocket status should show `"age": "0.2s"` for last message preview instead of raw timestamps.

## Reliability & Safety

- [ ] **5. Rate limiting / circuit breaker** — Server should respond 429 at >1000 events/sec; extension should exponential-backoff (100ms→500ms→2s) with 5-failure circuit break.
- [ ] **6. Memory enforcement** — Server has `isMemoryExceeded()` check but never triggers automatic buffer clearing.
- [ ] **7. v4 interception deferral** — Spec says intercepts defer until after `load` event + 100ms; enforcement is loose.

## Feature Completeness

- [ ] **8. Schema variant tracking** — WebSocket connections should track message variants (e.g., "message 89%, typing 8%").
- [ ] **9. Binary format detection** — Identify protobuf/MessagePack via magic bytes instead of just hex dump.
- [ ] **10. Network body E2E tests** — No E2E coverage for body capture (large bodies, binary, header sanitization).
- [ ] **11. A11y audit result caching** — Spec says cache for 30s per URL; currently re-runs every time.

## v5 Polish

- [ ] **12. Reproduction script enhancements** — Screenshot insertion, data fixture generation, visual assertions.
- [ ] **13. Extension health dashboard** — Buffer usage, memory, dropped events, POST success rate.
- [ ] **14. Selective capture profiles** — Pre-defined configs ("debugging", "performance", "security").
