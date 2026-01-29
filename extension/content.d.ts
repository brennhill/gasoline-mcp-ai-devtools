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
/**
 * Pending request statistics
 */
interface PendingRequestStats {
    readonly highlight: number;
    readonly execute: number;
    readonly a11y: number;
    readonly dom: number;
}
/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export declare function clearPendingRequests(): void;
/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns Counts of pending requests by type
 */
export declare function getPendingRequestStats(): PendingRequestStats;
export {};
//# sourceMappingURL=content.d.ts.map