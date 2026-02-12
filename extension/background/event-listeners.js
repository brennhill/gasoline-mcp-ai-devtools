/**
 * @fileoverview Event Listeners - Handles Chrome alarms, tab listeners,
 * storage change listeners, and other Chrome extension events.
 */
// =============================================================================
// CONSTANTS - Rate Limiting & DoS Protection
// =============================================================================
/**
 * Reconnect interval: 5 seconds
 * DoS Protection: If MCP server is down, we check every 5s (circuit breaker
 * will back off exponentially if failures continue).
 * Ensures connection restored quickly when server comes back up.
 */
const RECONNECT_INTERVAL_MINUTES = 5 / 60; // 5 seconds in minutes
/**
 * Error group flush interval: 30 seconds
 * DoS Protection: Deduplicates identical errors within a 5-second window
 * before sending to server. Reduces network traffic and API quota usage.
 * Flushed every 30 seconds to keep errors reasonably fresh.
 */
const ERROR_GROUP_FLUSH_INTERVAL_MINUTES = 0.5; // 30 seconds
/**
 * Memory check interval: 30 seconds
 * DoS Protection: Monitors estimated buffer memory and triggers circuit breaker
 * if soft limit (20MB) or hard limit (50MB) is exceeded.
 * Prevents memory exhaustion from unbounded capture buffer growth.
 */
const MEMORY_CHECK_INTERVAL_MINUTES = 0.5; // 30 seconds
/**
 * Error group cleanup interval: 10 minutes
 * DoS Protection: Removes stale error group deduplication state that is >5min old.
 * Prevents unbounded growth of error group metadata.
 */
const ERROR_GROUP_CLEANUP_INTERVAL_MINUTES = 10;
// =============================================================================
// ALARM NAMES
// =============================================================================
export const ALARM_NAMES = {
    RECONNECT: 'reconnect',
    ERROR_GROUP_FLUSH: 'errorGroupFlush',
    MEMORY_CHECK: 'memoryCheck',
    ERROR_GROUP_CLEANUP: 'errorGroupCleanup'
};
// =============================================================================
// CHROME ALARMS
// =============================================================================
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
 * Install tab updated listener to track URL changes
 */
export function installTabUpdatedListener(onTabUpdated) {
    if (typeof chrome === 'undefined' || !chrome.tabs || !chrome.tabs.onUpdated)
        return;
    chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
        // Only care about URL changes
        if (changeInfo.url) {
            onTabUpdated(tabId, changeInfo.url);
        }
    });
}
/**
 * Handle tracked tab URL change
 * Updates the stored URL when the tracked tab navigates
 */
export function handleTrackedTabUrlChange(updatedTabId, newUrl, logFn) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    chrome.storage.local.get(['trackedTabId'], (result) => {
        if (result.trackedTabId === updatedTabId) {
            chrome.storage.local.set({ trackedTabUrl: newUrl }, () => {
                if (logFn) {
                    logFn('[Gasoline] Tracked tab URL updated: ' + newUrl);
                }
            });
        }
    });
}
/**
 * Handle tracked tab being closed
 * SECURITY: Clears ephemeral tracking state when tab closes
 * Uses session storage for ephemeral tab tracking data
 */
export function handleTrackedTabClosed(closedTabId, logFn) {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    chrome.storage.local.get(['trackedTabId'], (result) => {
        if (result.trackedTabId === closedTabId) {
            if (logFn)
                logFn('[Gasoline] Tracked tab closed (id:', closedTabId);
            chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl', 'trackedTabTitle']);
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
                const newTabId = changes.trackedTabId.newValue ?? null;
                const oldTabId = changes.trackedTabId.oldValue ?? null;
                handlers.onTrackedTabChanged(newTabId, oldTabId);
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
    chrome.runtime.onStartup.addListener(async () => {
        try {
            const result = await chrome.storage.local.get(['trackedTabId']);
            const trackedTabId = result.trackedTabId;
            if (trackedTabId) {
                try {
                    await chrome.tabs.get(trackedTabId);
                    if (logFn)
                        logFn('[Gasoline] Browser restarted - tracked tab still exists, keeping tracking');
                }
                catch {
                    if (logFn)
                        logFn('[Gasoline] Browser restarted - tracked tab gone, clearing tracking state');
                    chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl', 'trackedTabTitle']);
                }
            }
        }
        catch {
            // Safety fallback: clear if we can't check
            chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl', 'trackedTabTitle']);
        }
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
                                error: err.message
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
        const wasLoaded = result.aiWebPilotEnabled !== false;
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
// Implementation
export function getTrackedTabInfo(callback) {
    if (!callback) {
        // Promise-based version
        return new Promise((resolve) => {
            getTrackedTabInfo((info) => resolve(info));
        });
    }
    // Callback-based version
    if (typeof chrome === 'undefined' || !chrome.storage) {
        callback({ trackedTabId: null, trackedTabUrl: null, trackedTabTitle: null });
        return;
    }
    chrome.storage.local.get(['trackedTabId', 'trackedTabUrl', 'trackedTabTitle'], (result) => {
        callback({
            trackedTabId: result.trackedTabId || null,
            trackedTabUrl: result.trackedTabUrl || null,
            trackedTabTitle: result.trackedTabTitle || null
        });
    });
}
/**
 * Clear tracked tab state
 */
export function clearTrackedTab() {
    if (typeof chrome === 'undefined' || !chrome.storage)
        return;
    chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl', 'trackedTabTitle']);
}
// Implementation
export function getAllConfigSettings(callback) {
    if (!callback) {
        // Promise-based version
        return new Promise((resolve) => {
            getAllConfigSettings((settings) => resolve(settings));
        });
    }
    // Callback-based version
    if (typeof chrome === 'undefined' || !chrome.storage) {
        callback({});
        return;
    }
    chrome.storage.local.get([
        'aiWebPilotEnabled',
        'webSocketCaptureEnabled',
        'networkWaterfallEnabled',
        'performanceMarksEnabled',
        'actionReplayEnabled',
        'screenshotOnError',
        'sourceMapEnabled',
        'networkBodyCaptureEnabled'
    ], (result) => {
        callback(result);
    });
}
//# sourceMappingURL=event-listeners.js.map