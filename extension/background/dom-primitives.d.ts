/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import type { DOMPrimitiveOptions, DOMResult } from './dom-types';
/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
 */
export declare function domPrimitive(action: string, selector: string, options: DOMPrimitiveOptions): DOMResult | Promise<DOMResult> | {
    success: boolean;
    elements: unknown[];
};
/**
 * wait_for variant that polls with MutationObserver (used when element not found initially).
 * This stays as a local convenience wrapper (tests and direct calls).
 * Runtime dispatch uses repeated domPrimitive('wait_for', ...) executeScript calls
 * so injected code only relies on one self-contained selector engine.
 */
export declare function domWaitFor(selector: string, timeoutMs: number): Promise<DOMResult>;
//# sourceMappingURL=dom-primitives.d.ts.map