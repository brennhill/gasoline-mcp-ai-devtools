/**
 * Purpose: Owns content.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/analyze-tool/index.md
 */

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

import { initTabTracking } from './content/tab-tracking'
import { initScriptInjection } from './content/script-injection'
import {
  initRequestTracking,
  getPendingRequestStats,
  clearPendingRequests,
  cleanupRequestTracking
} from './content/request-tracking'
import { initWindowMessageListener } from './content/window-message-listener'
import { initRuntimeMessageListener } from './content/runtime-message-listener'
import { initFaviconReplacer } from './content/favicon-replacer'

// Export for testing
export { getPendingRequestStats, clearPendingRequests, cleanupRequestTracking }

// ============================================================================
// INITIALIZATION
// ============================================================================

// Track whether scripts have been injected
let scriptsInjected = false

// Initialize tab tracking first, with callback for injection
initTabTracking((tracked) => {
  if (tracked && !scriptsInjected) {
    initScriptInjection()
    scriptsInjected = true
  }
})

// Initialize request tracking (cleanup handlers)
initRequestTracking()

// Initialize window message listener
initWindowMessageListener()

// Initialize runtime message listener
initRuntimeMessageListener()

// Initialize favicon replacer (visual indicator for tracked tabs)
initFaviconReplacer()
