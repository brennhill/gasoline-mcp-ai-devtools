/**
 * Purpose: Thin entry point for DOM primitives. Delegates to extracted self-contained modules:
 *   - dom-primitives-read.ts (get_text, get_value, get_attribute, wait_for, wait_for_text, wait_for_absent)
 *   - dom-primitives-action.ts (click, type, select, check, set_attribute, paste, key_press, hover, focus, scroll_to)
 *   - dom-primitives-intent.ts (open_composer, submit_active_composer, confirm_top_dialog)
 *   - dom-primitives-overlay.ts (dismiss_top_overlay, auto_dismiss_overlays)
 *   - dom-primitives-stability.ts (wait_for_stable, action_diff)
 *   - dom-primitives-list-interactive.ts (list_interactive)
 *   - dom-primitives-query.ts (query)
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { DOMPrimitiveOptions, DOMResult } from './dom-types.js';
export { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js';
export { domPrimitiveRead } from './dom-primitives-read.js';
export { domPrimitiveAction } from './dom-primitives-action.js';
/**
 * Unified dispatcher that delegates to the appropriate self-contained primitive.
 * Used by domWaitFor and backward-compatible call sites.
 * For production dispatch, dom-dispatch.ts routes directly to specific primitives.
 */
export declare function domPrimitive(action: string, selector: string, options: DOMPrimitiveOptions): DOMResult | Promise<DOMResult> | {
    success: boolean;
    elements: unknown[];
    candidate_count?: number;
    scope_rect_used?: {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    error?: string;
    message?: string;
};
/**
 * Backward-compatible wait helper used by unit tests and legacy call sites.
 * Polls wait_for and listens for DOM mutations for fast resolution.
 */
export declare function domWaitFor(selector: string, timeoutMs?: number): Promise<DOMResult>;
//# sourceMappingURL=dom-primitives.d.ts.map