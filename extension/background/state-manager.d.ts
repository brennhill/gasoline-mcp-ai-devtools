/**
 * @fileoverview State Manager - Manages extension state including error groups,
 * screenshot rate limiting, memory pressure, context annotations, source maps,
 * and processing query tracking.
 */
import type { LogEntry, MemoryPressureLevel, MemoryPressureState, ContextWarning, DebugLogEntry, ParsedSourceMap, BufferState } from '../types';
/** Error group max age - cleanup after 1 hour */
export declare const ERROR_GROUP_MAX_AGE_MS = 3600000;
/** Source map cache size limit */
export declare const SOURCE_MAP_CACHE_SIZE = 50;
/** Memory limits */
export declare const MEMORY_SOFT_LIMIT: number;
export declare const MEMORY_HARD_LIMIT: number;
export declare const MEMORY_CHECK_INTERVAL_MS = 30000;
export declare const MEMORY_AVG_LOG_ENTRY_SIZE = 500;
export declare const MEMORY_AVG_WS_EVENT_SIZE = 300;
export declare const MEMORY_AVG_NETWORK_BODY_SIZE = 1000;
export declare const MEMORY_AVG_ACTION_SIZE = 400;
/** Maximum pending buffer size */
export declare const MAX_PENDING_BUFFER = 1000;
/** Internal error group structure */
interface InternalErrorGroup {
    entry: LogEntry;
    count: number;
    firstSeen: number;
    lastSeen: number;
}
/** Process error group result */
interface ProcessErrorGroupResult {
    shouldSend: boolean;
    entry?: LogEntry & {
        _previousOccurrences?: number;
    };
}
/** Processed log entry with aggregation metadata */
type ProcessedLogEntry = LogEntry & {
    _aggregatedCount?: number;
    _firstSeen?: string;
    _lastSeen?: string;
};
/** Rate limit result */
interface RateLimitResult {
    allowed: boolean;
    reason?: 'session_limit' | 'rate_limit';
    nextAllowedIn?: number | null;
}
/** Parsed stack frame */
interface ParsedStackFrame {
    functionName: string;
    fileName: string;
    lineNumber: number;
    columnNumber: number;
    raw: string;
    originalFileName?: string;
    originalLineNumber?: number;
    originalColumnNumber?: number;
    originalFunctionName?: string;
    resolved?: boolean;
}
/** Original location from source map */
interface OriginalLocation {
    source: string;
    line: number;
    column: number;
    name: string | null;
}
/**
 * Create a signature for an error to identify duplicates
 */
export declare function createErrorSignature(entry: LogEntry): string;
/**
 * Process an error through the grouping system
 */
export declare function processErrorGroup(entry: LogEntry): ProcessErrorGroupResult;
/**
 * Get current state of error groups (for testing)
 */
export declare function getErrorGroupsState(): Map<string, InternalErrorGroup>;
/**
 * Clean up stale error groups older than ERROR_GROUP_MAX_AGE_MS
 */
export declare function cleanupStaleErrorGroups(debugLogFn?: (category: string, message: string, data?: unknown) => void): void;
/**
 * Flush error groups - send any accumulated counts
 */
export declare function flushErrorGroups(): ProcessedLogEntry[];
/**
 * Check if a screenshot is allowed based on rate limiting
 */
export declare function canTakeScreenshot(tabId: number): RateLimitResult;
/**
 * Record a screenshot timestamp
 */
export declare function recordScreenshot(tabId: number): void;
/**
 * Clear screenshot timestamps for a tab
 */
export declare function clearScreenshotTimestamps(tabId: number): void;
/**
 * Estimate total buffer memory usage from buffer contents
 */
export declare function estimateBufferMemory(buffers: BufferState): number;
/**
 * Check memory pressure and take appropriate action
 */
export declare function checkMemoryPressure(buffers: BufferState): {
    level: MemoryPressureLevel;
    action: string;
    estimatedMemory: number;
    alreadyApplied: boolean;
};
/**
 * Get the current memory pressure state
 */
export declare function getMemoryPressureState(): MemoryPressureState;
/**
 * Reset memory pressure state to initial values (for testing)
 */
export declare function resetMemoryPressureState(): void;
/**
 * Check if network body capture is disabled
 */
export declare function isNetworkBodyCaptureDisabled(): boolean;
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
 * Set source map enabled state
 */
export declare function setSourceMapEnabled(enabled: boolean): void;
/**
 * Check if source maps are enabled
 */
export declare function isSourceMapEnabled(): boolean;
/**
 * Set an entry in the source map cache with LRU eviction
 */
export declare function setSourceMapCacheEntry(url: string, map: ParsedSourceMap | null): void;
/**
 * Get an entry from the source map cache
 */
export declare function getSourceMapCacheEntry(url: string): ParsedSourceMap | null;
/**
 * Get the current size of the source map cache
 */
export declare function getSourceMapCacheSize(): number;
/**
 * Clear the source map cache
 */
export declare function clearSourceMapCache(): void;
/**
 * Decode a VLQ-encoded string into an array of integers
 */
export declare function decodeVLQ(str: string): number[];
/**
 * Parse a source map's mappings string into a structured format
 */
export declare function parseMappings(mappingsStr: string): number[][][];
/**
 * Parse a stack trace line into components
 */
export declare function parseStackFrame(line: string): ParsedStackFrame | null;
/**
 * Extract sourceMappingURL from script content
 */
export declare function extractSourceMapUrl(content: string): string | null;
/**
 * Parse source map data into a usable format
 */
export declare function parseSourceMapData(sourceMap: {
    mappings?: string;
    sources?: string[];
    names?: string[];
    sourceRoot?: string;
    sourcesContent?: string[];
}): ParsedSourceMap;
/**
 * Find original location from source map
 */
export declare function findOriginalLocation(sourceMap: ParsedSourceMap, line: number, column: number): OriginalLocation | null;
/**
 * Fetch a source map for a script URL
 */
export declare function fetchSourceMap(scriptUrl: string, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<ParsedSourceMap | null>;
/**
 * Resolve a single stack frame to original location
 */
export declare function resolveStackFrame(frame: ParsedStackFrame, debugLogFn?: (category: string, message: string, data?: unknown) => void): Promise<ParsedStackFrame>;
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
/**
 * Get all debug log entries
 */
export declare function getDebugLog(): DebugLogEntry[];
/**
 * Add entry to debug log buffer
 */
export declare function addDebugLogEntry(entry: DebugLogEntry): void;
/**
 * Clear debug log buffer
 */
export declare function clearDebugLog(): void;
export {};
//# sourceMappingURL=state-manager.d.ts.map