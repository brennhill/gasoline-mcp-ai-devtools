/**
 * Purpose: Manages pending request/response pairs (highlight, execute_js, a11y, DOM queries) with timeout cleanup for AI Web Pilot features.
 * Docs: docs/features/feature/interact-explore/index.md
 */
/**
 * Generic tracker for pending request/response pairs keyed by auto-incrementing numeric ID.
 */
class PendingRequestTracker {
    pending = new Map();
    nextId = 0;
    /** Register a resolver and return the assigned request ID. */
    register(resolve) {
        const id = ++this.nextId;
        this.pending.set(id, resolve);
        return id;
    }
    /** Resolve a pending request by ID and remove it from the tracker. */
    resolve(id, result) {
        const resolver = this.pending.get(id);
        if (resolver) {
            this.pending.delete(id);
            resolver(result);
        }
    }
    /** Check whether a request ID is still pending. */
    has(id) {
        return this.pending.has(id);
    }
    /** Remove a pending request without resolving it. */
    delete(id) {
        this.pending.delete(id);
    }
    /** Remove all pending requests. */
    clear() {
        this.pending.clear();
    }
    /** Number of pending requests. */
    get size() {
        return this.pending.size;
    }
}
// Typed tracker instances for each request category
const highlightTracker = new PendingRequestTracker();
const executeTracker = new PendingRequestTracker();
const a11yTracker = new PendingRequestTracker();
const domTracker = new PendingRequestTracker();
// Periodic cleanup timer (Issue #2 fix)
const CLEANUP_INTERVAL_MS = 30000; // 30 seconds
let cleanupTimer = null;
// Track request timestamps for stale detection
const requestTimestamps = new Map();
/**
 * Get request timestamps for stale detection (Issue #2 fix).
 * Returns array of [requestId, timestamp] pairs for cleanup.
 */
function getRequestTimestamps() {
    const timestamps = [];
    for (const [id, timestamp] of requestTimestamps) {
        timestamps.push([id, timestamp]);
    }
    return timestamps;
}
/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export function clearPendingRequests() {
    highlightTracker.clear();
    executeTracker.clear();
    a11yTracker.clear();
    domTracker.clear();
    requestTimestamps.clear();
}
/**
 * Perform periodic cleanup of stale requests (Issue #2 fix).
 * Removes requests older than 60 seconds as a fallback when pagehide/beforeunload don't fire.
 */
function performPeriodicCleanup() {
    const now = Date.now();
    const staleThreshold = 60000; // 60 seconds
    for (const [id, timestamp] of getRequestTimestamps()) {
        if (now - timestamp > staleThreshold) {
            // Remove stale request from all trackers
            highlightTracker.delete(id);
            executeTracker.delete(id);
            a11yTracker.delete(id);
            domTracker.delete(id);
            requestTimestamps.delete(id);
        }
    }
}
/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns Counts of pending requests by type
 */
export function getPendingRequestStats() {
    return {
        highlight: highlightTracker.size,
        execute: executeTracker.size,
        a11y: a11yTracker.size,
        dom: domTracker.size
    };
}
// --- Highlight requests ---
export function registerHighlightRequest(resolve) {
    return highlightTracker.register(resolve);
}
export function resolveHighlightRequest(requestId, result) {
    highlightTracker.resolve(requestId, result);
}
export function hasHighlightRequest(requestId) {
    return highlightTracker.has(requestId);
}
export function deleteHighlightRequest(requestId) {
    highlightTracker.delete(requestId);
}
// --- Execute requests ---
export function registerExecuteRequest(resolve) {
    return executeTracker.register(resolve);
}
export function resolveExecuteRequest(requestId, result) {
    executeTracker.resolve(requestId, result);
}
export function hasExecuteRequest(requestId) {
    return executeTracker.has(requestId);
}
export function deleteExecuteRequest(requestId) {
    executeTracker.delete(requestId);
}
// --- A11y requests ---
export function registerA11yRequest(resolve) {
    return a11yTracker.register(resolve);
}
export function resolveA11yRequest(requestId, result) {
    a11yTracker.resolve(requestId, result);
}
export function hasA11yRequest(requestId) {
    return a11yTracker.has(requestId);
}
export function deleteA11yRequest(requestId) {
    a11yTracker.delete(requestId);
}
// --- DOM requests ---
export function registerDomRequest(resolve) {
    return domTracker.register(resolve);
}
export function resolveDomRequest(requestId, result) {
    domTracker.resolve(requestId, result);
}
export function hasDomRequest(requestId) {
    return domTracker.has(requestId);
}
export function deleteDomRequest(requestId) {
    domTracker.delete(requestId);
}
/**
 * Cleanup periodic timer (Issue #2 fix).
 * Should be called when content script is shutting down.
 */
export function cleanupRequestTracking() {
    if (cleanupTimer) {
        clearInterval(cleanupTimer);
        cleanupTimer = null;
    }
    clearPendingRequests();
}
/**
 * Initialize request tracking (register cleanup handlers)
 */
export function initRequestTracking() {
    // Register cleanup handlers for page unload/navigation (Issue 2 fix)
    // Using 'pagehide' (modern, fires on both close and navigation) + 'beforeunload' (legacy fallback)
    window.addEventListener('pagehide', clearPendingRequests);
    window.addEventListener('beforeunload', clearPendingRequests);
    // Start periodic cleanup timer (Issue #2 fix)
    // Provides fallback when pagehide/beforeunload don't fire (e.g., page crash)
    cleanupTimer = setInterval(performPeriodicCleanup, CLEANUP_INTERVAL_MS);
}
//# sourceMappingURL=request-tracking.js.map