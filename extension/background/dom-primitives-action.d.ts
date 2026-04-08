/**
 * Purpose: Self-contained DOM primitives for mutating selector-based actions
 *   (click, type, select, check, set_attribute, paste, key_press, hover, focus, scroll_to).
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit.
 *      These are ambiguity-sensitive actions that need full ranking + overlay detection.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { DOMPrimitiveOptions, DOMResult } from './dom-types.js';
/**
 * Self-contained function for mutating DOM actions.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveAction, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export declare function domPrimitiveAction(action: string, selector: string, options: DOMPrimitiveOptions): DOMResult | Promise<DOMResult>;
//# sourceMappingURL=dom-primitives-action.d.ts.map