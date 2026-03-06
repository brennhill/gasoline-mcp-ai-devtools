/**
 * Purpose: Tab-state accessors, settings persistence, and content-script helpers.
 * Split from event-listeners.ts to keep files under 800 LOC.
 */
import { scaleTimeout } from '../lib/timeouts.js';
import { delay } from '../lib/timeout-utils.js';
import { StorageKey } from '../lib/constants.js';
import { getLocal, getLocals, setLocal, setLocals, removeLocals } from '../lib/storage-utils.js';
// =============================================================================
// CONTENT SCRIPT HELPERS
// =============================================================================
/**
 * Ping content script to check if it's loaded
 */
export async function pingContentScript(tabId, timeoutMs = scaleTimeout(500)) {
    try {
        const response = (await Promise.race([
            chrome.tabs.sendMessage(tabId, { type: 'gasoline_ping' }),
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
        await delay(scaleTimeout(100));
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
    try {
        const result = (await getLocals([
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
    const startTime = performance.now();
    const aiEnabled = await getLocal(StorageKey.AI_WEB_PILOT_ENABLED);
    const wasLoaded = aiEnabled !== false;
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
    const debugMode = await getLocal(StorageKey.DEBUG_MODE);
    return debugMode === true;
}
/**
 * Save setting to chrome.storage.local
 */
export function saveSetting(key, value) {
    setLocal(key, value);
}
const TRACKED_TAB_STORAGE_KEYS = [StorageKey.TRACKED_TAB_ID, StorageKey.TRACKED_TAB_URL, StorageKey.TRACKED_TAB_TITLE];
/**
 * Get tracked tab information, including Chrome tab status.
 */
export async function getTrackedTabInfo() {
    const result = (await getLocals(TRACKED_TAB_STORAGE_KEYS));
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
 * Persist tracked tab state.
 */
export async function setTrackedTab(tab) {
    if (!tab.id)
        return;
    await setLocals({
        [StorageKey.TRACKED_TAB_ID]: tab.id,
        [StorageKey.TRACKED_TAB_URL]: tab.url ?? '',
        [StorageKey.TRACKED_TAB_TITLE]: tab.title ?? ''
    });
}
/**
 * Clear tracked tab state
 */
export function clearTrackedTab() {
    removeLocals(TRACKED_TAB_STORAGE_KEYS);
}
/**
 * Get all extension config settings.
 */
export async function getAllConfigSettings() {
    const result = (await getLocals([
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
// =============================================================================
// ACTIVE TAB LOOKUP
// =============================================================================
/**
 * Query for the currently active tab in the current window.
 * Returns null if no active tab or no tab id.
 */
export async function getActiveTab() {
    const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true });
    const tab = activeTabs[0];
    if (!tab?.id) {
        return null;
    }
    return tab;
}
// =============================================================================
// TAB TOAST
// =============================================================================
/**
 * Send a gasoline_action_toast message to a tab.
 * Silently ignores errors (content script may not be loaded).
 */
export function sendTabToast(tabId, text, detail = '', state = 'success', duration_ms = 3000) {
    chrome.tabs
        .sendMessage(tabId, {
        type: 'gasoline_action_toast',
        text,
        detail,
        state,
        duration_ms
    })
        .catch(() => {
        /* content script may not be loaded */
    });
}
//# sourceMappingURL=tab-state.js.map