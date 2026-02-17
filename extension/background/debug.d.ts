/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Debug Logging Utilities
 * Standalone module to avoid circular dependencies.
 */
/** Log categories for debug output */
export declare const DebugCategory: {
    CONNECTION: "connection";
    CAPTURE: "capture";
    ERROR: "error";
    LIFECYCLE: "lifecycle";
    SETTINGS: "settings";
    SOURCEMAP: "sourcemap";
    QUERY: "query";
};
export type DebugCategoryType = (typeof DebugCategory)[keyof typeof DebugCategory];
//# sourceMappingURL=debug.d.ts.map