/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Why: Keeps content-script bridging predictable between extension and page contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */
/**
 * Show a brief visual toast overlay for AI actions.
 * Supports color-coded states and structured content with truncation.
 * For audio-related toasts, adds animated arrow pointing to extension icon.
 */
export declare function showActionToast(text: string, detail?: string, state?: 'trying' | 'success' | 'warning' | 'error' | 'audio', durationMs?: number): void;
//# sourceMappingURL=toast.d.ts.map