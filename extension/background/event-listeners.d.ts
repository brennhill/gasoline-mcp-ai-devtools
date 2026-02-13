/**
 * @fileoverview Event Listeners - Handles Chrome alarms, tab listeners,
 * storage change listeners, and other Chrome extension events.
 */
export declare const ALARM_NAMES: {
    readonly RECONNECT: "reconnect";
    readonly ERROR_GROUP_FLUSH: "errorGroupFlush";
    readonly MEMORY_CHECK: "memoryCheck";
    readonly ERROR_GROUP_CLEANUP: "errorGroupCleanup";
};
export type AlarmName = (typeof ALARM_NAMES)[keyof typeof ALARM_NAMES];
/**
 * Setup Chrome alarms for periodic tasks
 *
 * RATE LIMITING & DoS PROTECTION:
 * 1. RECONNECT (5s): Maintains MCP connection with exponential backoff
 * 2. ERROR_GROUP_FLUSH (30s): Deduplicates errors, reduces server load
 * 3. MEMORY_CHECK (30s): Monitors buffer memory, prevents exhaustion
 * 4. ERROR_GROUP_CLEANUP (10min): Removes stale deduplication state
 *
 * Note: Alarms are re-created on service worker startup (not persistent)
 * If service worker restarts, alarms must be recreated by this function
 */
export declare function setupChromeAlarms(): void;
/**
 * Install Chrome alarm listener
 */
export declare function installAlarmListener(handlers: {
    onReconnect: () => void;
    onErrorGroupFlush: () => void;
    onMemoryCheck: () => void;
    onErrorGroupCleanup: () => void;
}): void;
/**
 * Install tab removed listener
 */
export declare function installTabRemovedListener(onTabRemoved: (tabId: number) => void): void;
/**
 * Install tab updated listener to track URL changes
 */
export declare function installTabUpdatedListener(onTabUpdated: (tabId: number, newUrl: string) => void): void;
/**
 * Handle tracked tab URL change
 * Updates the stored URL when the tracked tab navigates
 */
export declare function handleTrackedTabUrlChange(updatedTabId: number, newUrl: string, logFn?: (message: string) => void): void;
/**
 * Handle tracked tab being closed
 * SECURITY: Clears ephemeral tracking state when tab closes
 * Uses session storage for ephemeral tab tracking data
 */
export declare function handleTrackedTabClosed(closedTabId: number, logFn?: (message: string, data?: unknown) => void): void;
/**
 * Install storage change listener
 */
export declare function installStorageChangeListener(handlers: {
    onAiWebPilotChanged?: (newValue: boolean) => void;
    onTrackedTabChanged?: (newTabId: number | null, oldTabId: number | null) => void;
}): void;
/**
 * Install browser startup listener (clears tracking state)
 */
export declare function installStartupListener(logFn?: (message: string) => void): void;
/**
 * Install keyboard shortcut listener for draw mode toggle (Ctrl+Shift+D / Cmd+Shift+D).
 * Sends GASOLINE_DRAW_MODE_START or GASOLINE_DRAW_MODE_STOP to the active tab's content script.
 */
export declare function installDrawModeCommandListener(logFn?: (message: string) => void): void;
/**
 * Ping content script to check if it's loaded
 */
export declare function pingContentScript(tabId: number, timeoutMs?: number): Promise<boolean>;
/**
 * Wait for tab to finish loading
 */
export declare function waitForTabLoad(tabId: number, timeoutMs?: number): Promise<boolean>;
/**
 * Forward a message to all content scripts
 */
export declare function forwardToAllContentScripts(message: {
    type: string;
    [key: string]: unknown;
}, debugLogFn?: (category: string, message: string, data?: unknown) => void): void;
/**
 * Load saved settings from chrome.storage.local
 */
export declare function loadSavedSettings(callback: (settings: {
    serverUrl?: string;
    logLevel?: string;
    screenshotOnError?: boolean;
    sourceMapEnabled?: boolean;
    debugMode?: boolean;
}) => void): void;
/**
 * Load AI Web Pilot enabled state from storage
 */
export declare function loadAiWebPilotState(callback: (enabled: boolean) => void, logFn?: (message: string) => void): void;
/**
 * Load debug mode state from storage
 */
export declare function loadDebugModeState(callback: (enabled: boolean) => void): void;
/**
 * Save setting to chrome.storage.local
 */
export declare function saveSetting(key: string, value: unknown): void;
/** Tracked tab info type */
export interface TrackedTabInfo {
    trackedTabId: number | null;
    trackedTabUrl: string | null;
    trackedTabTitle: string | null;
}
/**
 * Get tracked tab information (callback-based for compatibility with pre-async event listeners)
 */
export declare function getTrackedTabInfo(): Promise<TrackedTabInfo>;
export declare function getTrackedTabInfo(callback: (info: TrackedTabInfo) => void): void;
/**
 * Clear tracked tab state
 */
export declare function clearTrackedTab(): void;
/**
 * Get all extension config settings
 */
export declare function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>>;
export declare function getAllConfigSettings(callback: (settings: Record<string, boolean | string | undefined>) => void): void;
//# sourceMappingURL=event-listeners.d.ts.map