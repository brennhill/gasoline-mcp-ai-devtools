/**
 * @fileoverview Tab Tracking Module
 * Manages tracking status for the current tab
 */
// Whether this content script's tab is the currently tracked tab
let isTrackedTab = false
// The tab ID of this content script's tab
let currentTabId = null
/**
 * Update tracking status by checking storage and current tab ID.
 * Called on script load, storage changes, and tab activation.
 */
export async function updateTrackingStatus() {
  try {
    const storage = await chrome.storage.local.get(['trackedTabId'])
    // Request tab ID from background script (content scripts can't access chrome.tabs)
    const response = await chrome.runtime.sendMessage({ type: 'GET_TAB_ID' })
    currentTabId = response?.tabId ?? null
    isTrackedTab = currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId
  } catch {
    // Graceful degradation: if we can't check, assume not tracked
    isTrackedTab = false
  }
}
/**
 * Get the current tracking status
 */
export function getIsTrackedTab() {
  return isTrackedTab
}
/**
 * Get the current tab ID
 */
export function getCurrentTabId() {
  return currentTabId
}
/**
 * Initialize tab tracking (call once on script load).
 * Returns a promise that resolves when initial tracking status is known.
 * The onChange callback fires after each status update (initial + storage changes).
 */
export function initTabTracking(onChange) {
  const ready = updateTrackingStatus().then(() => {
    onChange?.(isTrackedTab)
  })
  chrome.storage.onChanged.addListener(async (changes) => {
    if (changes.trackedTabId) {
      await updateTrackingStatus()
      onChange?.(isTrackedTab)
    }
  })
  return ready
}
//# sourceMappingURL=tab-tracking.js.map
