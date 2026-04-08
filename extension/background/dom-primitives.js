// AUTO-GENERATED FILE. DO NOT EDIT DIRECTLY.
// Source: scripts/templates/dom-primitives.ts.tpl
// Action logic extracted to self-contained modules:
//   dom-primitives-read.ts, dom-primitives-action.ts,
//   dom-primitives-intent.ts, dom-primitives-overlay.ts, dom-primitives-stability.ts
// Generator: scripts/generate-dom-primitives.js
import { domPrimitiveRead } from './dom-primitives-read.js';
import { domPrimitiveAction } from './dom-primitives-action.js';
// Re-export list_interactive primitive for backward compatibility
export { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js';
// Re-export extracted primitives for direct dispatch from dom-dispatch.ts
export { domPrimitiveRead } from './dom-primitives-read.js';
export { domPrimitiveAction } from './dom-primitives-action.js';
const READ_ACTIONS = new Set(['get_text', 'get_value', 'get_attribute', 'wait_for', 'wait_for_text', 'wait_for_absent']);
/**
 * Unified dispatcher that delegates to the appropriate self-contained primitive.
 * Used by domWaitFor and backward-compatible call sites.
 * For production dispatch, dom-dispatch.ts routes directly to specific primitives.
 */
export function domPrimitive(action, selector, options) {
    if (READ_ACTIONS.has(action)) {
        return domPrimitiveRead(action, selector, options);
    }
    // All mutating selector-based actions go to domPrimitiveAction
    return domPrimitiveAction(action, selector, options);
}
/**
 * Backward-compatible wait helper used by unit tests and legacy call sites.
 * Polls wait_for and listens for DOM mutations for fast resolution.
 */
export function domWaitFor(selector, timeoutMs = 5000) {
    const timeout = Math.max(1, timeoutMs);
    const startedAt = Date.now();
    const pollIntervalMs = 50;
    return new Promise((resolve) => {
        let settled = false;
        let timer = null;
        let observer = null;
        const done = (result) => {
            if (settled)
                return;
            settled = true;
            if (timer)
                clearTimeout(timer);
            if (observer)
                observer.disconnect();
            resolve(result);
        };
        const check = () => {
            const result = domPrimitiveRead('wait_for', selector, { timeout_ms: timeout });
            if (result?.success) {
                done(result);
                return;
            }
            if (Date.now() - startedAt >= timeout) {
                done({
                    success: false,
                    action: 'wait_for',
                    selector,
                    error: 'timeout',
                    message: `Element not found within ${timeout}ms: ${selector}`
                });
                return;
            }
            timer = setTimeout(check, pollIntervalMs);
        };
        try {
            observer = new MutationObserver(() => {
                if (settled)
                    return;
                const immediate = domPrimitiveRead('wait_for', selector, { timeout_ms: timeout });
                if (immediate?.success)
                    done(immediate);
            });
            observer.observe(document.body || document.documentElement, {
                childList: true,
                subtree: true,
                attributes: true,
                characterData: true
            });
        }
        catch {
            // Best-effort optimization only; polling remains authoritative.
        }
        check();
    });
}
//# sourceMappingURL=dom-primitives.js.map