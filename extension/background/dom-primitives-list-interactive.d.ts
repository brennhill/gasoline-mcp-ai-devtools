/**
 * Self-contained function that scans a page for interactive elements.
 * Passed to chrome.scripting.executeScript({ func: domPrimitiveListInteractive }).
 * MUST NOT reference any module-level variables.
 */
export declare function domPrimitiveListInteractive(): {
    success: boolean;
    elements: unknown[];
};
//# sourceMappingURL=dom-primitives-list-interactive.d.ts.map