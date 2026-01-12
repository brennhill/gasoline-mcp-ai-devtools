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
import { initTabTracking } from './content/tab-tracking.js';
import { initScriptInjection } from './content/script-injection.js';
import { initRequestTracking, getPendingRequestStats, clearPendingRequests } from './content/request-tracking.js';
import { initWindowMessageListener } from './content/window-message-listener.js';
import { initRuntimeMessageListener } from './content/runtime-message-listener.js';
// Export for testing
export { getPendingRequestStats, clearPendingRequests };
// ============================================================================
// INITIALIZATION
// ============================================================================
// Initialize tab tracking
initTabTracking();
// Initialize request tracking (cleanup handlers)
initRequestTracking();
// Initialize window message listener
initWindowMessageListener();
// Initialize runtime message listener
initRuntimeMessageListener();
// Initialize script injection
initScriptInjection();
//# sourceMappingURL=content.js.map