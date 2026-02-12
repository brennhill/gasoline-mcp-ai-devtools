/**
 * @fileoverview Exception and unhandled rejection capture.
 * Monkey-patches window.onerror and listens for unhandledrejection events,
 * enriching errors with AI context before posting via bridge.
 */
export declare function installExceptionCapture(): void;
/**
 * Uninstall exception capture
 */
export declare function uninstallExceptionCapture(): void;
//# sourceMappingURL=exceptions.d.ts.map