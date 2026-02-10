/**
 * @fileoverview inject/index.ts - Main orchestration and barrel exports
 * Combines API, observers, and message handlers for page-level capture.
 */

// Re-export barrel pattern for tests and consumers
export { safeSerialize, getElementSelector, isSensitiveInput } from '../lib/serialize'
export {
  getContextAnnotations,
  setContextAnnotation,
  removeContextAnnotation,
  clearContextAnnotations,
} from '../lib/context'
export {
  getImplicitRole,
  isDynamicClass,
  computeCssPath,
  computeSelectors,
  recordEnhancedAction,
  getEnhancedActionBuffer,
  clearEnhancedActionBuffer,
  generatePlaywrightScript,
} from '../lib/reproduction'
export {
  recordAction,
  getActionBuffer,
  clearActionBuffer,
  handleClick,
  handleInput,
  handleScroll,
  handleKeydown,
  handleChange,
  installActionCapture,
  uninstallActionCapture,
  setActionCaptureEnabled,
  installNavigationCapture,
  uninstallNavigationCapture,
} from '../lib/actions'
export {
  parseResourceTiming,
  getNetworkWaterfall,
  trackPendingRequest,
  completePendingRequest,
  getPendingRequests,
  clearPendingRequests,
  getNetworkWaterfallForError,
  setNetworkWaterfallEnabled,
  isNetworkWaterfallEnabled,
  setNetworkBodyCaptureEnabled,
  isNetworkBodyCaptureEnabled,
  shouldCaptureUrl,
  setServerUrl,
  sanitizeHeaders,
  truncateRequestBody,
  truncateResponseBody,
  readResponseBody,
  readResponseBodyWithTimeout,
  wrapFetchWithBodies,
} from '../lib/network'
export {
  getPerformanceMarks,
  getPerformanceMeasures,
  getCapturedMarks,
  getCapturedMeasures,
  installPerformanceCapture,
  uninstallPerformanceCapture,
  isPerformanceCaptureActive,
  getPerformanceSnapshotForError,
  setPerformanceMarksEnabled,
  isPerformanceMarksEnabled,
} from '../lib/performance'
export { postLog } from '../lib/bridge'
export { installConsoleCapture, uninstallConsoleCapture } from '../lib/console'
export {
  parseStackFrames,
  parseSourceMap,
  extractSnippet,
  extractSourceSnippets,
  detectFramework,
  getReactComponentAncestry,
  captureStateSnapshot,
  generateAiSummary,
  enrichErrorWithAiContext,
  setAiContextEnabled,
  setAiContextStateSnapshot,
  setSourceMapCache,
  getSourceMapCache,
  getSourceMapCacheSize,
} from '../lib/ai-context'
export { installExceptionCapture, uninstallExceptionCapture } from '../lib/exceptions'
export {
  getSize,
  formatPayload,
  truncateWsMessage,
  createConnectionTracker,
  installWebSocketCapture,
  setWebSocketCaptureMode,
  setWebSocketCaptureEnabled,
  getWebSocketCaptureMode,
  uninstallWebSocketCapture,
  resetForTesting,
} from '../lib/websocket'
export { executeDOMQuery, getPageInfo, runAxeAudit, runAxeAuditWithTimeout, formatAxeResults } from '../lib/dom-queries'
export {
  mapInitiatorType,
  aggregateResourceTiming,
  capturePerformanceSnapshot,
  installPerfObservers,
  uninstallPerfObservers,
  getLongTaskMetrics,
  getFCP,
  getLCP,
  getCLS,
  getINP,
  sendPerformanceSnapshot,
  isPerformanceSnapshotEnabled,
  setPerformanceSnapshotEnabled,
} from '../lib/perf-snapshot'

// Re-export constants that tests import from inject.js
export { MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES, SENSITIVE_HEADERS } from '../lib/constants'

// Export API module
export { installGasolineAPI, uninstallGasolineAPI, type GasolineAPI } from './api'

// Export observer module
export {
  install,
  uninstall,
  wrapFetch,
  installFetchCapture,
  uninstallFetchCapture,
  installPhase1,
  installPhase2,
  getDeferralState,
  setDeferralEnabled,
  shouldDeferIntercepts,
  checkMemoryPressure,
  type DeferralState,
} from './observers'

// Export message handlers module
export { installMessageListener, executeJavaScript, safeSerializeForExecute } from './message-handlers'

// Export state management functions
export {
  captureState,
  restoreState,
  highlightElement,
  clearHighlight,
  type RestoreStateResult,
  type RestoredCounts,
  type HighlightResult,
} from './state'

import { installGasolineAPI } from './api'
import { installPhase1 } from './observers'
import { installMessageListener } from './message-handlers'
import { captureState, restoreState, sendPerformanceSnapshotWrapper } from './state'
import { sendPerformanceSnapshot } from '../lib/perf-snapshot'

/**
 * Auto-install when loaded in browser
 */
if (typeof window !== 'undefined' && typeof document !== 'undefined' && typeof (globalThis as Record<string, unknown>).process === 'undefined') {
  // Install Phase 1 (lightweight API + observers)
  installPhase1()

  // Install message listener with state functions
  installMessageListener(captureState, restoreState)

  // Install Gasoline API
  installGasolineAPI()

  // Send performance snapshot after page load + 2s settling time
  window.addEventListener('load', () => {
    setTimeout(() => {
      sendPerformanceSnapshot()
    }, 2000)
  })
}
