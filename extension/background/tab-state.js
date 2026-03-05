/**
 * Purpose: Tab-state accessors, settings persistence, and content-script helpers.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */
import { scaleTimeout } from '../lib/timeouts.js';
import { StorageKey } from '../lib/constants.js';
// =============================================================================
// CONTENT SCRIPT HELPERS
// =============================================================================
/**
 * Ping content script to check if it's loaded
 */
export async function pingContentScript(tabId, timeoutMs = scaleTimeout(500)) {
    try {
        const response = (await Promise.race([
            chrome.tabs.sendMessage(tabId, { type: 'GASOLINE_PING' }),
            new Promise((_, reject) => {
                setTimeout(() => reject(new Error(`Content script ping timeout after ${timeoutMs}ms on tab ${tabId}`)), timeoutMs);
            })
        ]));
        return response?.status === 'alive';
    }
    catch {
        return false;
    }
}
/**
 * Wait for tab to finish loading
 */
export async function waitForTabLoad(tabId, timeoutMs = scaleTimeout(5000)) {
    const startTime = Date.now();
    while (Date.now() - startTime < timeoutMs) {
        try {
            const tab = await chrome.tabs.get(tabId);
            if (tab.status === 'complete')
                return true;
        }
        catch {
            return false;
        }
        await new Promise((r) => {
            setTimeout(r, scaleTimeout(100));
        });
    }
    return false;
}
/**
 * Forward a message to all content scripts
 */
export async function forwardToAllContentScripts(message, debugLogFn) {
    if (typeof chrome === 'undefined' || !chrome.tabs)
        return;
    const tabs = await chrome.tabs.query({});
    for (const tab of tabs) {
        if (tab.id) {
            chrome.tabs.sendMessage(tab.id, message).catch((err) => {
                if (!err.message?.includes('Receiving end does not exist') &&
                    !err.message?.includes('Could not establish connection')) {
                    if (debugLogFn) {
                        debugLogFn('error', 'Unexpected error forwarding setting to tab', {
                            tabId: tab.id,
                            error: err.message
                        });
                    }
                }
            });
        }
    }
}
/**
 * Load saved settings from chrome.storage.local
 */
export async function loadSavedSettings() {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        return {};
    }
    try {
        const result = (await chrome.storage.local.get([
            StorageKey.SERVER_URL,
            StorageKey.LOG_LEVEL,
            StorageKey.SCREENSHOT_ON_ERROR,
            StorageKey.SOURCE_MAP_ENABLED,
            StorageKey.DEBUG_MODE
        ]));
        return result;
    }
    catch {
        console.warn('[Gasoline] Could not load saved settings - using defaults');
        return {};
    }
}
/**
 * Load AI Web Pilot enabled state from storage
 */
export async function loadAiWebPilotState(logFn) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        return false;
    }
    const startTime = performance.now();
    const result = (await chrome.storage.local.get([StorageKey.AI_WEB_PILOT_ENABLED]));
    const wasLoaded = result.aiWebPilotEnabled !== false;
    const loadTime = performance.now() - startTime;
    if (logFn) {
        logFn(`[Gasoline] AI Web Pilot loaded on startup: ${wasLoaded} (took ${loadTime.toFixed(1)}ms)`);
    }
    return wasLoaded;
}
/**
 * Load debug mode state from storage
 */
export async function loadDebugModeState() {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        return false;
    }
    const result = (await chrome.storage.local.get([StorageKey.DEBUG_MODE]));
    return result.debugMode === true;
}
/**
 * Save setting to chrome.storage.local
 */
export function saveSetting(key, value) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    chrome.storage.local.set({ [key]: value });
}
/**
 * Get tracked tab information, including Chrome tab status.
 */
export async function getTrackedTabInfo() {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        return { trackedTabId: null, trackedTabUrl: null, trackedTabTitle: null, tabStatus: null, trackedTabActive: null };
    }
    const result = (await chrome.storage.local.get([
        StorageKey.TRACKED_TAB_ID,
        StorageKey.TRACKED_TAB_URL,
        StorageKey.TRACKED_TAB_TITLE
    ]));
    const tabId = result.trackedTabId || null;
    let tabStatus = null;
    let trackedTabActive = null;
    // Query Chrome tab API for live tab status if we have a tracked tab
    if (tabId && typeof chrome !== 'undefined' && chrome.tabs) {
        try {
            const tab = await chrome.tabs.get(tabId);
            if (tab.status === 'loading' || tab.status === 'complete') {
                tabStatus = tab.status;
            }
            trackedTabActive = !!tab.active;
        }
        catch {
            // Tab may have been closed -- tabStatus stays null
        }
    }
    return {
        trackedTabId: tabId,
        trackedTabUrl: result.trackedTabUrl || null,
        trackedTabTitle: result.trackedTabTitle || null,
        tabStatus,
        trackedTabActive
    };
}
/**
 * Clear tracked tab state
 */
export function clearTrackedTab() {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    chrome.storage.local.remove([StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL, StorageKey.TRACKED_TAB_TITLE]);
}
/**
 * Get all extension config settings.
 */
export async function getAllConfigSettings() {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        return {};
    }
    const result = (await chrome.storage.local.get([
        StorageKey.AI_WEB_PILOT_ENABLED,
        StorageKey.WEBSOCKET_CAPTURE_ENABLED,
        StorageKey.NETWORK_WATERFALL_ENABLED,
        StorageKey.PERFORMANCE_MARKS_ENABLED,
        StorageKey.ACTION_REPLAY_ENABLED,
        StorageKey.SCREENSHOT_ON_ERROR,
        StorageKey.SOURCE_MAP_ENABLED,
        StorageKey.NETWORK_BODY_CAPTURE_ENABLED
    ]));
    return result;
}
//# sourceMappingURL=tab-state.js.map