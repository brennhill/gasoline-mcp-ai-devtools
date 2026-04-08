/**
 * Purpose: Self-contained DOM primitives for read/wait actions (get_text, get_value, get_attribute, wait_for, wait_for_text, wait_for_absent).
 * Why: Extracted from dom-primitives.ts to keep file sizes under the 800 LOC limit.
 *      These actions are read-only and use simplified (non-ambiguity) element resolution.
 * Docs: docs/features/feature/interact-explore/index.md
 */
import type { DOMPrimitiveOptions, DOMResult } from './dom-types.js';
/**
 * Self-contained function for read-only DOM primitives and wait_for actions.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveRead, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export declare function domPrimitiveRead(action: string, selector: string, options: DOMPrimitiveOptions): DOMResult;
//# sourceMappingURL=dom-primitives-read.d.ts.map