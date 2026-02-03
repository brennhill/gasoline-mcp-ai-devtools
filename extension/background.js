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
import { initializeExtension } from './background/init.js'
import { EXTENSION_SESSION_ID } from './background/index.js'
// =============================================================================
// === PUBLIC API: CONSTANTS (Test & Init)
// =============================================================================
// Memory enforcement constants
export {
  MEMORY_SOFT_LIMIT,
  MEMORY_HARD_LIMIT,
  MEMORY_CHECK_INTERVAL_MS,
  MEMORY_AVG_LOG_ENTRY_SIZE,
  MEMORY_AVG_WS_EVENT_SIZE,
  MEMORY_AVG_NETWORK_BODY_SIZE,
  MEMORY_AVG_ACTION_SIZE,
} from './background/state-manager.js'
// Rate limiting constants
export { RATE_LIMIT_CONFIG } from './background/communication.js'
// =============================================================================
// === PUBLIC API: CORE STATE
// =============================================================================
export {
  EXTENSION_SESSION_ID,
  serverUrl,
  debugMode,
  connectionStatus,
  currentLogLevel,
  screenshotOnError,
  extensionLogQueue,
  DebugCategory,
} from './background/index.js'
// =============================================================================
// === PUBLIC API: DEBUG LOGGING
// =============================================================================
export { debugLog, getDebugLog, clearDebugLog, exportDebugLog } from './background/index.js'
// =============================================================================
// === PUBLIC API: BATCHERS & CIRCUIT BREAKER
// =============================================================================
export {
  sharedServerCircuitBreaker,
  logBatcher,
  wsBatcher,
  enhancedActionBatcher,
  networkBodyBatcher,
  perfBatcher,
} from './background/index.js'
// =============================================================================
// === PUBLIC API: CORE HANDLERS
// =============================================================================
export {
  handleLogMessage,
  handleClearLogs,
  isConnectionCheckRunning,
  checkConnectionAndUpdate,
  applyCaptureOverrides,
} from './background/index.js'
// =============================================================================
// === PUBLIC API: POLLING WRAPPERS
// =============================================================================
export {
  pollPendingQueriesWrapper,
  postSettingsWrapper,
  postNetworkWaterfall,
  postExtensionLogsWrapper,
  sendStatusPingWrapper,
} from './background/index.js'
// =============================================================================
// === PUBLIC API: VERSION CHECKING
// =============================================================================
export {
  getExtensionVersion,
  isNewVersionAvailable,
  getAvailableVersion,
  updateVersionFromHealth,
  updateVersionBadge,
  getUpdateInfo,
  resetVersionCheck,
} from './background/version-check.js'
// =============================================================================
// === PUBLIC API: PENDING QUERIES & PILOT
// =============================================================================
export { handlePendingQuery, handlePilotCommand, isAiWebPilotEnabled } from './background/index.js'
// =============================================================================
// === PUBLIC API: STATE MANAGEMENT (Tests, Initialization)
// =============================================================================
// Error and memory management
export {
  createErrorSignature,
  processErrorGroup,
  flushErrorGroups,
  canTakeScreenshot,
  recordScreenshot,
  estimateBufferMemory,
  checkMemoryPressure,
  getMemoryPressureState,
  resetMemoryPressureState,
} from './background/state-manager.js'
// Context and annotations
export {
  measureContextSize,
  checkContextAnnotations,
  getContextWarning,
  resetContextWarning,
} from './background/state-manager.js'
// Source map management
export { setSourceMapEnabled, isSourceMapEnabled, clearSourceMapCache } from './background/state-manager.js'
// =============================================================================
// === PUBLIC API: COMMUNICATION (Tests)
// =============================================================================
export {
  createCircuitBreaker,
  createBatcherWithCircuitBreaker,
  createLogBatcher,
  sendLogsToServer,
  sendEnhancedActionsToServer,
  checkServerHealth,
  updateBadge,
  formatLogEntry,
  shouldCaptureLog,
} from './background/communication.js'
export { postQueryResult } from './background/server.js'
// =============================================================================
// === PUBLIC API: STATE SNAPSHOTS (Initialization)
// =============================================================================
export {
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot,
} from './background/message-handlers.js'
// =============================================================================
// === INTERNAL USE (Underscore Prefix)
// =============================================================================
export {
  _captureOverrides,
  _connectionCheckRunning,
  __aiWebPilotEnabledCache,
  __aiWebPilotCacheInitialized,
  __pilotInitCallback,
  _resetPilotCacheForTesting,
} from './background/index.js'
// =============================================================================
// INITIALIZATION
// =============================================================================
const _moduleLoadTime = performance.now()
console.log(`[DIAGNOSTIC] Module load start at ${_moduleLoadTime.toFixed(2)}ms (${new Date().toISOString()})`)
console.log(`[Gasoline] Background service worker loaded - session ${EXTENSION_SESSION_ID}`)
// Initialize the extension
initializeExtension()
//# sourceMappingURL=background.js.map
