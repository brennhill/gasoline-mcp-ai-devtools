/**
 * @fileoverview Event Listeners - Handles Chrome alarms, tab listeners,
 * storage change listeners, and other Chrome extension events.
 */
// =============================================================================
// CONSTANTS
// =============================================================================
/** Reconnect interval in minutes */
const RECONNECT_INTERVAL_MINUTES = 5 / 60; // 5 seconds in minutes
/** Error group flush interval in minutes */
const ERROR_GROUP_FLUSH_INTERVAL_MINUTES = 0.5; // 30 seconds
/** Memory check interval in minutes */
const MEMORY_CHECK_INTERVAL_MINUTES = 0.5; // 30 seconds
/** Error group cleanup interval in minutes */
const ERROR_GROUP_CLEANUP_INTERVAL_MINUTES = 10;
// =============================================================================
// ALARM NAMES
// =============================================================================
export const ALARM_NAMES = {
    RECONNECT: 'reconnect',
    ERROR_GROUP_FLUSH: 'errorGroupFlush',
    MEMORY_CHECK: 'memoryCheck',
    ERROR_GROUP_CLEANUP: 'errorGroupCleanup',
};
// =============================================================================
// CHROME ALARMS
// =============================================================================
/**
 * Setup Chrome alarms for periodic tasks
 */
export function setupChromeAlarms() {
    if (typeof chrome === 'undefined' || !chrome.alarms)
        return;
    chrome.alarms.create(ALARM_NAMES.RECONNECT, { periodInMinutes: RECONNECT_INTERVAL_MINUTES });
    chrome.alarms.create(ALARM_NAMES.ERROR_GROUP_FLUSH, { periodInMinutes: ERROR_GROUP_FLUSH_INTERVAL_MINUTES });
    chrome.alarms.create(ALARM_NAMES.MEMORY_CHECK, { periodInMinutes: MEMORY_CHECK_INTERVAL_MINUTES });
    chrome.alarms.create(ALARM_NAMES.ERROR_GROUP_CLEANUP, { periodInMinutes: ERROR_GROUP_CLEANUP_INTERVAL_MINUTES });
}
/**
 * Install Chrome alarm listener
 */
export function installAlarmListener(handlers) {
    if (typeof chrome === 'undefined' || !chrome.alarms)
        return;
    chrome.alarms.onAlarm.addListener((alarm) => {
        switch (alarm.name) {
            case ALARM_NAMES.RECONNECT:
                handlers.onReconnect();
                break;
            case ALARM_NAMES.ERROR_GROUP_FLUSH:
                handlers.onErrorGroupFlush();
                break;
            case ALARM_NAMES.MEMORY_CHECK:
                handlers.onMemoryCheck();
                break;
            case ALARM_NAMES.ERROR_GROUP_CLEANUP:
                handlers.onErrorGroupCleanup();
                break;
        }
    });
}
// =============================================================================
// TAB LISTENERS
// =============================================================================
/**
 * Install tab removed listener
 */
export function installTabRemovedListener(onTabRemoved) {
    if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.onRemoved)
        return;
    chrome.tabs.onRemoved.addListener((tabId) => {
        onTabRemoved(tabId);
    });
}
/**
 * Handle tracked tab being closed
 */
export function handleTrackedTabClosed(closedTabId, logFn) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    chrome.storage.local.get(['trackedTabId'], (result) => {
        if (result.trackedTabId === closedTabId) {
            if (logFn)
                logFn('[Gasoline] Tracked tab closed (id:', closedTabId);
            chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl']);
        }
    });
}
// =============================================================================
// STORAGE LISTENERS
// =============================================================================
/**
 * Install storage change listener
 */
export function installStorageChangeListener(handlers) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    chrome.storage.onChanged.addListener((changes, areaName) => {
        if (areaName === 'local') {
            if (changes.aiWebPilotEnabled && handlers.onAiWebPilotChanged) {
                handlers.onAiWebPilotChanged(changes.aiWebPilotEnabled.newValue === true);
            }
            if (changes.trackedTabId && handlers.onTrackedTabChanged) {
                handlers.onTrackedTabChanged();
            }
        }
    });
}
// =============================================================================
// RUNTIME LISTENERS
// =============================================================================
/**
 * Install browser startup listener (clears tracking state)
 */
export function installStartupListener(logFn) {
    if (typeof chrome === 'undefined' || !chrome.runtime || !chrome.runtime.onStartup)
        return;
    chrome.runtime.onStartup.addListener(() => {
        if (logFn)
            logFn('[Gasoline] Browser restarted - clearing tracking state');
        chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl']);
    });
}
// =============================================================================
// CONTENT SCRIPT HELPERS
// =============================================================================
/**
 * Ping content script to check if it's loaded
 */
export async function pingContentScript(tabId, timeoutMs = 500) {
    try {
        const response = await Promise.race([
            chrome.tabs.sendMessage(tabId, { type: 'GASOLINE_PING' }),
            new Promise((_, reject) => {
                setTimeout(() => reject(new Error('timeout')), timeoutMs);
            }),
        ]);
        return response?.status === 'alive';
    }
    catch {
        return false;
    }
}
/**
 * Wait for tab to finish loading
 */
export async function waitForTabLoad(tabId, timeoutMs = 5000) {
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
            setTimeout(r, 100);
        });
    }
    return false;
}
/**
 * Forward a message to all content scripts
 */
export function forwardToAllContentScripts(message, debugLogFn) {
    if (typeof chrome === 'undefined' || !chrome.tabs)
        return;
    chrome.tabs.query({}, (tabs) => {
        for (const tab of tabs) {
            if (tab.id) {
                chrome.tabs.sendMessage(tab.id, message).catch((err) => {
                    if (!err.message?.includes('Receiving end does not exist') &&
                        !err.message?.includes('Could not establish connection')) {
                        if (debugLogFn) {
                            debugLogFn('error', 'Unexpected error forwarding setting to tab', {
                                tabId: tab.id,
                                error: err.message,
                            });
                        }
                    }
                });
            }
        }
    });
}
// =============================================================================
// SETTINGS LOADING
// =============================================================================
/**
 * Load saved settings from chrome.storage.local
 */
export function loadSavedSettings(callback) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        callback({});
        return;
    }
    chrome.storage.local.get(['serverUrl', 'logLevel', 'screenshotOnError', 'sourceMapEnabled', 'debugMode'], (result) => {
        if (chrome.runtime.lastError) {
            console.warn('[Gasoline] Could not load saved settings:', chrome.runtime.lastError.message, '- using defaults');
            callback({});
            return;
        }
        callback(result);
    });
}
/**
 * Load AI Web Pilot enabled state from storage
 */
export function loadAiWebPilotState(callback, logFn) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        callback(false);
        return;
    }
    const startTime = performance.now();
    chrome.storage.local.get(['aiWebPilotEnabled'], (result) => {
        const wasLoaded = result.aiWebPilotEnabled === true;
        const loadTime = performance.now() - startTime;
        if (logFn) {
            logFn(`[Gasoline] AI Web Pilot loaded on startup: ${wasLoaded} (took ${loadTime.toFixed(1)}ms)`);
        }
        callback(wasLoaded);
    });
}
/**
 * Load debug mode state from storage
 */
export function loadDebugModeState(callback) {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        callback(false);
        return;
    }
    chrome.storage.local.get(['debugMode'], (result) => {
        callback(result.debugMode === true);
    });
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
 * Get tracked tab information
 */
export async function getTrackedTabInfo() {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        return { trackedTabId: null, trackedTabUrl: null };
    }
    return new Promise((resolve) => {
        chrome.storage.local.get(['trackedTabId', 'trackedTabUrl'], (result) => {
            resolve({
                trackedTabId: result.trackedTabId || null,
                trackedTabUrl: result.trackedTabUrl || null,
            });
        });
    });
}
/**
 * Clear tracked tab state
 */
export function clearTrackedTab() {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl']);
}
/**
 * Get all extension config settings
 */
export async function getAllConfigSettings() {
    if (typeof chrome === 'undefined' || !chrome.storage) {
        return {};
    }
    return new Promise((resolve) => {
        chrome.storage.local.get([
            'aiWebPilotEnabled',
            'webSocketCaptureEnabled',
            'networkWaterfallEnabled',
            'performanceMarksEnabled',
            'actionReplayEnabled',
            'screenshotOnError',
            'sourceMapEnabled',
            'networkBodyCaptureEnabled',
        ], (result) => {
            resolve(result);
        });
    });
}
//# sourceMappingURL=event-listeners.js.map