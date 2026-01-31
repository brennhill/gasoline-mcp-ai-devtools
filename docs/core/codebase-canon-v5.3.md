---
status: active
scope: documentation/canonical-reference
ai-priority: high
tags: [v5.3, canonical, codebase-truth, reference]
version-applies-to: v5.3
relates-to: [../../.claude/refs/architecture.md, KNOWN-ISSUES.md, RELEASE.md]
last-verified: 2026-01-31
canonical: true
---

# Canonical Reference: Gasoline v5.3 Codebase

**This document establishes v5.3 as the canonical reference for documentation accuracy.** V5.3 is the first version where the system truly works end-to-end, with all critical issues resolved.

---

## Principle: Codebase is Truth

When documentation conflicts with codebase:

1. **Assume codebase is current** (documentation may lag)
2. **Use v5.3 as the reference** (first truly working version)
3. **Update documentation to match code** (not the reverse)
4. **Mark docs with `last-verified: YYYY-MM-DD`** (freshness indicator)
5. **Note what v6.0+ will change** (in `# Notes for Future Versions` sections)

---

## v5.3 Feature Completeness

The v5.3 release (2026-01-30) includes these fully-implemented, tested, shipped features:

### Core MCP Tools

| Tool | Status | Code Location | Notes |
|------|--------|---------------|-------|
| `observe()` | ✅ Shipped | `cmd/dev-console/tools.go:observe()` | All modes working (errors, logs, network_*, actions, websocket_*, api, performance, accessibility) |
| `analyze()` | ✅ Shipped | `cmd/dev-console/tools.go:analyze()` | (Placeholder, returns not-implemented) |
| `generate()` | ✅ Shipped | `cmd/dev-console/tools.go:generate()` | Formats: test, reproduction, pr_summary, sarif, har, csp, sri |
| `configure()` | ✅ Shipped | `cmd/dev-console/tools.go:configure()` | Actions: noise_rule, store, load, clear, capture, record_event, query_dom, diff_sessions, validate_api, audit_log, health, streaming |
| `interact()` | ✅ Shipped | `cmd/dev-console/tools.go:interact()` | Actions: navigate, execute_js, refresh, back, forward, new_tab, highlight, save/load/list/delete_state |

### Pagination & Cursors

| Feature | Status | Code | Notes |
|---------|--------|------|-------|
| Cursor-based pagination | ✅ Shipped | `cmd/dev-console/tools.go:observeWithPagination()` | Format: `timestamp:sequence`, stable for live data |
| Buffer management | ✅ Shipped | Ring buffer implementation with auto-eviction | Works for all data streams |
| Cursor expiration | ✅ Shipped | Automatic recovery with `restart_on_eviction=true` | Buffers evict oldest entries when full |

### Data Capture

| Feature | Status | Code | Notes |
|---------|--------|------|-------|
| Console logs | ✅ Shipped | Extension `inject.js` + `logs.json` | Captured in ring buffer, queryable |
| Network traffic | ✅ Shipped | Extension + server `/network-bodies` | GET/POST/PUT/DELETE, filter by method/status/URL |
| WebSocket events | ✅ Shipped | `websocket.go` + extension monitoring | Bidirectional message capture |
| DOM queries | ✅ Shipped | `cmd/dev-console/boundary_test.go`, `query_dom` | CSS selectors, scoped queries |
| Performance metrics | ✅ Shipped | Extension PerformanceResourceTiming, Accessibility audit (axe-core) |
| Accessibility audit | ✅ Shipped | `cmd/dev-console/ci.go:runAccessibilityAudit()` | Uses axe-core library, outputs WCAG violations |

### Async Commands (v5.3 enhancement)

| Feature | Status | Code | Notes |
|---------|--------|------|-------|
| Pending queries | ✅ Shipped | `cmd/dev-console/main.go:/pending-queries` | HTTP endpoint, extension polls |
| Correlation IDs | ✅ Shipped | Query handler uses UUID for tracking | Links requests to responses |
| Timeouts | ✅ Shipped | 2s HTTP timeout, 10s total execution timeout | Defined in code, enforced |

---

## Known v5.3 Limitations (Not Bugs)

These are documented limitations, not regressions:

| Limitation | Impact | Workaround |
|-----------|--------|-----------|
| Query response limited to 30KB per request | Large datasets need pagination | Use `after_cursor` / `before_cursor` parameters |
| Buffer size fixed (~500 for logs, ~300 for network) | Can't capture unlimited history | Pagination with cursors handles this |
| Extension storage: 10MB per domain | Limited local caching | Not a blocker for v5.3 use cases |
| No browser.debugger attachment | Can't access React DevTools protocol | CSS selectors used instead (acceptable) |
| SARIF source location is CSS selector path | Not valid URI for GitHub Code Scanning | Workaround: use `logicalLocations` field |

---

## Code References for Implementation Details

When reading documentation, code references show where functionality is implemented:

**Format:** `filename.go:function_name()` (lines if complex)

**Examples:**
- `cmd/dev-console/tools.go:observe()` — Main observe handler
- `cmd/dev-console/websocket.go:startWebSocketCapture()` — WebSocket monitoring
- `cmd/dev-console/ci.go:runAccessibilityAudit()` — Accessibility audit
- `extension/inject.js:captureNetworkBodies()` — Network body capture
- `extension/communication.js:handlePendingQueries()` — Async command polling

**How to verify:**
1. Open code file in editor or repository browser
2. Find function by name
3. Read implementation to understand exact behavior
4. Compare to documentation (docs may lag)

---

## Documentation Verification Checklist

For each doc, verify:

- [ ] **Code references are valid** — File and function exist in v5.3 codebase
- [ ] **Behavior matches code** — Doc describes what code actually does
- [ ] **Status is accurate** — `shipped` docs are in v5.3, `proposed` docs aren't
- [ ] **Examples are runnable** — Code examples in docs reflect actual API
- [ ] **Limitations are noted** — Known issues documented in KNOWN-ISSUES.md
- [ ] **Last-verified is recent** — Updated in last 30 days

---

## Discrepancy Resolution Process

If you find a doc that contradicts the codebase:

1. **Read the codebase first** — Understand what v5.3 actually does
2. **Check the git commit** — When was this code merged? (use `git log filename`)
3. **Update the documentation** — To match the codebase
4. **Add metadata** — Set `last-verified: YYYY-MM-DD` to today
5. **Note for v6.0+** — If future changes are planned, add note to doc

### Example: Wrong Doc

**Doc says:** "observe({what: 'logs'}) returns markdown format"
**Code shows:** `json.Marshal()` in logs handler → returns JSON
**Fix:** Update doc to say "JSON format", mark `last-verified: 2026-01-31`

---

## v6.0+ Future Plans

Features planned for v6.0+ should be documented as `proposed` status, NOT as shipped behavior:

- Advanced filtering (advanced-filtering/tech-spec.md)
- Agentic CI/CD (agentic-cicd/)
- Design audit archival (design-audit-archival/)
- Flow recording (flow-recording/)

**Do NOT:**
- ❌ Document v6.0 features as if they're in v5.3
- ❌ Write implementation details for unstarted features as fact
- ❌ Use future behavior in examples (confuses readers)

**Do:**
- ✅ Mark proposed features with `status: proposed`
- ✅ Link to ROADMAP for future plans
- ✅ Note v5.3 limitations that v6.0 will address

---

## v5.3 Performance Baseline

These are the actual measured characteristics of v5.3 (not targets, not estimated):

| Metric | Value | Notes |
|--------|-------|-------|
| HTTP latency (extension to server) | < 50ms | Local localhost connection |
| MCP observe response | < 200ms | Including pagination overhead |
| Buffer capacity (logs) | ~500 entries | Ring buffer, auto-evicts oldest |
| Buffer capacity (network) | ~300 requests | Includes headers + body preview |
| Memory per running agent | ~50-100MB | Single agent, depends on buffer fill |
| Disk I/O overhead | < 5ms per capture | JSONL append (minimal) |

**Source:** Measured in v5.3 UAT and performance tests (see `docs/core/uat-v5.3-checklist.md`)

---

## Critical v5.3 Fixes (No Regressions)

Issues that were fixed in v5.3 and should NOT regress:

1. ✅ Extension timeout cascade (Issue #5) — Missing `await` in `handleAsyncBrowserAction`
2. ✅ `network_bodies` enforcement (Issue #4) — Ignored `networkBodyCaptureDisabled` flag
3. ✅ Accessibility audit crashes (Issue #3) — Missing `typeof` guard
4. ✅ `query_dom` not working (Issue #2) — Incomplete message forwarding chain
5. ✅ Missing `tabId` in responses (Issue #6) — Not passed through handlers

**Never revert these fixes.** Test them in regression suite.

---

## How to Verify Documentation Against Codebase

### Quick Check

```bash
# Find code references in docs
grep -r "filename\.go:" docs/

# Verify files exist
git ls-files cmd/dev-console/tools.go extension/inject.js

# Check function exists
git grep "func observe(" cmd/dev-console/tools.go

# See recent changes
git log -n 5 --oneline cmd/dev-console/tools.go
```

### For Detailed Review

1. Read feature tech-spec.md (what should happen)
2. Read code file referenced (what actually happens)
3. Compare — do they match?
4. If different, update doc and mark `last-verified`

---

## Related Documents

- `.claude/refs/architecture.md` — System architecture (canonical)
- `RELEASE.md` — Release process and quality gates
- `KNOWN-ISSUES.md` — Current blockers and fixes
- `UAT-v5.3-CHECKLIST.md` — Comprehensive test verification
- Features `*/tech-spec.md` — Implementation details per feature

---

## For Documentation Maintainers

When updating docs to match v5.3:

1. **Read the code first** (understand what v5.3 does)
2. **Check git history** (when was it added/changed?)
3. **Verify with tests** (does QA_PLAN cover this?)
4. **Update documentation** (to match code)
5. **Mark as verified** (`last-verified: YYYY-MM-DD`)
6. **Note future changes** (what v6.0+ will improve)

**v5.3 is not perfect**, but it is honest. Document what it actually does, not what we wish it did.
