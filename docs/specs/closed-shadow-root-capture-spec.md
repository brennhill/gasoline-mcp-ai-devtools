---
feature: Closed Shadow Root Capture
status: proposed
depends_on: shadow-dom-support-spec.md (open root support ships first)
---

# Tech Spec: Closed Shadow Root Capture

> Plain language only. Describes HOW the implementation works.

## Problem

`element.shadowRoot` returns `null` for closed shadow roots. Once `attachShadow({ mode: 'closed' })` executes, the ShadowRoot reference is gone forever — unless intercepted at creation time.

Without interception, Gasoline's deep traversal engine (from the open shadow DOM spec) stops at every closed boundary. Components using closed roots include: Salesforce Lightning, some Shopify Polaris internals, banking/fintech widgets, and any security-conscious web component.

## Approach

Monkey-patch `Element.prototype.attachShadow` in `early-patch.ts` to capture closed ShadowRoot references in a WeakMap before page JavaScript runs. The patch now uses a trampoline/getter-setter pattern so page-level reassignment (`Element.prototype.attachShadow = ...`) is intercepted without losing capture.

## Architecture

```
Timeline (per page load):
──────────────────────────────────────────────────────────────

1. early-patch.bundled.js (MAIN world, document_start)
   └─ Installs attachShadow trampoline + overwrite interceptor
   └─ Stores closed ShadowRoots in WeakMap on window
   └─ Buffers early diagnostics in __GASOLINE_EARLY_LOGS__

2. Page JavaScript runs
   └─ Calls attachShadow({ mode: 'closed' }) — captured transparently

3. inject.bundled.js loads (MAIN world, programmatic)
   └─ Flushes __GASOLINE_EARLY_LOGS__ into GASOLINE_LOG pipeline
   └─ Sets __GASOLINE_INJECT_READY__ = true
   └─ Logs reach background -> /logs on Go server

4. dom-primitives.ts executeScript calls (MAIN world, on-demand)
   └─ Reads WeakMap to traverse closed roots
```

## Implementation

### 1. Early Patch: `src/early-patch.ts`

Add the `attachShadow` patch alongside the existing WebSocket patch. Same IIFE, same file, same bundle.

```typescript
// --- Closed Shadow Root Capture (hardened) ---

const original = Element.prototype.attachShadow
const closedRoots = new WeakMap<Element, ShadowRoot>()
let delegate = original

const trampoline = function (this: Element, init: ShadowRootInit): ShadowRoot {
  const root = delegate.call(this, init)
  if (init.mode === 'closed') closedRoots.set(this, root)
  return root
}

Object.defineProperty(Element.prototype, 'attachShadow', {
  configurable: true,
  enumerable: false,
  get: () => trampoline,
  set: (next) => {
    // Keep capture active while preserving page override behavior.
    if (typeof next !== 'function' || next === trampoline) return
    delegate = function (this: Element, init: ShadowRootInit): ShadowRoot {
      return next.call(this, init)
    }
    queueEarlyLog('attachShadow overwrite intercepted')
  }
})
```

**Why this is safe:**
- The trampoline still calls the page's replacement function (delegate mode)
- Closed-root capture stays active even after reassignment
- Return value is the real ShadowRoot (not a wrapper)
- WeakMap means captured roots are GC'd when their host element is removed
- No observable side-effects: `init.mode` is still `'closed'`, `element.shadowRoot` still returns `null`

**Why early-patch and not inject:**
- `attachShadow` is called during custom element construction, which often happens during HTML parsing
- If a `<my-component>` tag appears in the initial HTML, its constructor runs during parse — before inject.bundled.js loads
- `early-patch.bundled.js` runs at `document_start` via manifest declaration — this is the only reliable interception point

### 2. Type Declarations: `src/types/global.d.ts`

Add to the `Window` interface:

```typescript
/** Early-patch: captured closed shadow roots (host → ShadowRoot) */
__GASOLINE_CLOSED_SHADOWS__?: WeakMap<Element, ShadowRoot>

/** Early-patch: original attachShadow saved before patch */
__GASOLINE_ORIGINAL_ATTACH_SHADOW__?: typeof Element.prototype.attachShadow

/** Early-patch: buffered diagnostics before inject.js is ready */
__GASOLINE_EARLY_LOGS__?: EarlyPatchLogEntry[]

/** inject.js readiness flag used to avoid duplicate early-patch log emission */
__GASOLINE_INJECT_READY__?: boolean
```

### 3. Early Telemetry Flush: `src/inject/early-patch-logs.ts` (new file)

At inject startup, flush any buffered early-patch logs through `postLog()`. This routes events into existing ingestion (`GASOLINE_LOG` -> background -> Go `/logs`) without adding a new API.

This is used for events that happened before `inject.bundled.js` finished loading.

### 4. Adoption: `src/inject/shadow-registry.ts` (optional)

Follows the same adoption pattern as `lib/websocket.ts:586-730`:

```typescript
// shadow-registry.ts — Adopts captured closed shadow roots from early-patch.

let closedRoots: WeakMap<Element, ShadowRoot> | null = null

/**
 * Adopt the closed shadow root registry from early-patch globals.
 * Called once during inject Phase 1 initialization.
 */
export function adoptClosedShadowRoots(): void {
  closedRoots = window.__GASOLINE_CLOSED_SHADOWS__ ?? null

  // Clean up globals
  delete window.__GASOLINE_CLOSED_SHADOWS__
  delete window.__GASOLINE_ORIGINAL_ATTACH_SHADOW__
}

/**
 * Look up a closed shadow root by its host element.
 * Returns null if the element has no captured closed root.
 */
export function getClosedShadowRoot(host: Element): ShadowRoot | null {
  return closedRoots?.get(host) ?? null
}

/**
 * Teardown: release references. Called on extension unload.
 */
export function destroyShadowRegistry(): void {
  closedRoots = null
  delete window.__GASOLINE_CLOSED_SHADOWS__
  delete window.__GASOLINE_ORIGINAL_ATTACH_SHADOW__
}
```

### 5. Integration with Deep Traversal (`dom-primitives.ts`)

The deep traversal engine (from the open shadow DOM spec) needs access to closed roots. Since `domPrimitive` runs as a self-contained function via `chrome.scripting.executeScript`, it cannot import modules. The closed root WeakMap must be passed as an argument or accessed via a window global.

**Option A: Window global (recommended)**

The WeakMap stays on `window.__GASOLINE_CLOSED_SHADOWS__` and `domPrimitive` reads it directly. The inject script does NOT delete the global — it only ensures the early-patch is cleaned up on unload.

```typescript
// Inside domPrimitive (self-contained)
function getShadowRoot(el: Element): ShadowRoot | null {
  if (el.shadowRoot) return el.shadowRoot // open
  const closed = (window as any).__GASOLINE_CLOSED_SHADOWS__
  return closed?.get(el) ?? null
}
```

**Why not Option B (pass as arg):** `chrome.scripting.executeScript` serializes arguments. WeakMap is not serializable. The global is the only viable path.

**Implication:** The `adoptClosedShadowRoots()` function should NOT delete `window.__GASOLINE_CLOSED_SHADOWS__`. Instead, it should only delete `__GASOLINE_ORIGINAL_ATTACH_SHADOW__` (the saved original, which is no longer needed). The WeakMap must remain accessible for the lifetime of the page.

Revised adoption:

```typescript
export function adoptClosedShadowRoots(): void {
  closedRoots = window.__GASOLINE_CLOSED_SHADOWS__ ?? null
  // Keep __GASOLINE_CLOSED_SHADOWS__ on window — dom-primitives needs it
  // Only clean up the original function reference
  delete window.__GASOLINE_ORIGINAL_ATTACH_SHADOW__
}
```

## DOM Query Control (`pierce_shadow`)

`analyze({ what: "dom" })` now supports `pierce_shadow` with three values:

- `true`: always traverse open + captured closed shadow roots.
- `false`: stay in light DOM only (default safe behavior in inject execution).
- `"auto"`: background resolves using active debug intent heuristic, then forwards boolean to inject.

Default is `"auto"` at tool level.

### Auto Heuristic (Active Debug Intent)

`"auto"` resolves to `true` only when all are true:

- AI Web Pilot is enabled.
- Target tab is the tracked debug tab.
- Target tab origin matches tracked tab origin.

Otherwise `"auto"` resolves to `false`.

Implementation detail:

- Resolution happens in `pending-queries.ts` (background) before `DOM_QUERY` is sent to content/inject.
- Inject-side `executeDOMQuery` accepts `true|false|"auto"`, but treats unresolved `"auto"` as `false` for safety.

## Risks and Mitigations

## User Safety Mode (Default Behavior)

This section defines how Gasoline avoids accidental breakage on normal browsing. Goal: users should not have a bad experience because they forgot Gasoline was enabled.

### Default Mode: `safe`

`safe` is the default operating mode. In `safe` mode:

- No aggressive early monkey-patches on arbitrary browsing.
- `attachShadow` and early WebSocket interception are enabled only when the user is explicitly in an active debugging flow.
- Active debugging flow means both:
  - AI Web Pilot/debugging is enabled by user intent.
  - Current page origin matches the tracked/debug target origin.

### Scoped Aggressive Hooks

Aggressive instrumentation is scoped to the active debug target only:

- If page origin is not the tracked origin, run passive capture only.
- If tracking/pilot is disabled, run passive capture only.
- If user exits debug flow, revert to passive capture.

### Automatic Compatibility Downgrade

When likely challenge/protection friction is detected, Gasoline must auto-downgrade to passive mode for that origin.

Downgrade triggers include (heuristic, any one is sufficient):

- Burst of `403`/`429` responses shortly after hook activation.
- Known challenge/captcha URL patterns.
- Access-denied/challenge markers observed in page title/URL/content snippets.
- Extension-level warnings indicating hook conflict.

On trigger:

- Disable aggressive hooks for that origin immediately.
- Continue passive telemetry so tools still work (reduced visibility).
- Emit a compatibility event for server/operator visibility.

### Persistent Per-Origin Compatibility State

Gasoline stores origin compatibility state locally:

- `mode`: `passive` | `standard` | `full`
- `reason`: short string (e.g., `challenge_detected`, `user_override`, `manual_allow`)
- `updated_at`, optional `passive_until`

Behavior:

- Origins with recent downgrade load as passive first.
- Optional TTL (`passive_until`) allows automatic re-evaluation later.
- User can manually promote/demote per origin.

### User-Facing UX Requirements

When compatibility downgrade happens, user gets clear non-technical feedback:

- Badge/toast: “Gasoline switched to compatibility mode on this site.”
- Optional inline detail: “Some deep capture features are paused to avoid site interference.”

This prevents confusion and reduces uninstall risk.

### Timeout Failsafe

Aggressive mode auto-expires after a configurable idle window (for example 10-30 minutes):

- If user forgets to turn Gasoline off, session de-escalates to passive automatically.
- Re-entry into aggressive mode requires active user/debug intent.

### Diagnostics and Observability

All mode transitions are logged and sent through existing server ingestion:

- `from_mode`, `to_mode`
- `origin`
- `reason`
- `trigger_signal` (if auto downgrade)
- `timestamp`

This enables support/debugging without adding new endpoints.

### Risk 1: Page detects the patch

**Likelihood:** Low but possible. A page could save a reference to `Element.prototype.attachShadow` before early-patch runs (impossible — early-patch runs at `document_start` before any page script) or compare the function's `toString()` output.

**Mitigation:** None needed. This is standard practice for browser automation tools (Playwright, Puppeteer, Selenium all do this). Gasoline is a developer tool, not a stealth tool.

### Risk 2: Patch doesn't run before page JS

**Likelihood:** Very low. Chrome's `document_start` content script injection is the earliest possible timing in MV3. However, there are two edge cases:
- **Prerendered pages:** Chrome may not inject content scripts into prerendered pages until activation. Components constructed during prerender would be missed.
- **Extension reload:** If the extension reloads while a page is already loaded, the patch misses components that already called `attachShadow`.

**Mitigation:** The guard `if (window.__GASOLINE_ORIGINAL_ATTACH_SHADOW__) return` prevents double-patching. Missed closed roots degrade gracefully — the deep traversal simply won't enter those roots, same as today. No errors, no crashes.

### Risk 3: WeakMap memory pressure

**Likelihood:** Negligible. WeakMap entries are keyed by the host Element. When the element is removed from DOM and GC'd, the WeakMap entry (and the ShadowRoot reference) is also collected. No unbounded growth.

### Risk 4: Framework interference

**Likelihood:** Low. Some frameworks (LitElement, Stencil) save a reference to `attachShadow` during module initialization. If their module loads before `early-patch.ts` (impossible in normal operation — `document_start` runs first), they'd bypass the patch. But even if a framework calls `Element.prototype.attachShadow` directly via a cached reference, the patch still works because we patch the prototype method, not instance methods.

**Edge case:** Some code compares strict function identity of `Element.prototype.attachShadow`. The trampoline changes identity, so those checks can fail.

**Mitigation:** keep delegate behavior so replacement semantics still run; emit warning telemetry to help diagnose identity-sensitive pages.

### Risk 5: early-patch.ts bundle size increase

**Likelihood:** Certain but minor. The patch adds ~15 lines (~300 bytes minified) to the early-patch bundle. Current early-patch is ~800 bytes minified. A ~35% size increase is acceptable for a script that runs once at page load.

## Performance

- **attachShadow overhead:** One WeakMap `.set()` call per closed shadow root creation. WeakMap operations are O(1). Unmeasurable overhead.
- **Deep traversal overhead:** One WeakMap `.get()` call per element checked during traversal. Same O(1) cost. No overhead on pages without closed shadow roots (the WeakMap is simply empty).
- **Memory:** WeakMap holds strong references to ShadowRoot objects. A closed ShadowRoot stays in memory as long as its host element exists — this is identical to what happens with open shadow roots (the browser holds the same reference via `element.shadowRoot`). No additional memory pressure.

## Testing

### Unit Tests: `extension/background/closed-shadow-capture.test.js`

```
1. early-patch saves original attachShadow
2. Patched attachShadow captures closed roots in WeakMap
3. Patched attachShadow does NOT capture open roots (shadowRoot already accessible)
4. Original attachShadow behavior preserved (return value, mode, delegatesFocus)
5. Overwrite interception: page assigns replacement function, capture still works
6. Non-function overwrite attempt is ignored and logged
7. WeakMap entries are per-host (multiple components work independently)
```

### Integration Tests: `tests/e2e/shadow-dom-closed.test.ts`

```
1. Create page with closed shadow root component
2. Verify list_interactive finds elements inside closed root
3. Verify click/type actions work on elements inside closed root
4. Verify get_text returns content from inside closed root
5. Verify selector generation produces working >>> selectors for closed roots
```

### Regression Tests

```
1. Pages with zero shadow DOM: no performance regression
2. Pages with only open shadow roots: behavior unchanged
3. attachShadow({ mode: 'open' }) still returns accessible shadowRoot
4. Extension unload cleans up globals
```

### Smoke Test

`scripts/smoke-tests/04-network-websocket.sh` includes a browser smoke check that:
1. Reassigns `Element.prototype.attachShadow` from page JS
2. Creates a closed shadow root and verifies `hasCaptured=true`
3. Verifies overwrite telemetry appears in `observe(logs)` with the injected marker

## Files Modified

| File | Change |
|------|--------|
| `src/early-patch.ts` | Add attachShadow patch (~15 lines) |
| `src/types/global.d.ts` | Add Window interface members (2 lines) |
| `src/inject/early-patch-logs.ts` | New file: flush buffered early-patch logs into GASOLINE_LOG |
| `src/inject/shadow-registry.ts` | New file: adoption + lookup (~30 lines) |
| `src/inject/index.ts` | Call `adoptClosedShadowRoots()` in Phase 1 |
| `src/background/dom-primitives.ts` | Add `getShadowRoot()` helper used by deep traversal |
| `scripts/smoke-tests/04-network-websocket.sh` | Add overwrite-resilience smoke test with server log assertion |

## Dependencies

This spec **requires** the open shadow DOM support spec to ship first. The deep traversal engine (recursive `querySelectorAllDeep`, `>>>` combinator, TreeWalker cross-root walking) must exist before closed root support adds value. This spec only adds the **capture mechanism** — the traversal engine consumes it.

## Rollout

1. Ship open shadow DOM support (deep traversal for open roots)
2. Ship this spec (closed root capture via early-patch)
3. Ship `safe` default mode with origin-scoped aggressive hooks
4. Ship automatic downgrade + persisted per-origin compatibility memory
5. Add UX messaging for compatibility-mode transitions
6. Add timeout failsafe for aggressive mode auto-expiry
7. The deep traversal engine picks up closed roots automatically via `getShadowRoot()` when aggressive mode is active
