/**
 * @fileoverview State Manager Facade
 * Re-exports all state management functions from specialized submodules:
 * - error-groups.ts: Error deduplication and grouping
 * - cache-limits.ts: Memory management and source map caching
 * - snapshots.ts: Stack trace resolution and processing queries
 */

// Re-export error grouping functions and types
export {
  createErrorSignature,
  processErrorGroup,
  getErrorGroupsState,
  cleanupStaleErrorGroups,
  flushErrorGroups,
  type ProcessedLogEntry,
  ERROR_GROUP_MAX_AGE_MS
} from './error-groups'

// Re-export cache and memory management
export {
  canTakeScreenshot,
  recordScreenshot,
  clearScreenshotTimestamps,
  estimateBufferMemory,
  checkMemoryPressure,
  getMemoryPressureState,
  resetMemoryPressureState,
  isNetworkBodyCaptureDisabled,
  setSourceMapEnabled,
  isSourceMapEnabled,
  setSourceMapCacheEntry,
  getSourceMapCacheEntry,
  getSourceMapCacheSize,
  clearSourceMapCache,
  SOURCE_MAP_CACHE_SIZE,
  MEMORY_SOFT_LIMIT,
  MEMORY_HARD_LIMIT,
  MEMORY_CHECK_INTERVAL_MS,
  MEMORY_AVG_LOG_ENTRY_SIZE,
  MEMORY_AVG_WS_EVENT_SIZE,
  MEMORY_AVG_NETWORK_BODY_SIZE,
  MEMORY_AVG_ACTION_SIZE,
  MAX_PENDING_BUFFER
} from './cache-limits'

// Re-export source map and context monitoring
export {
  measureContextSize,
  checkContextAnnotations,
  getContextWarning,
  resetContextWarning,
  decodeVLQ,
  parseMappings,
  parseStackFrame,
  extractSourceMapUrl,
  parseSourceMapData,
  findOriginalLocation,
  fetchSourceMap,
  resolveStackFrame,
  resolveStackTrace,
  getProcessingQueriesState,
  addProcessingQuery,
  removeProcessingQuery,
  isQueryProcessing,
  cleanupStaleProcessingQueries
} from './snapshots'

// Debug log functions are defined and exported below

// =============================================================================
// DEBUG LOG BUFFER
// =============================================================================

import type { DebugLogEntry } from '../types'

/** Debug log buffer */
const debugLogBuffer: DebugLogEntry[] = []

/** Debug log buffer size */
const DEBUG_LOG_MAX_ENTRIES = 200

/**
 * Get all debug log entries
 */
export function getDebugLog(): DebugLogEntry[] {
  return [...debugLogBuffer]
}

/**
 * Add entry to debug log buffer
 */
export function addDebugLogEntry(entry: DebugLogEntry): void {
  debugLogBuffer.push(entry)
  if (debugLogBuffer.length > DEBUG_LOG_MAX_ENTRIES) {
    debugLogBuffer.shift()
  }
}

/**
 * Clear debug log buffer
 */
export function clearDebugLog(): void {
  debugLogBuffer.length = 0
}
