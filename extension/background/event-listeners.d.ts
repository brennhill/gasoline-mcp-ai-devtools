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
 * Handle tracked tab being closed
 */
export declare function handleTrackedTabClosed(closedTabId: number, logFn?: (message: string, data?: unknown) => void): void;
/**
 * Install storage change listener
 */
export declare function installStorageChangeListener(handlers: {
    onAiWebPilotChanged?: (newValue: boolean) => void;
    onTrackedTabChanged?: () => void;
}): void;
/**
 * Install browser startup listener (clears tracking state)
 */
export declare function installStartupListener(logFn?: (message: string) => void): void;
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
/**
 * Get tracked tab information
 */
export declare function getTrackedTabInfo(): Promise<{
    trackedTabId: number | null;
    trackedTabUrl: string | null;
}>;
/**
 * Clear tracked tab state
 */
export declare function clearTrackedTab(): void;
/**
 * Get all extension config settings
 */
export declare function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>>;
//# sourceMappingURL=event-listeners.d.ts.map