/**
 * Purpose: Tracks which queries are currently being processed, with TTL-based cleanup for stale entries.
 * Docs: docs/features/feature/interact-explore/index.md
 */
// =============================================================================
// CONSTANTS
// =============================================================================
/** Processing query TTL */
const PROCESSING_QUERY_TTL_MS = 60000;
// =============================================================================
// STATE
// =============================================================================
/** Processing queries tracking */
const processingQueries = new Map();
// =============================================================================
// PROCESSING QUERY TRACKING
// =============================================================================
/**
 * Get current state of processing queries (for testing)
 */
export function getProcessingQueriesState() {
    return processingQueries;
}
/**
 * Add a query to the processing set with timestamp
 */
export function addProcessingQuery(queryId, timestamp = Date.now()) {
    processingQueries.set(queryId, timestamp);
}
/**
 * Remove a query from the processing set
 */
export function removeProcessingQuery(queryId) {
    processingQueries.delete(queryId);
}
/**
 * Check if a query is currently being processed
 */
export function isQueryProcessing(queryId) {
    return processingQueries.has(queryId);
}
/**
 * Clean up stale processing queries that have exceeded the TTL
 */
export function cleanupStaleProcessingQueries(debugLogFn) {
    const now = Date.now();
    for (const [queryId, timestamp] of processingQueries) {
        if (now - timestamp > PROCESSING_QUERY_TTL_MS) {
            processingQueries.delete(queryId);
            if (debugLogFn) {
                debugLogFn('connection', 'Cleaned up stale processing query', {
                    queryId,
                    age: Math.round((now - timestamp) / 1000) + 's'
                });
            }
        }
    }
}
//# sourceMappingURL=processing-queries.js.map