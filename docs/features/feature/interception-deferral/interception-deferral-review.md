# Review: Interception Deferral Spec

**Reviewer**: Principal Engineer Review
**Spec**: `docs/ai-first/tech-spec-interception-deferral.md`
**Date**: 2026-01-26

---

## Executive Summary

A well-motivated spec that addresses a real problem: Gasoline's API monkey-patching introduces measurable overhead and initialization races during the critical rendering path. The two-phase approach is architecturally sound, and the tradeoff of missing early events is correctly justified. However, there are three critical gaps: a race between the fallback timeout and the load event handler that can cause double installation, missing coverage for XHR interception, and the settings communication channel introduces a timing window where deferral preference may arrive after Phase 1 has already decided.

---

## Critical Issues (Must Fix Before Implementation)

### 1. Double Installation via Timeout Race

**Section**: "Deferral Trigger"

The code pseudocode shows:

```js
window.addEventListener('load', installDeferred, { once: true })
setTimeout(installPhase2, 10000)  // Fallback timeout
```

If `load` fires at 9.9 seconds, both paths execute `installPhase2`. The spec mentions a `phase2Installed` guard (Edge Cases section), but the pseudocode does not include it. The guard must be shown in the primary code block, not buried in edge cases. Additionally, the `once: true` on the load listener does not prevent the `setTimeout` callback from firing.

**Fix**: The pseudocode in "Extension Changes > inject.js" must wrap `installPhase2` with a guard:

```js
function installPhase2Once() {
  if (phase2Installed) return;
  phase2Installed = true;
  installPhase2();
}
```

And both the `setTimeout` and the load listener should call `installPhase2Once`.

### 2. Settings Message Race Condition

**Section**: "content.js"

The content script sends `get_settings` to the background script and then posts the result to the page via `window.postMessage`. But `inject.js` Phase 1 runs immediately at script injection time. If the settings message arrives _after_ Phase 1 has already decided `deferralEnabled = true` (the default), there is no issue. But if the user has "Capture from page start" enabled and the settings message arrives _after_ the load event has already fired, the deferred install path has already been committed to, and the "immediate" preference is ignored.

**Fix**: The deferral decision must be made synchronously at injection time. Pass the deferral preference as a data attribute on the injected script element (`<script data-gasoline-defer="false">`), which the content script can set before injection. This eliminates the async race entirely.

### 3. XHR Not Mentioned

**Section**: "Phase 2 (Deferred)"

The spec lists fetch wrapping but does not mention `XMLHttpRequest`. While modern apps lean on `fetch`, XHR is still used by axios (default in many codebases), jQuery, and legacy code. If Gasoline wraps XHR anywhere in the current codebase, the spec must account for it. If it does not wrap XHR, the spec should state this explicitly so there is no ambiguity about coverage gaps during the deferral window.

---

## Recommendations (Should Consider)

### 4. PerformanceObserver is Not Truly Zero-Cost

**Section**: "Performance Constraints"

The spec claims PerformanceObservers in Phase 1 have "zero overhead." This is not accurate. While they do not replace prototypes, each observer callback executes on the main thread and can contribute to long task duration if the callback does meaningful work (e.g., serialization, postMessage). The overhead is small but nonzero. Recommend changing "zero overhead" to "negligible overhead (<0.1ms per callback)" for accuracy.

### 5. Retroactive WebSocket Discovery is Unreliable

**Section**: "Early Events Are Not Lost", item 3

The spec claims WebSocket connections opened before Phase 2 can be discovered via `performance.getEntriesByType('resource')` with `initiatorType === 'websocket'`. In practice, not all browsers populate this field for WebSocket connections. Chrome does not reliably include WebSocket handshakes in the Resource Timing API entries. Firefox behavior varies by version.

**Recommendation**: Document this as a best-effort mechanism. If no resource entries are found, fall back to scanning the `PerformanceObserver` buffer for `initiatorType === 'xmlhttprequest'` entries that match `ws://` or `wss://` upgrade patterns. Also consider that the current `inject.js` already exports WebSocket wrapping functions -- verify whether the extension has other mechanisms (e.g., `webRequest` API from background.js) that could provide backup coverage.

### 6. The 100ms Post-Load Delay is Arbitrary

**Section**: "Deferral Trigger"

The 100ms buffer is reasonable but undocumented in terms of what it is based on. Late-firing scripts during the load event are typically < 50ms on modern hardware, and analytics scripts that fire "just after load" tend to use `requestIdleCallback` or `setTimeout(fn, 0)`. The 100ms value should be configurable (stored alongside the deferral toggle), or at minimum the spec should cite the measurement that justifies it.

### 7. Test Scenario 6 Contradicts the Implementation

**Section**: "Test Scenarios"

Test 6 says "Page takes 15 seconds to load -> Phase 2 installs at 10s timeout." But the implementation says `setTimeout(installPhase2, 10000)`. If `load` fires at 15 seconds, Phase 2 would have installed at 10 seconds via the timeout AND again at 15.1 seconds via the load handler. This reinforces the need for the double-installation guard in Critical Issue #1.

### 8. No Telemetry for Deferral Impact

The `get_page_info` response includes deferral diagnostics, but there is no mechanism to measure the _actual_ FCP/LCP impact of deferral vs. immediate mode. Consider adding an A/B comparison metric: when "Capture from page start" is toggled, record before/after FCP values so the AI can quantify whether deferral actually improved performance for a given site.

---

## Implementation Roadmap

1. **Resolve the settings delivery mechanism**: Change from async `postMessage` to synchronous data attribute on the script element. This is a content.js change.

2. **Implement Phase 2 guard**: Add `phase2Installed` boolean check at the top of `installPhase2`. Wire both the load listener and the timeout fallback through the guarded wrapper.

3. **Audit XHR coverage**: Search inject.js and its modules for XMLHttpRequest wrapping. If present, add to Phase 2 list. If absent, document explicitly.

4. **Write tests (TDD per CLAUDE.md)**: Start with `extension-tests/interception-deferral.test.js`. Cover all 13 scenarios, with particular focus on the double-install guard (scenario 12), the settings race (new scenario needed), and the timeout fallback (scenario 6).

5. **Implement Phase 1**: Extract lightweight setup into `installPhase1()`. Verify PerformanceObservers, `__gasoline` namespace, and message listener are sufficient.

6. **Implement Phase 2 deferral**: Restructure the bottom of inject.js per the spec. Add the `readyState === 'complete'` fast path.

7. **Add retroactive WebSocket discovery**: Implement the `performance.getEntriesByType` scan as a best-effort mechanism, with a comment noting browser compatibility limitations.

8. **Update `get_page_info` response**: Add the `gasoline` diagnostics section with injection/phase2 timestamps and capture window metadata.

9. **Add options page toggle**: Wire "Capture from page start" through `chrome.storage.local` and surface it in options.js.

10. **Performance validation**: Measure FCP/LCP on a reference page with and without deferral. Document the delta.
