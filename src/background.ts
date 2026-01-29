/**
 * @fileoverview Background Service Worker Facade
 * Re-exports all public APIs from background modules for backward compatibility.
 * Main modules:
 * - background/index.ts: Core initialization and batchers
 * - background/state-manager.ts: State management (error groups, cache, memory)
 * - background/communication.ts: Server communication
 * - background/init.ts: Extension startup
 */

import { initializeExtension } from './background/init';
import { EXTENSION_SESSION_ID } from './background/index';

// =============================================================================
// RE-EXPORT CONSTANTS
// =============================================================================

export {
  MEMORY_SOFT_LIMIT,
  MEMORY_HARD_LIMIT,
  MEMORY_CHECK_INTERVAL_MS,
  MEMORY_AVG_LOG_ENTRY_SIZE,
  MEMORY_AVG_WS_EVENT_SIZE,
  MEMORY_AVG_NETWORK_BODY_SIZE,
  MEMORY_AVG_ACTION_SIZE,
  ERROR_GROUP_MAX_AGE_MS,
  MAX_PENDING_BUFFER,
  SOURCE_MAP_CACHE_SIZE,
} from './background/state-manager';

export { RATE_LIMIT_CONFIG } from './background/communication';

// =============================================================================
// RE-EXPORT STATE AND CONFIG
// =============================================================================

export {
  EXTENSION_SESSION_ID,
  serverUrl,
  debugMode,
  connectionStatus,
  currentLogLevel,
  screenshotOnError,
  _captureOverrides,
  aiControlled,
  _connectionCheckRunning,
  extensionLogQueue,
  DebugCategory,
} from './background/index';

// =============================================================================
// RE-EXPORT DEBUG LOGGING
// =============================================================================

export {
  diagnosticLog,
  debugLog,
  getDebugLog,
  clearDebugLog,
  exportDebugLog,
  setDebugMode,
} from './background/index';

// =============================================================================
// RE-EXPORT CIRCUIT BREAKER AND BATCHERS
// =============================================================================

export {
  sharedServerCircuitBreaker,
  logBatcherWithCB,
  logBatcher,
  wsBatcherWithCB,
  wsBatcher,
  enhancedActionBatcherWithCB,
  enhancedActionBatcher,
  networkBodyBatcherWithCB,
  networkBodyBatcher,
  perfBatcherWithCB,
  perfBatcher,
} from './background/index';

// =============================================================================
// RE-EXPORT LOG AND REQUEST HANDLERS
// =============================================================================

export {
  handleLogMessage,
  handleClearLogs,
  isConnectionCheckRunning,
  checkConnectionAndUpdate,
  applyCaptureOverrides,
} from './background/index';

// =============================================================================
// RE-EXPORT POLLING WRAPPERS
// =============================================================================

export {
  pollPendingQueriesWrapper,
  postSettingsWrapper,
  postNetworkWaterfall,
  postExtensionLogsWrapper,
  sendStatusPingWrapper,
} from './background/index';

// =============================================================================
// RE-EXPORT PENDING QUERY HANDLERS
// =============================================================================

export {
  handlePendingQuery,
  handlePilotCommand,
} from './background/index';

// =============================================================================
// RE-EXPORT AI WEB PILOT STATE
// =============================================================================

export {
  __aiWebPilotEnabledCache,
  __aiWebPilotCacheInitialized,
  __pilotInitCallback,
  _resetPilotCacheForTesting,
  isAiWebPilotEnabled,
} from './background/index';

// =============================================================================
// RE-EXPORT STATE MANAGEMENT FUNCTIONS
// =============================================================================

export {
  createErrorSignature,
  processErrorGroup,
  getErrorGroupsState,
  cleanupStaleErrorGroups,
  flushErrorGroups,
  canTakeScreenshot,
  recordScreenshot,
  clearScreenshotTimestamps,
  estimateBufferMemory,
  checkMemoryPressure,
  getMemoryPressureState,
  resetMemoryPressureState,
  isNetworkBodyCaptureDisabled,
  measureContextSize,
  checkContextAnnotations,
  getContextWarning,
  resetContextWarning,
  setSourceMapEnabled,
  isSourceMapEnabled,
  setSourceMapCacheEntry,
  getSourceMapCacheEntry,
  getSourceMapCacheSize,
  clearSourceMapCache,
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
  cleanupStaleProcessingQueries,
} from './background/state-manager';

// =============================================================================
// RE-EXPORT COMMUNICATION FUNCTIONS
// =============================================================================

export {
  createCircuitBreaker,
  createBatcherWithCircuitBreaker,
  createLogBatcher,
  sendLogsToServer,
  sendWSEventsToServer,
  sendNetworkBodiesToServer,
  sendEnhancedActionsToServer,
  sendPerformanceSnapshotsToServer,
  sendNetworkWaterfallToServer,
  checkServerHealth,
  updateBadge,
  formatLogEntry,
  shouldCaptureLog,
  postQueryResult,
} from './background/communication';

// =============================================================================
// RE-EXPORT POLLING FUNCTIONS
// =============================================================================

export {
  startQueryPolling,
  stopQueryPolling,
  startSettingsHeartbeat,
  stopSettingsHeartbeat,
  startWaterfallPosting,
  stopWaterfallPosting,
  startExtensionLogsPosting,
  stopExtensionLogsPosting,
  startStatusPing,
  stopStatusPing,
} from './background/polling';

// =============================================================================
// RE-EXPORT STATE SNAPSHOT FUNCTIONS
// =============================================================================

export {
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot,
} from './background/message-handlers';

// =============================================================================
// INITIALIZATION
// =============================================================================

const _moduleLoadTime = performance.now();
console.log(`[DIAGNOSTIC] Module load start at ${_moduleLoadTime.toFixed(2)}ms (${new Date().toISOString()})`);

console.log(`[Gasoline] Background service worker loaded - session ${EXTENSION_SESSION_ID}`);

// Initialize the extension
initializeExtension();
