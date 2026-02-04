/**
 * @fileoverview State Manager Facade
 * Re-exports all state management functions from specialized submodules:
 * - error-groups.ts: Error deduplication and grouping
 * - cache-limits.ts: Memory management and source map caching
 * - snapshots.ts: Stack trace resolution and processing queries
 */
export { createErrorSignature, processErrorGroup, getErrorGroupsState, cleanupStaleErrorGroups, flushErrorGroups, type ProcessedLogEntry, ERROR_GROUP_MAX_AGE_MS, } from './error-groups';
export { canTakeScreenshot, recordScreenshot, clearScreenshotTimestamps, estimateBufferMemory, checkMemoryPressure, getMemoryPressureState, resetMemoryPressureState, isNetworkBodyCaptureDisabled, setSourceMapEnabled, isSourceMapEnabled, setSourceMapCacheEntry, getSourceMapCacheEntry, getSourceMapCacheSize, clearSourceMapCache, SOURCE_MAP_CACHE_SIZE, MEMORY_SOFT_LIMIT, MEMORY_HARD_LIMIT, MEMORY_CHECK_INTERVAL_MS, MEMORY_AVG_LOG_ENTRY_SIZE, MEMORY_AVG_WS_EVENT_SIZE, MEMORY_AVG_NETWORK_BODY_SIZE, MEMORY_AVG_ACTION_SIZE, MAX_PENDING_BUFFER, } from './cache-limits';
export { measureContextSize, checkContextAnnotations, getContextWarning, resetContextWarning, decodeVLQ, parseMappings, parseStackFrame, extractSourceMapUrl, parseSourceMapData, findOriginalLocation, fetchSourceMap, resolveStackFrame, resolveStackTrace, getProcessingQueriesState, addProcessingQuery, removeProcessingQuery, isQueryProcessing, cleanupStaleProcessingQueries, } from './snapshots';
import type { DebugLogEntry } from '../types';
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
//# sourceMappingURL=state-manager.d.ts.map