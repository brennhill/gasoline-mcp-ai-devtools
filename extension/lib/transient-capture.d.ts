/**
 * Purpose: Captures transient UI elements (toasts, alerts, snackbars) via MutationObserver.
 * Why: AI agents miss transient UI because elements disappear before the next screenshot.
 * Docs: docs/features/feature/transient-capture/index.md
 */
/**
 * Install the transient element capture MutationObserver.
 */
export declare function installTransientCapture(): void;
/**
 * Uninstall the transient element capture MutationObserver.
 */
export declare function uninstallTransientCapture(): void;
//# sourceMappingURL=transient-capture.d.ts.map