/**
 * Purpose: Tracks in-flight query IDs with timestamps and cleans up stale entries.
 */

// processing-queries.ts — Processing query lifecycle tracking.

// =============================================================================
// CONSTANTS
// =============================================================================

/** Processing query TTL */
const PROCESSING_QUERY_TTL_MS = 60000

// =============================================================================
// STATE
// =============================================================================

/** Processing queries tracking */
const processingQueries = new Map<string, number>()

// =============================================================================
// CRUD
// =============================================================================

/**
 * Get current state of processing queries (for testing)
 */
export function getProcessingQueriesState(): Map<string, number> {
  return processingQueries
}

/**
 * Add a query to the processing set with timestamp
 */
export function addProcessingQuery(queryId: string, timestamp: number = Date.now()): void {
  processingQueries.set(queryId, timestamp)
}

/**
 * Remove a query from the processing set
 */
export function removeProcessingQuery(queryId: string): void {
  processingQueries.delete(queryId)
}

/**
 * Check if a query is currently being processed
 */
export function isQueryProcessing(queryId: string): boolean {
  return processingQueries.has(queryId)
}

/**
 * Clean up stale processing queries that have exceeded the TTL
 */
export function cleanupStaleProcessingQueries(
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): void {
  const now = Date.now()
  for (const [queryId, timestamp] of processingQueries) {
    if (now - timestamp > PROCESSING_QUERY_TTL_MS) {
      processingQueries.delete(queryId)
      if (debugLogFn) {
        debugLogFn('connection', 'Cleaned up stale processing query', {
          queryId,
          age: Math.round((now - timestamp) / 1000) + 's'
        })
      }
    }
  }
}
