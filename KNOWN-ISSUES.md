# Known Issues & Roadmap

## v5.1.0 — Current Release

### What's New
- **Single-tab tracking isolation** (security fix) — Extension only captures telemetry from the explicitly tracked tab. Other tabs are completely isolated.
- **"Track This Tab" button** — Renamed from "Track This Page" for clarity. One-click enable/disable.
- **"No Tracking" mode** — Clear LLM-facing warning when tracking is disabled. Status ping to server every 30s.
- **Chrome internal page blocking** — Cannot track `chrome://`, `about:`, or other internal pages.
- **Browser restart handling** — Tracking state cleared on browser restart.
- **validate_api parameter fix** — Renamed conflicting parameter to `operation`.
- **Network schema improvements** — Unit suffixes, compression ratios, timestamps on waterfall/bodies.

### Known Issues (Targeted for v5.2)

| # | Severity | Issue | Description |
|---|----------|-------|-------------|
| 2 | High | `query_dom` not implemented | MCP schema advertises the feature, but background.js returns `not_implemented`. Needs message forwarding from background → content → inject. |
| 3 | High | Accessibility audit runtime error | `runAxeAuditWithTimeout` reports "not defined" at runtime despite being imported. Likely Chrome caching issue — clean reinstall may resolve. |
| 4 | Medium | `network_bodies` returns no data | Multiple page loads return empty arrays. Body capture may be disabled by default or URL filtering too aggressive. Investigation needed. |
| 5 | Medium | Extension timeouts after several operations | After 5-6 navigate commands, extension starts timing out. Pilot still shows connected. Possible message queue backup or memory leak. |
| 6 | Medium | `observe()` missing tabId in responses | LLM cannot detect which tab data came from. content.js attaches tabId but server doesn't surface it in MCP responses. |

### v5.2 Roadmap

**Priority fixes:**
1. Fix `network_bodies` capture (Issue #4) — Core feature, blocks body inspection
2. Implement `query_dom` (Issue #2) — Or remove from schema until ready
3. Fix accessibility audit (Issue #3) — Clean extension reinstall + defensive check
4. Fix extension timeouts (Issue #5) — Investigate message queue / memory
5. Add tabId to observe responses (Issue #6) — Enable LLM tab-switch detection

**Planned improvements:**
- Visual indicator on tracked tab (extension badge icon)
- Confirmation dialog when switching tracked tab
- Tab switch suggestion when tracked tab closes

### Full Issue Details

See [docs/core/in-progress/UAT-ISSUES-TRACKER.md](docs/core/in-progress/UAT-ISSUES-TRACKER.md) for detailed investigation notes, code references, and fix proposals.
