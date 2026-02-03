---
status: active
scope: issues/blockers
ai-priority: high
tags: [known-issues, blockers, v5.3, v6.0]
version-applies-to: v5.3+
relates-to: [RELEASE.md, UAT-v5.3-CHECKLIST.md, ../roadmap.md]
last-verified: 2026-01-30
canonical: true
---

# Known Issues & Roadmap

## v5.2.0 — In Progress

### Fixed in v5.2

| # | Fix | Details |
|---|-----|---------|
| 3 | Accessibility audit defensive check | Added `typeof` guard for `runAxeAuditWithTimeout` in inject.js. Returns structured error instead of crashing. |
| 4 | `network_bodies` capture enforcement | Enforced `networkBodyCaptureDisabled` flag in message handler. Added tab-tracking hint when capture is on but no bodies found. |
| 5 | Extension timeout cascade | Added `await` to `handleAsyncBrowserAction` call — missing `await` caused premature cleanup and duplicate processing. |
| 6 | tabId in observe responses | Added Tab column to `observe({what: "errors"})` and `observe({what: "logs"})`. Extension now passes `tabId` through `handleLogMessage`. |
| 2 | `query_dom` implementation | Completed message forwarding chain: background.js → content.js → inject.js. Calls existing `executeDOMQuery()`. 20 new extension tests. |

### Remaining Issues

All known issues have been resolved in v5.2.

### v5.2 Planned Improvements (Specs Written)

- ~~**query_dom implementation** (Issue #2) — Complete the message forwarding chain.~~ **DONE**
- **Tab Tracking UX** — Badge indicator, switch confirmation dialog, tab close recovery. Spec: `docs/features/feature/tab-tracking-ux/product-spec.md`

---

## v6.0 Roadmap (Specs In Progress)

### Priority 1: Agentic CI/CD (Thesis Validation)

| Feature | Status | Spec |
|---------|--------|------|
| Self-Healing Tests | Spec written | `docs/features/feature/self-healing-tests/product-spec.md` |
| Gasoline MCP CI Infrastructure | Spec written | `docs/features/feature/gasoline-ci/product-spec.md` |
| Context Streaming | Spec written | `docs/features/feature/context-streaming/product-spec.md` |
| PR Preview Exploration | Spec written | `docs/features/feature/pr-preview-exploration/product-spec.md` |
| Agentic E2E Repair | Spec written | `docs/features/feature/agentic-e2e-repair/product-spec.md` |
| Deployment Watchdog | Spec written | `docs/features/feature/deployment-watchdog/product-spec.md` |
| Configuration Profiles | Spec written | `docs/features/feature/config-profiles/product-spec.md` |

### Priority 2: Competitive Parity

| Feature | Status | Spec |
|---------|--------|------|
| SEO Audit | Spec written | `docs/features/feature/seo-audit/product-spec.md` |
| Performance Audit | Spec written | `docs/features/feature/performance-audit/product-spec.md` |
| Best Practices Audit | Spec written | `docs/features/feature/best-practices-audit/product-spec.md` |
| Enhanced WCAG Audit | Spec written | `docs/features/feature/enhanced-wcag-audit/product-spec.md` |
| Auto-Paste Screenshots | Spec written | `docs/features/feature/auto-paste-screenshots/product-spec.md` |
| Annotated Screenshots | Spec written | `docs/features/feature/annotated-screenshots/product-spec.md` |
| Form Filling Automation | Spec written | `docs/features/feature/form-filling/product-spec.md` |
| E2E Testing Integration | Spec written | `docs/features/feature/e2e-testing-integration/product-spec.md` |
| CPU/Network Emulation | Spec written | `docs/features/feature/cpu-network-emulation/product-spec.md` |
| Dialog Handling | Spec written | `docs/features/feature/dialog-handling/product-spec.md` |
| Drag & Drop Automation | Spec written | `docs/features/feature/drag-drop-automation/product-spec.md` |
| A11y Tree Snapshots | Spec written | `docs/features/feature/a11y-tree-snapshots/product-spec.md` |
| Local Web Scraping | Spec written | `docs/features/feature/local-web-scraping/product-spec.md` |

### Code Health

See [docs/core/architect-review.md](docs/core/architect-review.md) for the principal architect code review with 12 prioritized recommendations.

---

## v5.1.0 — Current Release

### What's New
- **Single-tab tracking isolation** (security fix) — Extension only captures telemetry from the explicitly tracked tab. Other tabs are completely isolated.
- **"Track This Tab" button** — Renamed from "Track This Page" for clarity. One-click enable/disable.
- **"No Tracking" mode** — Clear LLM-facing warning when tracking is disabled. Status ping to server every 30s.
- **Chrome internal page blocking** — Cannot track `chrome://`, `about:`, or other internal pages.
- **Browser restart handling** — Tracking state cleared on browser restart.
- **validate_api parameter fix** — Renamed conflicting parameter to `operation`.
- **Network schema improvements** — Unit suffixes, compression ratios, timestamps on waterfall/bodies.

### Full Issue Details

See [docs/core/in-progress/UAT-ISSUES-TRACKER.md](docs/core/in-progress/UAT-ISSUES-TRACKER.md) for detailed investigation notes, code references, and fix proposals.

---

## Extension Timeout Issues (Added 2026-02-02)

**Problem:**
Extension sometimes times out on initial interact() or refresh() commands with:
```
extension_timeout — Browser extension didn't respond
```

**Observed in:**
- Chrome Web Store Developer Console
- After page navigation
- Fresh page loads

**Workaround:**
- Retry the command after 2-3 seconds
- Extension eventually responds after initial timeout
- Appears to be timing issue with content script initialization

**Root Cause (To Investigate):**
- Content script may not be fully loaded when first interact() command sent
- Possible race condition between content script ready and background script sending commands
- May need "content script ready" signal before allowing interact()

**Priority:** Medium
**Affects:** v5.4.0
**Tracking:** Issue to be created in GitHub

**Proposed Fix:**
Add content script ready handshake:
1. Content script sends "GASOLINE_READY" message when initialized
2. Background script waits for ready signal before accepting interact() commands
3. Queue commands if received before ready, execute after

