/**
 * @fileoverview Message Types for Gasoline Extension
 *
 * Comprehensive discriminated unions for all message types used in the extension.
 * This is the single source of truth for message payloads between:
 * - Background service worker
 * - Content scripts
 * - Inject scripts (page context)
 * - Popup
 */

// =============================================================================
// TELEMETRY DATA TYPES
// =============================================================================

/**
 * Log levels supported by the extension
 */
export type LogLevel = 'debug' | 'log' | 'info' | 'warn' | 'error';

/**
 * Log level filter including 'all' option
 */
export type LogLevelFilter = LogLevel | 'all';

/**
 * Log entry types
 */
export type LogType = 'console' | 'network' | 'exception' | 'screenshot';

/**
 * Base log entry with common fields
 */
export interface BaseLogEntry {
  readonly ts: string;
  readonly level: LogLevel;
  readonly type?: LogType;
  readonly tabId?: number;
  readonly _enrichments?: readonly string[];
  readonly _context?: Readonly<Record<string, unknown>>;
}

/**
 * Console log entry
 */
export interface ConsoleLogEntry extends BaseLogEntry {
  readonly type: 'console';
  readonly args?: readonly unknown[];
  readonly message?: string;
}

/**
 * Network error log entry
 */
export interface NetworkLogEntry extends BaseLogEntry {
  readonly type: 'network';
  readonly level: 'error';
  readonly method: string;
  readonly url: string;
  readonly status?: number;
  readonly statusText?: string;
  readonly duration?: number;
  readonly response?: string;
  readonly error?: string;
  readonly headers?: Readonly<Record<string, string>>;
}

/**
 * Exception log entry
 */
export interface ExceptionLogEntry extends BaseLogEntry {
  readonly type: 'exception';
  readonly level: 'error';
  readonly message: string;
  readonly stack?: string;
  readonly filename?: string;
  readonly lineno?: number;
  readonly colno?: number;
  readonly _sourceMapResolved?: boolean;
  readonly _errorId?: string;
  readonly _aiContext?: AiContextData;
}

/**
 * Screenshot log entry
 */
export interface ScreenshotLogEntry extends BaseLogEntry {
  readonly type: 'screenshot';
  readonly url?: string;
  readonly screenshotFile?: string;
  readonly trigger: 'error' | 'manual';
  readonly relatedErrorId?: string;
  readonly _screenshotFailed?: boolean;
  readonly error?: string;
}

/**
 * Union of all log entry types
 */
export type LogEntry =
  | ConsoleLogEntry
  | NetworkLogEntry
  | ExceptionLogEntry
  | ScreenshotLogEntry;

/**
 * Processed log entry with optional aggregation metadata
 */
export interface ProcessedLogEntry extends BaseLogEntry {
  readonly _aggregatedCount?: number;
  readonly _firstSeen?: string;
  readonly _lastSeen?: string;
  readonly _previousOccurrences?: number;
}

// =============================================================================
// WEBSOCKET TYPES
// =============================================================================

/**
 * WebSocket capture modes
 */
export type WebSocketCaptureMode = 'lifecycle' | 'messages' | 'full';

/**
 * WebSocket event types
 */
export type WebSocketEventType = 'open' | 'close' | 'error' | 'message';

/**
 * WebSocket event payload
 */
export interface WebSocketEvent {
  readonly type: WebSocketEventType;
  readonly url: string;
  readonly ts: string;
  readonly connectionId?: string;
  readonly data?: string;
  readonly size?: number;
  readonly direction?: 'sent' | 'received';
  readonly code?: number;
  readonly reason?: string;
}

// =============================================================================
// NETWORK TYPES
// =============================================================================

/**
 * Network waterfall entry phases
 */
export interface WaterfallPhases {
  readonly dns: number;
  readonly connect: number;
  readonly tls: number;
  readonly ttfb: number;
  readonly download: number;
}

/**
 * Parsed network waterfall entry
 */
export interface WaterfallEntry {
  readonly url: string;
  readonly initiatorType: string;
  readonly startTime: number;
  readonly duration: number;
  readonly phases: WaterfallPhases;
  readonly transferSize: number;
  readonly encodedBodySize: number;
  readonly decodedBodySize: number;
  readonly cached?: boolean;
}

/**
 * Pending network request tracking
 */
export interface PendingRequest {
  readonly id: string;
  readonly url: string;
  readonly method: string;
  readonly startTime: number;
}

/**
 * Network body capture payload
 */
export interface NetworkBodyPayload {
  readonly url: string;
  readonly method: string;
  readonly status: number;
  readonly contentType: string;
  readonly requestBody?: string;
  readonly responseBody?: string;
  readonly duration: number;
}

// =============================================================================
// PERFORMANCE TYPES
// =============================================================================

/**
 * Performance mark entry
 */
export interface PerformanceMark {
  readonly name: string;
  readonly startTime: number;
  readonly entryType: 'mark';
}

/**
 * Performance measure entry
 */
export interface PerformanceMeasure {
  readonly name: string;
  readonly startTime: number;
  readonly duration: number;
  readonly entryType: 'measure';
}

/**
 * Long task metrics
 */
export interface LongTaskMetrics {
  readonly count: number;
  readonly totalDuration: number;
  readonly maxDuration: number;
  readonly tasks: ReadonlyArray<{
    readonly duration: number;
    readonly startTime: number;
  }>;
}

/**
 * Web Vitals metrics
 */
export interface WebVitals {
  readonly fcp?: number;
  readonly lcp?: number;
  readonly cls?: number;
  readonly inp?: number;
}

/**
 * Performance snapshot payload
 */
export interface PerformanceSnapshot {
  readonly ts: string;
  readonly url: string;
  readonly vitals: WebVitals;
  readonly longTasks: LongTaskMetrics;
  readonly resources: {
    readonly count: number;
    readonly totalSize: number;
    readonly byType: Readonly<Record<string, { count: number; size: number }>>;
    readonly slowest: ReadonlyArray<{
      readonly url: string;
      readonly duration: number;
      readonly size: number;
    }>;
  };
  readonly memory?: {
    readonly usedJSHeapSize: number;
    readonly totalJSHeapSize: number;
  };
}

// =============================================================================
// USER ACTION TYPES
// =============================================================================

/**
 * Action types for user action replay
 */
export type ActionType = 'click' | 'input' | 'scroll' | 'keydown' | 'change' | 'navigate';

/**
 * Basic action entry
 */
export interface ActionEntry {
  readonly type: ActionType;
  readonly target: string;
  readonly timestamp: string;
  readonly value?: string;
}

/**
 * Multi-strategy selector result
 */
export interface SelectorStrategies {
  readonly testId?: string;
  readonly aria?: string;
  readonly role?: string;
  readonly cssPath?: string;
  readonly xpath?: string;
  readonly text?: string;
}

/**
 * Enhanced action entry with multiple selector strategies
 */
export interface EnhancedAction {
  readonly type: ActionType;
  readonly ts: string;
  readonly url: string;
  readonly selectors: SelectorStrategies;
  readonly value?: string;
  readonly key?: string;
  readonly modifiers?: {
    readonly ctrl?: boolean;
    readonly alt?: boolean;
    readonly shift?: boolean;
    readonly meta?: boolean;
  };
  readonly scrollPosition?: {
    readonly x: number;
    readonly y: number;
  };
}

// =============================================================================
// AI CONTEXT TYPES
// =============================================================================

/**
 * Parsed stack frame
 */
export interface StackFrame {
  readonly functionName: string;
  readonly fileName: string;
  readonly lineNumber: number;
  readonly columnNumber: number;
  readonly raw: string;
  readonly originalFileName?: string;
  readonly originalLineNumber?: number;
  readonly originalColumnNumber?: number;
  readonly originalFunctionName?: string;
  readonly resolved?: boolean;
}

/**
 * Source code snippet
 */
export interface SourceSnippet {
  readonly file: string;
  readonly line: number;
  readonly lines: readonly string[];
  readonly highlightLine: number;
}

/**
 * React component ancestry info
 */
export interface ReactComponentAncestry {
  readonly component: string;
  readonly props?: Readonly<Record<string, unknown>>;
  readonly ancestors: readonly string[];
}

/**
 * AI context data attached to errors
 */
export interface AiContextData {
  readonly framework?: string;
  readonly snippets?: readonly SourceSnippet[];
  readonly componentAncestry?: ReactComponentAncestry;
  readonly stateSnapshot?: Readonly<Record<string, unknown>>;
  readonly summary?: string;
}

// =============================================================================
// ACCESSIBILITY TYPES
// =============================================================================

/**
 * Accessibility violation node
 */
export interface A11yViolationNode {
  readonly html: string;
  readonly target: readonly string[];
  readonly failureSummary: string;
}

/**
 * Accessibility violation
 */
export interface A11yViolation {
  readonly id: string;
  readonly impact: string;
  readonly description: string;
  readonly help: string;
  readonly helpUrl: string;
  readonly nodes: readonly A11yViolationNode[];
}

/**
 * Accessibility audit result
 */
export interface A11yAuditResult {
  readonly violations: readonly A11yViolation[];
  readonly passes: readonly {
    readonly id: string;
    readonly description: string;
    readonly nodes: readonly { html: string; target: string[] }[];
  }[];
  readonly incomplete: readonly {
    readonly id: string;
    readonly description: string;
    readonly nodes: readonly { html: string; target: string[] }[];
  }[];
  readonly inapplicable: readonly { id: string; description: string }[];
  readonly summary?: {
    readonly violationCount: number;
    readonly passCount: number;
  };
  readonly error?: string;
}

// =============================================================================
// DOM QUERY TYPES
// =============================================================================

/**
 * DOM element info from query
 */
export interface DomElementInfo {
  readonly tag: string;
  readonly id?: string;
  readonly classes?: readonly string[];
  readonly text?: string;
  readonly html?: string;
  readonly attributes?: Readonly<Record<string, string>>;
  readonly boundingRect?: {
    readonly x: number;
    readonly y: number;
    readonly width: number;
    readonly height: number;
  };
}

/**
 * DOM query result
 */
export interface DomQueryResult {
  readonly elements: readonly DomElementInfo[];
  readonly count: number;
  readonly truncated: boolean;
  readonly error?: string;
}

/**
 * Page info result
 */
export interface PageInfo {
  readonly url: string;
  readonly title: string;
  readonly favicon?: string;
  readonly status: string;
  readonly viewport?: {
    readonly width: number;
    readonly height: number;
  };
}

// =============================================================================
// STATE MANAGEMENT TYPES
// =============================================================================

/**
 * Browser state snapshot
 */
export interface BrowserStateSnapshot {
  readonly url: string;
  readonly timestamp: number;
  readonly localStorage: Readonly<Record<string, string>>;
  readonly sessionStorage: Readonly<Record<string, string>>;
  readonly cookies: string;
}

/**
 * Saved state snapshot with metadata
 */
export interface SavedStateSnapshot extends BrowserStateSnapshot {
  readonly name: string;
  readonly size_bytes: number;
}

/**
 * State action types
 */
export type StateAction = 'capture' | 'save' | 'load' | 'list' | 'delete' | 'restore';

// =============================================================================
// BACKGROUND MESSAGE TYPES (chrome.runtime messages)
// =============================================================================

/**
 * Message to get current tab ID
 */
export interface GetTabIdMessage {
  readonly type: 'GET_TAB_ID';
}

export interface GetTabIdResponse {
  readonly tabId?: number;
}

/**
 * WebSocket event message from content script
 */
export interface WsEventMessage {
  readonly type: 'ws_event';
  readonly payload: WebSocketEvent;
  readonly tabId?: number;
}

/**
 * Enhanced action message from content script
 */
export interface EnhancedActionMessage {
  readonly type: 'enhanced_action';
  readonly payload: EnhancedAction;
  readonly tabId?: number;
}

/**
 * Network body message from content script
 */
export interface NetworkBodyMessage {
  readonly type: 'network_body';
  readonly payload: NetworkBodyPayload;
  readonly tabId?: number;
}

/**
 * Performance snapshot message from content script
 */
export interface PerformanceSnapshotMessage {
  readonly type: 'performance_snapshot';
  readonly payload: PerformanceSnapshot;
  readonly tabId?: number;
}

/**
 * Log message from content script
 */
export interface LogMessage {
  readonly type: 'log';
  readonly payload: LogEntry;
  readonly tabId?: number;
}

/**
 * Get extension status message
 */
export interface GetStatusMessage {
  readonly type: 'getStatus';
}

/**
 * Clear logs message
 */
export interface ClearLogsMessage {
  readonly type: 'clearLogs';
}

/**
 * Set log level message
 */
export interface SetLogLevelMessage {
  readonly type: 'setLogLevel';
  readonly level: LogLevelFilter;
}

/**
 * Toggle boolean setting messages
 */
export interface SetBooleanSettingMessage {
  readonly type:
    | 'setScreenshotOnError'
    | 'setAiWebPilotEnabled'
    | 'setSourceMapEnabled'
    | 'setNetworkWaterfallEnabled'
    | 'setPerformanceMarksEnabled'
    | 'setActionReplayEnabled'
    | 'setWebSocketCaptureEnabled'
    | 'setPerformanceSnapshotEnabled'
    | 'setDeferralEnabled'
    | 'setNetworkBodyCaptureEnabled'
    | 'setDebugMode';
  readonly enabled: boolean;
}

/**
 * Set WebSocket capture mode message
 */
export interface SetWebSocketCaptureModeMessage {
  readonly type: 'setWebSocketCaptureMode';
  readonly mode: WebSocketCaptureMode;
}

/**
 * Get AI Web Pilot enabled message
 */
export interface GetAiWebPilotEnabledMessage {
  readonly type: 'getAiWebPilotEnabled';
}

export interface GetAiWebPilotEnabledResponse {
  readonly enabled: boolean;
}

/**
 * Get diagnostic state message
 */
export interface GetDiagnosticStateMessage {
  readonly type: 'getDiagnosticState';
}

export interface GetDiagnosticStateResponse {
  readonly cache: boolean;
  readonly storage: boolean | undefined;
  readonly timestamp: string;
}

/**
 * Capture screenshot message
 */
export interface CaptureScreenshotMessage {
  readonly type: 'captureScreenshot';
}

/**
 * Debug log messages
 */
export interface GetDebugLogMessage {
  readonly type: 'getDebugLog';
}

export interface ClearDebugLogMessage {
  readonly type: 'clearDebugLog';
}

/**
 * Set server URL message
 */
export interface SetServerUrlMessage {
  readonly type: 'setServerUrl';
  readonly url: string;
}

/**
 * Status update notification (background to popup)
 */
export interface StatusUpdateMessage {
  readonly type: 'statusUpdate';
  readonly status: ConnectionStatus & { aiControlled: boolean };
}

/**
 * Union of all background-bound messages
 */
export type BackgroundMessage =
  | GetTabIdMessage
  | WsEventMessage
  | EnhancedActionMessage
  | NetworkBodyMessage
  | PerformanceSnapshotMessage
  | LogMessage
  | GetStatusMessage
  | ClearLogsMessage
  | SetLogLevelMessage
  | SetBooleanSettingMessage
  | SetWebSocketCaptureModeMessage
  | GetAiWebPilotEnabledMessage
  | GetDiagnosticStateMessage
  | CaptureScreenshotMessage
  | GetDebugLogMessage
  | ClearDebugLogMessage
  | SetServerUrlMessage;

// =============================================================================
// CONTENT SCRIPT MESSAGE TYPES (background to content)
// =============================================================================

/**
 * Ping message to check if content script is loaded
 */
export interface ContentPingMessage {
  readonly type: 'GASOLINE_PING';
}

export interface ContentPingResponse {
  readonly status: 'alive';
  readonly timestamp: number;
}

/**
 * Highlight element message
 */
export interface HighlightMessage {
  readonly type: 'GASOLINE_HIGHLIGHT';
  readonly params: {
    readonly selector: string;
    readonly duration_ms?: number;
  };
}

export interface HighlightResponse {
  readonly success: boolean;
  readonly selector?: string;
  readonly bounds?: {
    readonly x: number;
    readonly y: number;
    readonly width: number;
    readonly height: number;
  };
  readonly error?: string;
}

/**
 * Execute JavaScript message
 */
export interface ExecuteJsMessage {
  readonly type: 'GASOLINE_EXECUTE_JS';
  readonly params: {
    readonly script: string;
    readonly timeout_ms?: number;
  };
}

/**
 * Execute query message (polling system)
 */
export interface ExecuteQueryMessage {
  readonly type: 'GASOLINE_EXECUTE_QUERY';
  readonly queryId: string;
  readonly params: string | Record<string, unknown>;
}

/**
 * DOM query message
 */
export interface DomQueryMessage {
  readonly type: 'DOM_QUERY';
  readonly params: string | {
    readonly selector?: string;
    readonly limit?: number;
    readonly includeHtml?: boolean;
  };
}

/**
 * Accessibility query message
 */
export interface A11yQueryMessage {
  readonly type: 'A11Y_QUERY';
  readonly params: string | {
    readonly selector?: string;
    readonly runOnly?: string[];
  };
}

/**
 * Get network waterfall message
 */
export interface GetNetworkWaterfallMessage {
  readonly type: 'GET_NETWORK_WATERFALL';
}

/**
 * State management message
 */
export interface ManageStateMessage {
  readonly type: 'GASOLINE_MANAGE_STATE';
  readonly params: {
    readonly action: StateAction;
    readonly name?: string;
    readonly state?: BrowserStateSnapshot;
    readonly include_url?: boolean;
  };
}

/**
 * Union of all content-script-bound messages
 */
export type ContentMessage =
  | ContentPingMessage
  | HighlightMessage
  | ExecuteJsMessage
  | ExecuteQueryMessage
  | DomQueryMessage
  | A11yQueryMessage
  | GetNetworkWaterfallMessage
  | ManageStateMessage
  | SetBooleanSettingMessage
  | SetWebSocketCaptureModeMessage
  | SetServerUrlMessage;

// =============================================================================
// INJECT SCRIPT MESSAGE TYPES (postMessage between content and inject)
// =============================================================================

/**
 * Page to content script messages (postMessage types)
 */
export type PageMessageType =
  | 'GASOLINE_LOG'
  | 'GASOLINE_WS'
  | 'GASOLINE_NETWORK_BODY'
  | 'GASOLINE_ENHANCED_ACTION'
  | 'GASOLINE_PERFORMANCE_SNAPSHOT'
  | 'GASOLINE_HIGHLIGHT_RESPONSE'
  | 'GASOLINE_EXECUTE_JS_RESULT'
  | 'GASOLINE_A11Y_QUERY_RESPONSE'
  | 'GASOLINE_DOM_QUERY_RESPONSE'
  | 'GASOLINE_STATE_RESPONSE'
  | 'GASOLINE_WATERFALL_RESPONSE';

/**
 * Content to page messages (postMessage types)
 */
export type ContentToPageMessageType =
  | 'GASOLINE_SETTING'
  | 'GASOLINE_HIGHLIGHT_REQUEST'
  | 'GASOLINE_EXECUTE_JS'
  | 'GASOLINE_A11Y_QUERY'
  | 'GASOLINE_DOM_QUERY'
  | 'GASOLINE_STATE_COMMAND'
  | 'GASOLINE_GET_WATERFALL';

/**
 * Execute JS result
 */
export interface ExecuteJsResult {
  readonly success: boolean;
  readonly result?: unknown;
  readonly error?: string;
  readonly message?: string;
  readonly stack?: string;
}

// =============================================================================
// STATE TYPES
// =============================================================================

/**
 * Circuit breaker states
 */
export type CircuitBreakerState = 'closed' | 'open' | 'half-open';

/**
 * Circuit breaker statistics
 */
export interface CircuitBreakerStats {
  readonly state: CircuitBreakerState;
  readonly consecutiveFailures: number;
  readonly totalFailures: number;
  readonly totalSuccesses: number;
  readonly currentBackoff: number;
}

/**
 * Memory pressure levels
 */
export type MemoryPressureLevel = 'normal' | 'soft' | 'hard';

/**
 * Memory pressure state
 */
export interface MemoryPressureState {
  readonly memoryPressureLevel: MemoryPressureLevel;
  readonly lastMemoryCheck: number;
  readonly networkBodyCaptureDisabled: boolean;
  readonly reducedCapacities: boolean;
}

/**
 * Connection status
 */
export interface ConnectionStatus {
  readonly connected: boolean;
  readonly entries: number;
  readonly maxEntries: number;
  readonly errorCount: number;
  readonly logFile: string;
  readonly logFileSize?: number;
  readonly serverVersion?: string;
  readonly extensionVersion?: string;
  readonly versionMismatch?: boolean;
}

/**
 * Context annotation warning
 */
export interface ContextWarning {
  readonly sizeKB: number;
  readonly count: number;
  readonly triggeredAt: number;
}

/**
 * Debug log categories
 */
export type DebugCategory =
  | 'connection'
  | 'capture'
  | 'error'
  | 'lifecycle'
  | 'settings'
  | 'sourcemap'
  | 'query';

/**
 * Debug log entry
 */
export interface DebugLogEntry {
  readonly ts: string;
  readonly category: DebugCategory;
  readonly message: string;
  readonly data?: unknown;
}

/**
 * Error group for deduplication
 */
export interface ErrorGroup {
  readonly entry: LogEntry;
  readonly count: number;
  readonly firstSeen: number;
  readonly lastSeen: number;
}

/**
 * Rate limit check result
 */
export interface RateLimitResult {
  readonly allowed: boolean;
  readonly reason?: 'session_limit' | 'rate_limit';
  readonly nextAllowedIn?: number | null;
}

/**
 * Capture screenshot result
 */
export interface CaptureScreenshotResult {
  readonly success: boolean;
  readonly entry?: ScreenshotLogEntry;
  readonly error?: string;
  readonly nextAllowedIn?: number;
}

// =============================================================================
// PENDING QUERY TYPES
// =============================================================================

/**
 * Query types from server
 */
export type QueryType =
  | 'dom'
  | 'a11y'
  | 'execute'
  | 'highlight'
  | 'page_info'
  | 'tabs'
  | 'browser_action'
  | 'state_capture'
  | 'state_save'
  | 'state_load'
  | 'state_list'
  | 'state_delete';

/**
 * Pending query from server
 */
export interface PendingQuery {
  readonly id: string;
  readonly type: QueryType;
  readonly params: string | Record<string, unknown>;
  readonly correlation_id?: string;
}

/**
 * Browser action parameters
 */
export interface BrowserActionParams {
  readonly action: 'refresh' | 'navigate' | 'back' | 'forward';
  readonly url?: string;
}

/**
 * Browser action result
 */
export interface BrowserActionResult {
  readonly success: boolean;
  readonly action?: string;
  readonly url?: string;
  readonly content_script_status?: 'loaded' | 'refreshed' | 'failed' | 'unavailable';
  readonly message?: string;
  readonly error?: string;
}

/**
 * Tabs query result
 */
export interface TabInfo {
  readonly id: number;
  readonly url: string;
  readonly title: string;
  readonly active: boolean;
  readonly windowId: number;
  readonly index: number;
}

// =============================================================================
// SOURCE MAP TYPES
// =============================================================================

/**
 * Parsed source map
 */
export interface ParsedSourceMap {
  readonly sources: readonly string[];
  readonly names: readonly string[];
  readonly sourceRoot: string;
  readonly mappings: readonly (readonly (readonly number[])[])[];
  readonly sourcesContent: readonly string[];
}

/**
 * Original location from source map
 */
export interface OriginalLocation {
  readonly source: string;
  readonly line: number;
  readonly column: number;
  readonly name: string | null;
}

// =============================================================================
// CHROME API WRAPPER TYPES
// =============================================================================

/**
 * Chrome message sender info
 */
export interface ChromeMessageSender {
  readonly tab?: {
    readonly id?: number;
    readonly url?: string;
    readonly windowId?: number;
  };
  readonly frameId?: number;
  readonly url?: string;
}

/**
 * Chrome tab info
 */
export interface ChromeTabInfo {
  readonly id?: number;
  readonly url?: string;
  readonly title?: string;
  readonly windowId?: number;
  readonly status?: string;
  readonly active?: boolean;
  readonly favIconUrl?: string;
  readonly width?: number;
  readonly height?: number;
}

/**
 * Chrome storage change info
 */
export interface StorageChange<T = unknown> {
  readonly oldValue?: T;
  readonly newValue?: T;
}

/**
 * Storage area name
 */
export type StorageAreaName = 'sync' | 'local' | 'session';
