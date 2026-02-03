/**
 * @fileoverview content.ts - Message bridge between page and extension contexts.
 * Injects inject.js into the page as a module script, then listens for
 * window.postMessage events (GASOLINE_LOG, GASOLINE_WS, GASOLINE_NETWORK_BODY,
 * GASOLINE_ENHANCED_ACTION, GASOLINE_PERF_SNAPSHOT) and forwards them to the
 * background service worker via chrome.runtime.sendMessage.
 * Also handles chrome.runtime messages for on-demand queries (DOM, a11y, perf).
 * Design: Tab-scoped filtering - only forwards messages from the explicitly
 * tracked tab. Validates message origin (event.source === window) to prevent
 * cross-frame injection. Attaches tabId to all forwarded messages.
 */

import { initTabTracking, getIsTrackedTab } from './content/tab-tracking'
import { initScriptInjection } from './content/script-injection'
import { initRequestTracking, getPendingRequestStats, clearPendingRequests } from './content/request-tracking'
import { initWindowMessageListener } from './content/window-message-listener'
import { initRuntimeMessageListener } from './content/runtime-message-listener'
import { initFaviconReplacer } from './content/favicon-replacer'

// Export for testing
export { getPendingRequestStats, clearPendingRequests }

// ============================================================================
// INITIALIZATION
// ============================================================================

// Track whether scripts have been injected
let scriptsInjected = false

// Initialize tab tracking first
initTabTracking()

// Initialize request tracking (cleanup handlers)
initRequestTracking()

// Initialize window message listener
initWindowMessageListener()

// Initialize runtime message listener
initRuntimeMessageListener()

// Initialize favicon replacer (visual indicator for tracked tabs)
initFaviconReplacer()

// Listen for tracking status changes and inject scripts only when tracked
chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    // Check if this tab is now tracked
    if (getIsTrackedTab() && !scriptsInjected) {
      // Tab just became tracked - inject scripts
      initScriptInjection()
      scriptsInjected = true
    }
    // Note: We don't remove scripts when tab becomes untracked
    // because that could break the page. Just stop injecting on new tracked tabs.
  }
})

// Check initial tracking status and inject if already tracked
// Use setTimeout to ensure initTabTracking() has completed its async work
setTimeout(() => {
  if (getIsTrackedTab() && !scriptsInjected) {
    initScriptInjection()
    scriptsInjected = true
  }
}, 100)
