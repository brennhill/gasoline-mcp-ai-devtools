/**
 * Purpose: Owns debug.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */
/**
 * @fileoverview Debug Types
 * Debug logging and categorization
 */
/**
 * Debug log categories
 */
export type DebugCategory = 'connection' | 'capture' | 'error' | 'lifecycle' | 'settings' | 'sourcemap' | 'query';
/**
 * Debug log entry
 */
export interface DebugLogEntry {
    readonly ts: string;
    readonly category: DebugCategory;
    readonly message: string;
    readonly data?: unknown;
}
//# sourceMappingURL=debug.d.ts.map