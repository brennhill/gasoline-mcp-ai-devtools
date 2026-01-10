/**
 * @fileoverview inject.ts - Page-level capture script for browser telemetry.
 * Runs in the page context (not extension sandbox) to intercept console methods,
 * fetch/XHR requests, WebSocket connections, errors, and user actions. Posts
 * captured events to the content script via window.postMessage.
 * Design: Monkey-patches native APIs (console, fetch, WebSocket) with safe wrappers.
 * Defers network/WS interception until after page load to avoid impacting performance.
 * Buffers are size-capped (actions: 20, waterfall: 50, perf entries: 50).
 * Exposes window.__gasoline for version detection and programmatic control.
 */
import type { LogEntry, ActionEntry, EnhancedAction, SelectorStrategies, WaterfallEntry, PerformanceMark, PerformanceMeasure, ExecuteJsResult, BrowserStateSnapshot } from './types/index';
export { safeSerialize, getElementSelector, isSensitiveInput } from './lib/serialize';
export { getContextAnnotations, setContextAnnotation, removeContextAnnotation, clearContextAnnotations, } from './lib/context';
export { getImplicitRole, isDynamicClass, computeCssPath, computeSelectors, recordEnhancedAction, getEnhancedActionBuffer, clearEnhancedActionBuffer, generatePlaywrightScript, } from './lib/reproduction';
export { recordAction, getActionBuffer, clearActionBuffer, handleClick, handleInput, handleScroll, handleKeydown, handleChange, installActionCapture, uninstallActionCapture, setActionCaptureEnabled, installNavigationCapture, uninstallNavigationCapture, } from './lib/actions';
export { parseResourceTiming, getNetworkWaterfall, trackPendingRequest, completePendingRequest, getPendingRequests, clearPendingRequests, getNetworkWaterfallForError, setNetworkWaterfallEnabled, isNetworkWaterfallEnabled, setNetworkBodyCaptureEnabled, isNetworkBodyCaptureEnabled, shouldCaptureUrl, setServerUrl, sanitizeHeaders, truncateRequestBody, truncateResponseBody, readResponseBody, readResponseBodyWithTimeout, wrapFetchWithBodies, } from './lib/network';
export { getPerformanceMarks, getPerformanceMeasures, getCapturedMarks, getCapturedMeasures, installPerformanceCapture, uninstallPerformanceCapture, isPerformanceCaptureActive, getPerformanceSnapshotForError, setPerformanceMarksEnabled, isPerformanceMarksEnabled, } from './lib/performance';
export { postLog } from './lib/bridge';
export { installConsoleCapture, uninstallConsoleCapture } from './lib/console';
export { parseStackFrames, parseSourceMap, extractSnippet, extractSourceSnippets, detectFramework, getReactComponentAncestry, captureStateSnapshot, generateAiSummary, enrichErrorWithAiContext, setAiContextEnabled, setAiContextStateSnapshot, setSourceMapCache, getSourceMapCache, getSourceMapCacheSize, } from './lib/ai-context';
export { installExceptionCapture, uninstallExceptionCapture } from './lib/exceptions';
export { getSize, formatPayload, truncateWsMessage, createConnectionTracker, installWebSocketCapture, setWebSocketCaptureMode, setWebSocketCaptureEnabled, getWebSocketCaptureMode, uninstallWebSocketCapture, } from './lib/websocket';
export { executeDOMQuery, getPageInfo, runAxeAudit, runAxeAuditWithTimeout, formatAxeResults, } from './lib/dom-queries';
export { mapInitiatorType, aggregateResourceTiming, capturePerformanceSnapshot, installPerfObservers, uninstallPerfObservers, getLongTaskMetrics, getFCP, getLCP, getCLS, getINP, sendPerformanceSnapshot, isPerformanceSnapshotEnabled, setPerformanceSnapshotEnabled, } from './lib/perf-snapshot';
export { MAX_WATERFALL_ENTRIES, MAX_PERFORMANCE_ENTRIES, SENSITIVE_HEADERS } from './lib/constants';
/**
 * Deferral state for diagnostics
 */
interface DeferralState {
    deferralEnabled: boolean;
    phase2Installed: boolean;
    injectionTimestamp: number;
    phase2Timestamp: number;
}
/**
 * Memory pressure check state
 */
interface MemoryPressureState {
    memoryUsageMB: number;
    networkBodiesEnabled: boolean;
    wsBufferCapacity: number;
    networkBufferCapacity: number;
}
/**
 * Highlight result
 */
interface HighlightResult {
    success: boolean;
    selector?: string;
    bounds?: {
        x: number;
        y: number;
        width: number;
        height: number;
    };
    error?: string;
}
/**
 * Restored state counts
 */
interface RestoredCounts {
    localStorage: number;
    sessionStorage: number;
    cookies: number;
    skipped: number;
}
/**
 * Restore state result
 */
interface RestoreStateResult {
    success: boolean;
    restored?: RestoredCounts;
    error?: string;
}
/**
 * GasolineAPI interface exposed on window.__gasoline
 */
interface GasolineAPI {
    annotate(key: string, value: unknown): void;
    removeAnnotation(key: string): void;
    clearAnnotations(): void;
    getContext(): Record<string, unknown> | null;
    getActions(): ActionEntry[];
    clearActions(): void;
    setActionCapture(enabled: boolean): void;
    setNetworkWaterfall(enabled: boolean): void;
    getNetworkWaterfall(options?: {
        since?: number;
        initiatorTypes?: string[];
    }): WaterfallEntry[];
    setPerformanceMarks(enabled: boolean): void;
    getMarks(options?: {
        since?: number;
    }): PerformanceMark[];
    getMeasures(options?: {
        since?: number;
    }): PerformanceMeasure[];
    enrichError(error: LogEntry): Promise<LogEntry>;
    setAiContext(enabled: boolean): void;
    setStateSnapshot(enabled: boolean): void;
    recordAction(type: string, element: Element, opts?: Record<string, unknown>): void;
    getEnhancedActions(): EnhancedAction[];
    clearEnhancedActions(): void;
    generateScript(actions?: EnhancedAction[], opts?: Record<string, unknown>): string;
    getSelectors(element: Element): SelectorStrategies;
    version: string;
}
declare global {
    interface Window {
        __gasoline?: GasolineAPI;
    }
}
/**
 * Wrap fetch to capture network errors
 */
export declare function wrapFetch(originalFetchFn: typeof fetch): typeof fetch;
/**
 * Install fetch capture.
 * Uses wrapFetchWithBodies to capture request/response bodies for all requests,
 * then wraps that with wrapFetch to also capture error details for 4xx/5xx responses.
 * This ensures both body capture (GASOLINE_NETWORK_BODY) and error logging work together.
 */
export declare function installFetchCapture(): void;
/**
 * Uninstall fetch capture
 */
export declare function uninstallFetchCapture(): void;
/**
 * Install all capture hooks
 */
export declare function install(): void;
/**
 * Uninstall all capture hooks
 */
export declare function uninstall(): void;
/**
 * Install the window.__gasoline API for developers to interact with Gasoline
 */
export declare function installGasolineAPI(): void;
/**
 * Uninstall the window.__gasoline API
 */
export declare function uninstallGasolineAPI(): void;
/**
 * Safe serialization for complex objects returned from executeJavaScript.
 * Handles circular references, DOM nodes, functions, and large objects.
 * @param value - Value to serialize
 * @param depth - Current recursion depth
 * @param seen - Set of already-seen objects for circular detection
 * @returns Serialized value
 */
export declare function safeSerializeForExecute(value: unknown, depth?: number, seen?: WeakSet<object>): unknown;
/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 * Used by the AI Web Pilot execute_javascript tool.
 * @param script - JavaScript expression to evaluate
 * @param timeoutMs - Timeout in milliseconds (default 5000)
 * @returns Result with success/result or error details
 */
export declare function executeJavaScript(script: string, timeoutMs?: number): Promise<ExecuteJsResult>;
/**
 * Check if heavy intercepts should be deferred until page load
 * @returns True if page is still loading
 */
export declare function shouldDeferIntercepts(): boolean;
/**
 * Check memory pressure and adjust buffer capacities
 * @param state - Current buffer state
 * @returns Adjusted state
 */
export declare function checkMemoryPressure(state: MemoryPressureState): MemoryPressureState;
/**
 * Phase 1 (Immediate): Lightweight, non-intercepting setup.
 * - Registers window.__gasoline API
 * - Sets up message listener (already done above)
 * - Starts PerformanceObservers for paint timing and CLS
 * - Records injection timestamp
 * - Triggers Phase 2 based on deferral settings
 */
export declare function installPhase1(): void;
/**
 * Phase 2 (Deferred): Heavy interceptors.
 * Installs console wrapping, fetch wrapping, WebSocket replacement,
 * error handlers, action capture, and navigation capture.
 */
export declare function installPhase2(): void;
/**
 * Get the current deferral state for diagnostics and testing.
 */
export declare function getDeferralState(): DeferralState;
/**
 * Set whether interception deferral is enabled.
 * When false, Phase 2 runs immediately (matching pre-deferral behavior).
 */
export declare function setDeferralEnabled(enabled: boolean): void;
/**
 * Highlight a DOM element by injecting a red overlay div.
 * @param selector - CSS selector for the element to highlight
 * @param durationMs - How long to show the highlight (default 5000ms)
 * @returns Result with success, bounds, or error
 */
export declare function highlightElement(selector: string, durationMs?: number): HighlightResult | undefined;
/**
 * Clear any existing highlight
 */
export declare function clearHighlight(): void;
/**
 * Capture browser state (localStorage, sessionStorage, cookies).
 * Returns a snapshot that can be restored later.
 * @returns State snapshot with url, timestamp, localStorage, sessionStorage, cookies
 */
export declare function captureState(): BrowserStateSnapshot;
/**
 * Restore browser state from a snapshot.
 * Clears existing state before restoring.
 * @param state - State snapshot from captureState()
 * @param includeUrl - Whether to navigate to the saved URL (default true)
 * @returns Result with success and restored counts
 */
export declare function restoreState(state: BrowserStateSnapshot, includeUrl?: boolean): RestoreStateResult;
//# sourceMappingURL=inject.d.ts.map