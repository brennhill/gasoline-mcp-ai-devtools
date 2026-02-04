/**
 * @fileoverview Tab Tracking Module
 * Manages tracking status for the current tab
 */
// Whether this content script's tab is the currently tracked tab
let isTrackedTab = false;
// The tab ID of this content script's tab
let currentTabId = null;
/**
 * Update tracking status by checking storage and current tab ID.
 * Called on script load, storage changes, and tab activation.
 */
export async function updateTrackingStatus() {
    try {
        const storage = await chrome.storage.local.get(['trackedTabId']);
        // Request tab ID from background script (content scripts can't access chrome.tabs)
        const response = (await chrome.runtime.sendMessage({ type: 'GET_TAB_ID' }));
        currentTabId = response?.tabId ?? null;
        isTrackedTab = currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId;
    }
    catch {
        // Graceful degradation: if we can't check, assume not tracked
        isTrackedTab = false;
    }
}
/**
 * Get the current tracking status
 */
export function getIsTrackedTab() {
    return isTrackedTab;
}
/**
 * Get the current tab ID
 */
export function getCurrentTabId() {
    return currentTabId;
}
/**
 * Initialize tab tracking (call once on script load)
 */
export function initTabTracking() {
    // Initialize tracking status on script load
    updateTrackingStatus();
    // Listen for tracking changes in storage
    chrome.storage.onChanged.addListener((changes) => {
        if (changes.trackedTabId) {
            updateTrackingStatus();
        }
    });
}
//# sourceMappingURL=tab-tracking.js.map