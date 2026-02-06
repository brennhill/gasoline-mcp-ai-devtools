/**
 * @fileoverview inject/index.ts - Main orchestration and barrel exports
 * Combines API, observers, and message handlers for page-level capture.
 */
export { safeSerialize, getElementSelector, isSensitiveInput } from '../lib/serialize';
export { getContextAnnotations, setContextAnnotation, removeContextAnnotation, clearContextAnnotations, } from '../lib/context';
export { getImplicitRole, isDynamicClass, computeCssPath, computeSelectors, recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer, generatePlaywrightScript, } from '../lib/reproduction';
export { recordAction, getActionBuffer, clearActionBuffer, handleClick, handleInput, handleScroll, handleKeydown, handleChange, installActionCapture, uninstallActionCapture, setActionCaptureEnabled, installNavigationCapture, uninstallNavigationCapture, } from '../lib/actions';
export { parseResourceTiming, getNetworkWaterfall, trackPendingRequest, completePendingRequest, getPendingRequests, clearPendingRequests, getNetworkWaterfallForError, setNetworkWaterfallEnabled, isNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled, isNetworkBodyCaptureEnabled, shouldCaptureUrl, setServerUrl, sanitizeHeaders, truncateRequestBody, truncateResponseBody, readResponseBody, readResponseBodyWithTimeout, wrapFetchWithBodies, } from '../lib/network';
export { getPerformanceMarks, getPerformanceMeasures, getCapturedMarks, getCapturedMeasures, installPerformanceCapture, uninstallPerformanceCapture, isPerformanceCaptureActive, getPerformanceSnapshotForError, setPerformanceMarksEnabled, isPerformanceMarksEnabled, } from '../lib/performance';
export { postLog } from '../lib/bridge';
export { installConsoleCapture, uninstallConsoleCapture } from '../lib/console';
export { parseStackFrames, parseSourceMap, extractSnippet, extractSourceSnippets, detectFramework, getReactComponentAncestry, captureStateSnapshot, generateAiSummary, enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot, setSourceMapCache, getSourceMapCache, getSourceMapCacheSize, } from '../lib/ai-context';
export { installExceptionCapture, uninstallExceptionCapture } from '../lib/exceptions';
export { getSize, formatPayload, truncateWsMessage, createConnectionTracker, installWebSocketCapture, setWebSocketCaptureMode, setWebSocketCaptureEnabled, getWebSocketCaptureMode, uninstallWebSocketCapture, } from '../lib/websocket';
export { executeDOMQuery, getPageInfo, runAxeAudit, runAxeAuditWithTimeout, formatAxeResults } from '../lib/dom-queries';
export { mapInitiatorType, aggregateResourceTiming, capturePerformanceSnapshot, installPerfObservers, uninstallPerfObservers, getLongTaskMetrics, getFCP, getLCP, getCLS, getINP, sendPerformanceSnapshot, isPerformanceSnapshotEnabled, setPerformanceSnapshotEnabled, } from '../lib/perf-snapshot';
export { MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES, SENSITIVE_HEADERS } from '../lib/constants';
export { installGasolineAPI, uninstallGasolineAPI, type GasolineAPI } from './api';
export { install, uninstall, wrapFetch, installFetchCapture, uninstallFetchCapture, installPhase1, installPhase2, getDeferralState, setDeferralEnabled, shouldDeferIntercepts, checkMemoryPressure, type DeferralState, } from './observers';
export { installMessageListener, executeJavaScript, safeSerializeForExecute } from './message-handlers';
export { captureState, restoreState, highlightElement, clearHighlight, type RestoreStateResult, type RestoredCounts, type HighlightResult, } from './state';
//# sourceMappingURL=index.d.ts.map