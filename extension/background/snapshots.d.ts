/**
 * Purpose: Fetches and caches source maps, parses stack frames with VLQ decoding, and resolves stack traces for better error messages.
 * Docs: docs/features/feature/observe/index.md
 */
import type { LogEntry, ContextWarning } from '../types/index.js';
/**
 * Measure the serialized byte size of _context in a log entry
 */
export declare function measureContextSize(entry: LogEntry): number;
/**
 * Check a batch of entries for excessive context annotation usage
 */
export declare function checkContextAnnotations(entries: LogEntry[]): void;
/**
 * Get the current context annotation warning state
 */
export declare function getContextWarning(): ContextWarning | null;
/**
 * Reset the context annotation warning (for testing)
 */
export declare function resetContextWarning(): void;
/**
 * Resolve an entire stack trace
 */
export declare function resolveStackTrace(stack: string, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<string>;
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
//# sourceMappingURL=snapshots.d.ts.map