/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * Self-contained function that scans a page for interactive elements.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveListInteractive }).
 * MUST NOT reference any module-level variables.
 */
export declare function domPrimitiveListInteractive(scopeSelector?: string, options?: {
    scope_rect?: {
        x?: unknown;
        y?: unknown;
        width?: unknown;
        height?: unknown;
    };
}): {
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
//# sourceMappingURL=dom-primitives-list-interactive.d.ts.map