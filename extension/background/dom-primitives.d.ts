/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
import type { DOMPrimitiveOptions, DOMResult } from './dom-types';
export { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js';
/**
 * Single self-contained function for all DOM primitives.
 * Passed to chrome.scripting.executeScript({ func: domPrimitive, args: [...] }).
 * MUST NOT reference any module-level variables — Chrome serializes the function source only.
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
export declare function domWaitFor(selector: string, timeoutMs?: number): Promise<DOMResult>;
//# sourceMappingURL=dom-primitives.d.ts.map