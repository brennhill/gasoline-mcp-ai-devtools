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
| 1.8 | Enable "Screenshot on Error" in popup, trigger error | Screenshot captured in JSONL log | [ ] |

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

## Phase 4B: AI Web Pilot (Bi-Directional Actions)

### Safety Gate Prerequisites
| # | Action | Expected | Pass |
|---|--------|----------|------|
| 4B.0 | Extension popup shows "AI Web Pilot" toggle | Toggle visible and defaults to OFF | [ ] |
| 4B.1 | Call `highlight_element` with toggle OFF | Returns `ai_web_pilot_disabled` error | [ ] |
| 4B.2 | Call `manage_state` with toggle OFF | Returns `ai_web_pilot_disabled` error | [ ] |
| 4B.3 | Call `execute_javascript` with toggle OFF | Returns `ai_web_pilot_disabled` error | [ ] |
| 4B.4 | Enable toggle in popup, verify chrome.storage | `aiWebPilotEnabled: true` in sync storage | [ ] |

### highlight_element Tool
| # | Tool Call | Expected | Pass |
|---|-----------|----------|------|
| 4B.5 | `highlight_element {selector: "#my-button"}` | Red overlay appears on element | [ ] |
| 4B.6 | Verify highlight styles | 4px red border, fixed position, max z-index, pointer-events: none | [ ] |
| 4B.7 | Highlight with `duration_ms: 2000` | Overlay auto-removes after ~2 seconds | [ ] |
| 4B.8 | Highlight non-existent element | Returns `element_not_found` error | [ ] |
| 4B.9 | Highlight second element | First overlay removed, second appears | [ ] |
| 4B.10 | Scroll page while highlighted | Overlay tracks element position | [ ] |
| 4B.11 | Result includes bounds | Response has `{x, y, width, height}` | [ ] |

### manage_state Tool
| # | Tool Call | Expected | Pass |
|---|-----------|----------|------|
| 4B.12 | `manage_state {action: "capture"}` | Returns current page state (scroll, URL, forms) | [ ] |
| 4B.13 | `manage_state {action: "save", name: "test"}` | Snapshot saved successfully | [ ] |
| 4B.14 | `manage_state {action: "list"}` | Shows "test" snapshot in list | [ ] |
| 4B.15 | Modify form input, then `restore` named snapshot | Form value reverts to saved state | [ ] |
| 4B.16 | `manage_state {action: "delete", name: "test"}` | Snapshot removed from list | [ ] |
| 4B.17 | Restore non-existent snapshot | Returns `snapshot_not_found` error | [ ] |

### execute_javascript Tool
| # | Tool Call | Expected | Pass |
|---|-----------|----------|------|
| 4B.18 | `execute_javascript {script: "return 2+2"}` | Returns `{result: 4}` | [ ] |
| 4B.19 | `execute_javascript {script: "return document.title"}` | Returns page title string | [ ] |
| 4B.20 | `execute_javascript {script: "return window.location.href"}` | Returns current URL | [ ] |
| 4B.21 | `execute_javascript {script: "return {a:1, b:[2,3]}"}` | Returns JSON object `{a:1, b:[2,3]}` | [ ] |
| 4B.22 | Read DOM value: `return document.getElementById('x').value` | Returns input value | [ ] |
| 4B.23 | Read global: `return window.someGlobal` | Returns global variable value | [ ] |
| 4B.24 | Invalid syntax: `return {{{` | Returns error with message | [ ] |
| 4B.25 | Runtime error: `return undefined.foo` | Returns error with message | [ ] |
| 4B.26 | No return value: `console.log('test')` | Returns `{result: undefined}` or success | [ ] |

### Integration Workflow
| # | Workflow | Expected | Pass |
|---|----------|----------|------|
| 4B.27 | Highlight → Execute to read value → Verify | AI can see what it's about to interact with | [ ] |
| 4B.28 | Save state → Execute to modify DOM → Restore | State rollback works after AI changes | [ ] |
| 4B.29 | Disable toggle mid-session → Call tool | Immediately returns disabled error | [ ] |

## Phase 5: Edge Cases & Resilience

| # | Action | Expected | Pass |
|---|--------|----------|------|
| 5.1 | Reload page | Capture resumes (interception deferral works) | [ ] |
| 5.2 | Navigate to new origin | New page captured, old data still accessible | [ ] |
| 5.3 | `query_dom` with CSS selector | Returns matching elements | [ ] |
| 5.4 | Extension popup | Shows connection status, no JS errors | [ ] |
| 5.5 | Server memory stays bounded | After heavy activity, no unbounded growth | [ ] |

## Phase 5B: Enhanced Features (P5)

### Binary Format Detection
| # | Action | Expected | Pass |
|---|--------|----------|------|
| 5B.1 | Fetch MessagePack response | `observe {what: "network"}` shows `binary_format: "messagepack"` | [ ] |
| 5B.2 | Fetch Protobuf response | `observe {what: "network"}` shows `binary_format: "protobuf"` | [ ] |
| 5B.3 | WebSocket binary message | Binary format detected in WS message metadata | [ ] |
| 5B.4 | Unknown binary format | Returns `binary_format: null` or undetected | [ ] |
| 5B.5 | Regular JSON response | No binary_format field (text, not binary) | [ ] |

### Reproduction Script Enhancements
| # | Tool Call | Expected | Pass |
|---|-----------|----------|------|
| 5B.6 | `generate {format: "reproduction", options: {include_screenshots: true}}` | Script includes `page.screenshot()` calls | [ ] |
| 5B.7 | `generate {format: "reproduction", options: {generate_fixtures: true}}` | Separate fixtures file generated with API responses | [ ] |
| 5B.8 | `generate {format: "reproduction", options: {visual_assertions: true}}` | Script includes `toHaveScreenshot()` assertions | [ ] |
| 5B.9 | Combined options | All three enhancements work together | [ ] |
| 5B.10 | Default options (all false) | Clean script without screenshots/fixtures/assertions | [ ] |

### Network Body E2E Verification
| # | Action | Expected | Pass |
|---|--------|----------|------|
| 5B.11 | Large response (>1MB) | Body truncated to configured limit | [ ] |
| 5B.12 | Binary response (image) | Binary preserved, not corrupted | [ ] |
| 5B.13 | Request with Authorization header | Header stripped from captured data | [ ] |
| 5B.14 | POST with body | Request body captured correctly | [ ] |
| 5B.15 | Chunked/streaming response | Complete response captured | [ ] |
| 5B.16 | 4xx/5xx error responses | Error bodies captured | [ ] |

## Phase 6: Version Verification

| # | Check | Expected | Pass |
|---|-------|----------|------|
| 6.1 | `./dist/gasoline --version` | `gasoline v5.0.0` | [ ] |
| 6.2 | MCP initialize response | `"version": "5.0.0"` | [ ] |
| 6.3 | `window.__gasoline.version` in browser console | `"5.0.0"` | [ ] |
| 6.4 | Extension manifest (chrome://extensions) | Version 5.0.0 | [ ] |

## Phase 7: Automated E2E Tests

Run the Playwright e2e tests to verify full integration:

```bash
# Build server first
make dev

# Run all e2e tests
cd e2e-tests && npm test

# Run specific test suites
npx playwright test pilot-features.spec.js    # AI Web Pilot
npx playwright test v5-features.spec.js       # v5 core features
npx playwright test phase4-features.spec.js   # Phase 4 features

# Run in headed mode for visual verification
REVIEW=1 npx playwright test pilot-features.spec.js
```

| # | Test Suite | Pass |
|---|-----------|------|
| 7.1 | `pilot-features.spec.js` — AI Web Pilot e2e tests | [ ] |
| 7.2 | `v5-features.spec.js` — Multi-strategy selectors, action recording | [ ] |
| 7.3 | `phase4-features.spec.js` — Capture control, error clustering | [ ] |
| 7.4 | `console-capture.spec.js` — Console interception | [ ] |
| 7.5 | `websocket-capture.spec.js` — WebSocket capture | [ ] |
| 7.6 | `feature-toggles.spec.js` — Toggle functionality | [ ] |

---

## Result

- **Date:** ____
- **Tester:** ____
- **Browser:** Chrome ____
- **OS:** ____
- **Verdict:** PASS / FAIL
- **Notes:**

---

## Quick Reference: AI Web Pilot Testing

### Enable Toggle
1. Click Gasoline extension icon
2. Find "AI Web Pilot" toggle
3. Toggle ON (turns green)

### Test highlight_element
```json
{"method": "tools/call", "params": {"name": "highlight_element", "arguments": {"selector": "button", "duration_ms": 3000}}}
```

### Test manage_state
```json
{"method": "tools/call", "params": {"name": "manage_state", "arguments": {"action": "save", "name": "before-test"}}}
```

### Test execute_javascript
```json
{"method": "tools/call", "params": {"name": "execute_javascript", "arguments": {"script": "return document.title"}}}
```
