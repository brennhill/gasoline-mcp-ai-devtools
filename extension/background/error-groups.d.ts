/**
 * Purpose: Deduplicates and groups identical errors within configurable time windows to reduce server traffic.
 * Docs: docs/features/feature/error-clustering/index.md
 */
/**
 * @fileoverview Error Grouping and Deduplication
 * Manages error group tracking, deduplication within configurable windows,
 * and cleanup of stale error groups.
 */
import type { LogEntry } from '../types/index.js';
/** Process error group result */
interface ProcessErrorGroupResult {
    shouldSend: boolean;
    entry?: LogEntry & {
        _previousOccurrences?: number;
    };
}
/** Processed log entry with aggregation metadata */
export type ProcessedLogEntry = LogEntry & {
    _aggregatedCount?: number;
    _firstSeen?: string;
    _lastSeen?: string;
};
export declare function createErrorSignature(entry: LogEntry): string;
export declare function processErrorGroup(entry: LogEntry): ProcessErrorGroupResult;
/**
 * Clean up stale error groups older than ERROR_GROUP_MAX_AGE_MS
 */
export declare function cleanupStaleErrorGroups(debugLogFn?: (category: string, message: string, data?: unknown) => void): void;
/**
 * Flush error groups - send any accumulated counts
 */
export declare function flushErrorGroups(): ProcessedLogEntry[];
export {};
//# sourceMappingURL=error-groups.d.ts.map