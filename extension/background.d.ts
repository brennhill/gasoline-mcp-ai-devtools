/**
 * @fileoverview Background Service Worker Facade - Minimal Public API
 *
 * This facade provides a clean, minimal public API for the extension.
 * Direct use of internal modules (communication/, state-manager/, polling/)
 * should go through initialization in init.ts, not through the facade.
 *
 * Main modules:
 * - background/index.ts: Core state and batchers
 * - background/init.ts: Extension startup
 * - background/communication.ts: Server communication (internal)
 * - background/state-manager.ts: State management (internal)
 * - background/polling.ts: Polling loops (internal)
 */
export { MEMORY_SOFT_LIMIT, MEMORY_HARD_LIMIT, MEMORY_CHECK_INTERVAL_MS, MEMORY_AVG_LOG_ENTRY_SIZE, MEMORY_AVG_WS_EVENT_SIZE, MEMORY_AVG_NETWORK_BODY_SIZE, MEMORY_AVG_ACTION_SIZE } from './background/state-manager';
export { RATE_LIMIT_CONFIG } from './background/communication';
export { EXTENSION_SESSION_ID, serverUrl, debugMode, connectionStatus, currentLogLevel, screenshotOnError, extensionLogQueue, DebugCategory } from './background/index';
export { debugLog, getDebugLog, clearDebugLog, exportDebugLog } from './background/index';
export { sharedServerCircuitBreaker, logBatcher, wsBatcher, enhancedActionBatcher, networkBodyBatcher, perfBatcher } from './background/index';
export { handleLogMessage, handleClearLogs, isConnectionCheckRunning, checkConnectionAndUpdate, applyCaptureOverrides } from './background/index';
export { sendStatusPingWrapper } from './background/index';
export { getExtensionVersion, isNewVersionAvailable, getAvailableVersion, updateVersionFromHealth, updateVersionBadge, getUpdateInfo, resetVersionCheck } from './background/version-check';
export { handlePendingQuery, handlePilotCommand, isAiWebPilotEnabled, markInitComplete } from './background/index';
export { createErrorSignature, processErrorGroup, flushErrorGroups, cleanupStaleErrorGroups, canTakeScreenshot, recordScreenshot, estimateBufferMemory, checkMemoryPressure, getMemoryPressureState, resetMemoryPressureState, getProcessingQueriesState, cleanupStaleProcessingQueries } from './background/state-manager';
export { measureContextSize, checkContextAnnotations, getContextWarning, resetContextWarning } from './background/state-manager';
export { setSourceMapEnabled, isSourceMapEnabled, clearSourceMapCache } from './background/state-manager';
export { SOURCE_MAP_CACHE_SIZE, setSourceMapCacheEntry, getSourceMapCacheEntry, getSourceMapCacheSize } from './background/cache-limits';
export { createCircuitBreaker, createBatcherWithCircuitBreaker, createLogBatcher, sendLogsToServer, sendEnhancedActionsToServer, checkServerHealth, updateBadge, formatLogEntry, shouldCaptureLog } from './background/communication';
export { postQueryResult, pollPendingQueries } from './background/server';
export { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from './background/message-handlers';
export { _captureOverrides, _connectionCheckRunning, __aiWebPilotEnabledCache, __aiWebPilotCacheInitialized, __pilotInitCallback, _resetPilotCacheForTesting } from './background/index';
//# sourceMappingURL=background.d.ts.map