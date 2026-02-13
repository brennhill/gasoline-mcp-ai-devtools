/**
 * @fileoverview options.ts — Extension settings page for user-configurable options.
 * Manages server URL, domain filters (allowlist/blocklist), screenshot-on-error toggle,
 * source map resolution toggle, and interception deferral toggle.
 * Persists settings via chrome.storage.local and notifies the background worker
 * of changes so they take effect without requiring extension reload.
 * Design: Toggle controls use CSS class 'active' for state. Domain filters are
 * stored as newline-separated strings, parsed to arrays on save.
 */
interface ExportResult {
    success: boolean;
    filename?: string;
    error?: string;
}
interface ClearLogResponse {
    success?: boolean;
    error?: string;
}
/**
 * Load saved options
 */
export declare function loadOptions(): void;
/**
 * Save options to storage and notify background
 * ARCHITECTURE: Options page writes to storage directly (for immediate persistence),
 * then sends messages to background so it can update its internal state.
 * Background is the authoritative source of truth for actual behavior.
 * Example: debugMode=true in storage enables logging immediately, AND background
 * updates its debugMode variable so new logs use the new setting.
 */
export declare function saveOptions(): void;
/**
 * Toggle deferral setting
 */
export declare function toggleDeferral(): void;
/**
 * Toggle debug mode setting
 */
export declare function toggleDebugMode(): void;
/**
 * Toggle theme between dark (default) and light
 */
export declare function toggleTheme(): void;
/**
 * Test connection to server
 */
export declare function testConnection(): Promise<void>;
/**
 * Export debug log to a downloadable file
 */
export declare function handleExportDebugLog(): Promise<ExportResult>;
/**
 * Clear the debug log buffer
 */
export declare function handleClearDebugLog(): Promise<ClearLogResponse>;
export {};
//# sourceMappingURL=options.d.ts.map