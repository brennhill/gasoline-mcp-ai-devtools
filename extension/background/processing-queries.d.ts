/**
 * Purpose: Tracks in-flight query IDs with timestamps and cleans up stale entries.
 */
/**
 * Get current state of processing queries (for testing)
 */
export declare function getProcessingQueriesState(): Map<string, number>;
/**
 * Add a query to the processing set with timestamp
 */
export declare function addProcessingQuery(queryId: string, timestamp?: number): void;
/**
 * Remove a query from the processing set
 */
export declare function removeProcessingQuery(queryId: string): void;
/**
 * Check if a query is currently being processed
 */
export declare function isQueryProcessing(queryId: string): boolean;
/**
 * Clean up stale processing queries that have exceeded the TTL
 */
export declare function cleanupStaleProcessingQueries(debugLogFn?: (category: string, message: string, data?: unknown) => void): void;
//# sourceMappingURL=processing-queries.d.ts.map