/**
 * Purpose: Monitors context annotation sizes in log entries and warns when annotations are excessively large.
 * Docs: docs/features/feature/observe/index.md
 */
// =============================================================================
// CONSTANTS
// =============================================================================
/** Context annotation thresholds */
const CONTEXT_SIZE_THRESHOLD = 20 * 1024;
const CONTEXT_WARNING_WINDOW_MS = 60000;
const CONTEXT_WARNING_COUNT = 3;
// =============================================================================
// STATE
// =============================================================================
/** Context annotation monitoring state */
let contextExcessiveTimestamps = [];
let contextWarningState = null;
// =============================================================================
// CONTEXT ANNOTATION MONITORING
// =============================================================================
/**
 * Measure the serialized byte size of _context in a log entry
 */
export function measureContextSize(entry) {
    const context = entry._context;
    if (!context || typeof context !== 'object')
        return 0;
    const keys = Object.keys(context);
    if (keys.length === 0)
        return 0;
    return JSON.stringify(context).length;
}
/**
 * Check a batch of entries for excessive context annotation usage
 */
export function checkContextAnnotations(entries) {
    const now = Date.now();
    for (const entry of entries) {
        const size = measureContextSize(entry);
        if (size > CONTEXT_SIZE_THRESHOLD) {
            contextExcessiveTimestamps.push({ ts: now, size });
        }
    }
    contextExcessiveTimestamps = contextExcessiveTimestamps.filter((t) => now - t.ts < CONTEXT_WARNING_WINDOW_MS);
    if (contextExcessiveTimestamps.length >= CONTEXT_WARNING_COUNT) {
        const avgSize = contextExcessiveTimestamps.reduce((sum, t) => sum + t.size, 0) / contextExcessiveTimestamps.length;
        contextWarningState = {
            sizeKB: Math.round(avgSize / 1024),
            count: contextExcessiveTimestamps.length,
            triggeredAt: now
        };
    }
    else if (contextWarningState && contextExcessiveTimestamps.length === 0) {
        contextWarningState = null;
    }
}
/**
 * Get the current context annotation warning state
 */
export function getContextWarning() {
    return contextWarningState;
}
/**
 * Reset the context annotation warning (for testing)
 */
export function resetContextWarning() {
    contextExcessiveTimestamps = [];
    contextWarningState = null;
}
//# sourceMappingURL=context-monitor.js.map