/**
 * Purpose: Owns runtime-messages.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */

/**
 * @fileoverview Runtime Message Types
 * Chrome runtime messages for background, content, and inject script communication
 */

import type { LogEntry, ScreenshotLogEntry } from './telemetry'
import type { WebSocketEvent, WebSocketCaptureMode } from './websocket'
import type { NetworkBodyPayload, WaterfallEntry } from './network'
import type { EnhancedAction } from './actions'
import type { PerformanceSnapshot } from './performance'
import type { LogLevelFilter } from './telemetry'
import type { ConnectionStatus } from './state'
import type { BrowserStateSnapshot, StateAction } from './state'
import type { DomQueryResult } from './dom'
import type { A11yAuditResult } from './accessibility'

// =============================================================================
// BACKGROUND MESSAGE TYPES (chrome.runtime messages)
// =============================================================================

/**
 * Message to get current tab ID
 */
export interface GetTabIdMessage {
  readonly type: 'GET_TAB_ID'
}

export interface GetTabIdResponse {
  readonly tabId?: number
}

/**
 * WebSocket event message from content script
 */
export interface WsEventMessage {
  readonly type: 'ws_event'
  readonly payload: WebSocketEvent
  readonly tabId?: number
}

/**
 * Enhanced action message from content script
 */
export interface EnhancedActionMessage {
  readonly type: 'enhanced_action'
  readonly payload: EnhancedAction
  readonly tabId?: number
}

/**
 * Network body message from content script
 */
export interface NetworkBodyMessage {
  readonly type: 'network_body'
  readonly payload: NetworkBodyPayload
  readonly tabId?: number
}

/**
 * Performance snapshot message from content script
 */
export interface PerformanceSnapshotMessage {
  readonly type: 'performance_snapshot'
  readonly payload: PerformanceSnapshot
  readonly tabId?: number
}

/**
 * Log message from content script
 */
export interface LogMessage {
  readonly type: 'log'
  readonly payload: LogEntry
  readonly tabId?: number
}

/**
 * Get extension status message
 */
export interface GetStatusMessage {
  readonly type: 'getStatus'
}

/**
 * Clear logs message
 */
export interface ClearLogsMessage {
  readonly type: 'clearLogs'
}

/**
 * Set log level message
 */
export interface SetLogLevelMessage {
  readonly type: 'setLogLevel'
  readonly level: LogLevelFilter
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
    | 'setActionToastsEnabled'
    | 'setSubtitlesEnabled'
    | 'setDebugMode'
  readonly enabled: boolean
}

/**
 * Set WebSocket capture mode message
 */
export interface SetWebSocketCaptureModeMessage {
  readonly type: 'setWebSocketCaptureMode'
  readonly mode: WebSocketCaptureMode
}

/**
 * Get AI Web Pilot enabled message
 */
export interface GetAiWebPilotEnabledMessage {
  readonly type: 'getAiWebPilotEnabled'
}

export interface GetAiWebPilotEnabledResponse {
  readonly enabled: boolean
}

/**
 * Get tracking state message (for favicon replacer)
 */
export interface GetTrackingStateMessage {
  readonly type: 'getTrackingState'
}

export interface GetTrackingStateResponse {
  readonly state: {
    isTracked: boolean
    aiPilotEnabled: boolean
  }
}

/**
 * Get diagnostic state message
 */
export interface GetDiagnosticStateMessage {
  readonly type: 'getDiagnosticState'
}

export interface GetDiagnosticStateResponse {
  readonly cache: boolean
  readonly storage: boolean | undefined
  readonly timestamp: string
}

/**
 * Capture screenshot message
 */
export interface CaptureScreenshotMessage {
  readonly type: 'captureScreenshot'
}

/**
 * Debug log messages
 */
export interface GetDebugLogMessage {
  readonly type: 'getDebugLog'
}

export interface ClearDebugLogMessage {
  readonly type: 'clearDebugLog'
}

/**
 * Set server URL message
 */
export interface SetServerUrlMessage {
  readonly type: 'setServerUrl'
  readonly url: string
}

/**
 * Status update notification (background to popup)
 */
export interface StatusUpdateMessage {
  readonly type: 'statusUpdate'
  readonly status: ConnectionStatus & { aiControlled: boolean }
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
  | GetTrackingStateMessage
  | GetDiagnosticStateMessage
  | CaptureScreenshotMessage
  | GetDebugLogMessage
  | ClearDebugLogMessage
  | SetServerUrlMessage
  | DrawModeCaptureScreenshotMessage
  | DrawModeCompletedMessage

/**
 * Draw mode: content script requests screenshot capture
 */
export interface DrawModeCaptureScreenshotMessage {
  readonly type: 'GASOLINE_CAPTURE_SCREENSHOT'
}

/**
 * Draw mode: content script sends completed annotation results.
 * Fields match the wire format sent by extension/content/draw-mode.js.
 */
export interface DrawModeCompletedMessage {
  readonly type: 'DRAW_MODE_COMPLETED'
  readonly annotations?: readonly unknown[]
  readonly screenshot_data_url?: string
  readonly elementDetails?: Readonly<Record<string, unknown>>
  readonly page_url?: string
  readonly correlation_id?: string
  readonly annot_session_name?: string
}

// =============================================================================
// CONTENT SCRIPT MESSAGE TYPES (background to content)
// =============================================================================

/**
 * Ping message to check if content script is loaded
 */
export interface ContentPingMessage {
  readonly type: 'GASOLINE_PING'
}

export interface ContentPingResponse {
  readonly status: 'alive'
  readonly timestamp: number
}

/**
 * Highlight element message
 */
export interface HighlightMessage {
  readonly type: 'GASOLINE_HIGHLIGHT'
  readonly params: {
    readonly selector: string
    readonly duration_ms?: number
  }
}

export interface HighlightResponse {
  readonly success: boolean
  readonly selector?: string
  readonly bounds?: {
    readonly x: number
    readonly y: number
    readonly width: number
    readonly height: number
  }
  readonly error?: string
}

/**
 * Execute JavaScript message
 */
export interface ExecuteJsMessage {
  readonly type: 'GASOLINE_EXECUTE_JS'
  readonly params: {
    readonly script: string
    readonly timeout_ms?: number
  }
}

/**
 * Execute query message (polling system)
 */
export interface ExecuteQueryMessage {
  readonly type: 'GASOLINE_EXECUTE_QUERY'
  readonly queryId: string
  readonly params: string | Record<string, unknown>
}

/**
 * DOM query message
 */
export interface DomQueryMessage {
  readonly type: 'DOM_QUERY'
  readonly params:
    | string
    | {
        readonly selector?: string
        readonly limit?: number
        readonly includeHtml?: boolean
      }
}

/**
 * Accessibility query message
 */
export interface A11yQueryMessage {
  readonly type: 'A11Y_QUERY'
  readonly params:
    | string
    | {
        readonly selector?: string
        readonly runOnly?: string[]
      }
}

/**
 * Get network waterfall message
 */
export interface GetNetworkWaterfallMessage {
  readonly type: 'GET_NETWORK_WATERFALL'
}

/**
 * Link health check message
 */
export interface LinkHealthMessage {
  readonly type: 'LINK_HEALTH_QUERY'
  readonly params?: string | Record<string, unknown>
}

/**
 * Computed styles query message
 */
export interface ComputedStylesQueryMessage {
  readonly type: 'COMPUTED_STYLES_QUERY'
  readonly params?: string | Record<string, unknown>
}

/**
 * Form discovery query message
 */
export interface FormDiscoveryQueryMessage {
  readonly type: 'FORM_DISCOVERY_QUERY'
  readonly params?: string | Record<string, unknown>
}

/**
 * Draw mode control messages (background to content)
 */
export interface DrawModeStartMessage {
  readonly type: 'GASOLINE_DRAW_MODE_START'
  readonly started_by?: string
  readonly annot_session_name?: string
  readonly correlation_id?: string
}

export interface DrawModeStopMessage {
  readonly type: 'GASOLINE_DRAW_MODE_STOP'
}

export interface GetAnnotationsMessage {
  readonly type: 'GASOLINE_GET_ANNOTATIONS'
}

/**
 * Tracking state change notification (background to content)
 */
export interface TrackingStateChangedMessage {
  readonly type: 'trackingStateChanged'
  readonly isTracked: boolean
  readonly aiPilotEnabled: boolean
}

/**
 * State management message
 */
export interface ManageStateMessage {
  readonly type: 'GASOLINE_MANAGE_STATE'
  readonly params: {
    readonly action: StateAction
    readonly name?: string
    readonly state?: BrowserStateSnapshot
    readonly include_url?: boolean
  }
}

/**
 * Action toast message — visual indicator for AI actions.
 * Supports color-coded states: trying (blue), success (green), warning (amber), error (red), audio (orange with animation).
 */
export interface ActionToastMessage {
  readonly type: 'GASOLINE_ACTION_TOAST'
  readonly text: string
  readonly detail?: string
  readonly state?: 'trying' | 'success' | 'warning' | 'error' | 'audio'
  readonly duration_ms?: number
}

/**
 * Subtitle overlay message (persistent narration text)
 */
export interface SubtitleMessage {
  readonly type: 'GASOLINE_SUBTITLE'
  readonly text: string
}

/**
 * Recording watermark overlay message
 */
export interface RecordingWatermarkMessage {
  readonly type: 'GASOLINE_RECORDING_WATERMARK'
  readonly visible: boolean
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
  | LinkHealthMessage
  | ComputedStylesQueryMessage
  | FormDiscoveryQueryMessage
  | ManageStateMessage
  | ActionToastMessage
  | SubtitleMessage
  | RecordingWatermarkMessage
  | DrawModeStartMessage
  | DrawModeStopMessage
  | GetAnnotationsMessage
  | TrackingStateChangedMessage
  | SetBooleanSettingMessage
  | SetWebSocketCaptureModeMessage
  | SetServerUrlMessage

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
  | 'GASOLINE_WATERFALL_RESPONSE'
  | 'GASOLINE_LINK_HEALTH_RESPONSE'

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
  | 'GASOLINE_GET_WATERFALL'
  | 'GASOLINE_LINK_HEALTH_QUERY'

// =============================================================================
// OFFSCREEN DOCUMENT MESSAGE TYPES (service worker ↔ offscreen)
// =============================================================================

/**
 * Start recording message (SW → offscreen)
 */
export interface OffscreenStartRecordingMessage {
  readonly target: 'offscreen'
  readonly type: 'OFFSCREEN_START_RECORDING'
  readonly streamId: string
  readonly serverUrl: string
  readonly name: string
  readonly fps: number
  readonly audioMode: string
  readonly tabId: number
  readonly url: string
}

/**
 * Stop recording message (SW → offscreen)
 */
export interface OffscreenStopRecordingMessage {
  readonly target: 'offscreen'
  readonly type: 'OFFSCREEN_STOP_RECORDING'
}

/**
 * Recording started confirmation (offscreen → SW)
 */
export interface OffscreenRecordingStartedMessage {
  readonly target: 'background'
  readonly type: 'OFFSCREEN_RECORDING_STARTED'
  readonly success: boolean
  readonly error?: string
}

/**
 * Recording stopped result (offscreen → SW)
 */
export interface OffscreenRecordingStoppedMessage {
  readonly target: 'background'
  readonly type: 'OFFSCREEN_RECORDING_STOPPED'
  readonly status: string
  readonly name: string
  readonly duration_seconds?: number
  readonly size_bytes?: number
  readonly truncated?: boolean
  readonly path?: string
  readonly error?: string
}

/**
 * Union of offscreen messages
 */
export type OffscreenMessage =
  | OffscreenStartRecordingMessage
  | OffscreenStopRecordingMessage
  | OffscreenRecordingStartedMessage
  | OffscreenRecordingStoppedMessage

/**
 * Execute JS result
 */
export interface ExecuteJsResult {
  readonly success: boolean
  readonly result?: unknown
  readonly error?: string
  readonly message?: string
  readonly stack?: string
}
