/**
 * Purpose: Tab-state accessors, settings persistence, and content-script helpers.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */
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
}, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<void>;
/** Settings returned by loadSavedSettings */
export interface SavedSettings {
    serverUrl?: string;
    logLevel?: string;
    screenshotOnError?: boolean;
    sourceMapEnabled?: boolean;
    debugMode?: boolean;
}
/**
 * Load saved settings from chrome.storage.local
 */
export declare function loadSavedSettings(): Promise<SavedSettings>;
/**
 * Load AI Web Pilot enabled state from storage
 */
export declare function loadAiWebPilotState(logFn?: (message: string) => void): Promise<boolean>;
/**
 * Load debug mode state from storage
 */
export declare function loadDebugModeState(): Promise<boolean>;
/**
 * Save setting to chrome.storage.local
 */
export declare function saveSetting(key: string, value: unknown): void;
/** Tracked tab info type */
export interface TrackedTabInfo {
    trackedTabId: number | null;
    trackedTabUrl: string | null;
    trackedTabTitle: string | null;
    tabStatus: 'loading' | 'complete' | null;
    trackedTabActive: boolean | null;
}
/**
 * Get tracked tab information, including Chrome tab status.
 */
export declare function getTrackedTabInfo(): Promise<TrackedTabInfo>;
/**
 * Clear tracked tab state
 */
export declare function clearTrackedTab(): void;
/**
 * Get all extension config settings.
 */
export declare function getAllConfigSettings(): Promise<Record<string, boolean | string | undefined>>;
//# sourceMappingURL=tab-state.d.ts.map