# v5 (5.0.0) UAT Checklist

## Setup
- [ ] `make dev` builds without errors
- [ ] `./dist/gasoline --version` shows `gasoline v5.0.0`
- [ ] Server starts: `./dist/gasoline` shows banner with v5.0.0
- [ ] Extension loaded in Chrome (unpacked from `extension/`)
- [ ] Extension popup shows "Connected" status

---

## Phase 1: Basic Pipeline (extension → server → MCP)

| # | Action | Expected | Pass |
|---|--------|----------|------|
| 1.1 | `console.error("UAT test")` in DevTools | `observe {what: "errors"}` returns the error | [ ] |
| 1.2 | `console.log("info message")` | `observe {what: "logs"}` returns the log | [ ] |
| 1.3 | `fetch('/api/test')` in console | `observe {what: "network"}` shows request+response | [ ] |
| 1.4 | Open WS connection (or navigate to WS-enabled page) | `observe {what: "websocket_status"}` shows connection | [ ] |
| 1.5 | Click a button/link | `observe {what: "actions"}` captures interaction | [ ] |
| 1.6 | Check page info | `observe {what: "page"}` returns URL, title, viewport | [ ] |
| 1.7 | Navigate to different page | Page info updates on next call | [ ] |

## Phase 2: Composite Tools

| # | Tool Call | Expected | Pass |
|---|-----------|----------|------|
| 2.1 | `analyze {target: "performance"}` | Returns metrics or "no data yet" | [ ] |
| 2.2 | `analyze {target: "api"}` (after network activity) | Returns inferred API endpoints | [ ] |
| 2.3 | `analyze {target: "accessibility"}` | Returns a11y violations/passes | [ ] |
| 2.4 | `analyze {target: "changes"}` | Returns changes since checkpoint | [ ] |
| 2.5 | `generate {format: "har"}` | Valid HAR JSON with entries array | [ ] |
| 2.6 | `generate {format: "sarif"}` (after a11y audit) | Valid SARIF with runs[] | [ ] |
| 2.7 | `generate {format: "reproduction"}` | Playwright-style script | [ ] |
| 2.8 | `configure {action: "store", key: "test", value: "hello"}` | Stores successfully | [ ] |
| 2.9 | `configure {action: "load"}` | Returns stored key | [ ] |

## Phase 3: Security Tools

| # | Tool Call | Expected | Pass |
|---|-----------|----------|------|
| 3.1 | `security_audit` | Reports on headers, cookies, transport | [ ] |
| 3.2 | `generate_csp` (after navigation) | CSP directives from observed origins | [ ] |
| 3.3 | `audit_third_parties` (page with CDN scripts) | Classifies origins by risk | [ ] |
| 3.4 | `diff_security {action: "snapshot", name: "before"}` | Snapshot saved | [ ] |
| 3.5 | `diff_security {action: "list"}` | Shows "before" snapshot | [ ] |
| 3.6 | `diff_security {action: "compare", compare_from: "before"}` | Shows diff vs current | [ ] |

## Phase 4: AI-First Features

| # | Action | Expected | Pass |
|---|--------|----------|------|
| 4.1 | `configure {action: "noise_rule", ...}` then trigger matching log | Noise filtered from subsequent observe | [ ] |
| 4.2 | Call `observe` — check response structure | Includes `alerts` block (may be empty) | [ ] |
| 4.3 | Trigger rapid errors (>3x spike in 10s) | Anomaly alert in next `observe` | [ ] |
| 4.4 | Check `tools/list` response | `_meta.data_counts` present per tool | [ ] |

## Phase 5: Edge Cases & Resilience

| # | Action | Expected | Pass |
|---|--------|----------|------|
| 5.1 | Reload page | Capture resumes (interception deferral works) | [ ] |
| 5.2 | Navigate to new origin | New page captured, old data still accessible | [ ] |
| 5.3 | `query_dom` with CSS selector | Returns matching elements | [ ] |
| 5.4 | Extension popup | Shows connection status, no JS errors | [ ] |
| 5.5 | Server memory stays bounded | After heavy activity, no unbounded growth | [ ] |

## Phase 6: Version Verification

| # | Check | Expected | Pass |
|---|-------|----------|------|
| 6.1 | `./dist/gasoline --version` | `gasoline v5.0.0` | [ ] |
| 6.2 | MCP initialize response | `"version": "5.0.0"` | [ ] |
| 6.3 | `window.__gasoline.version` in browser console | `"5.0.0"` | [ ] |
| 6.4 | Extension manifest (chrome://extensions) | Version 5.0.0 | [ ] |

---

## Result

- **Date:** ____
- **Tester:** ____
- **Browser:** Chrome ____
- **OS:** ____
- **Verdict:** PASS / FAIL
- **Notes:**
