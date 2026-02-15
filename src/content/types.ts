/**
 * @fileoverview Content Script Internal Types
 * Type definitions for internal content script use
 */

import type {
  WebSocketCaptureMode,
  StateAction,
  BrowserStateSnapshot,
  PageMessageType,
  ContentToPageMessageType,
  LogEntry,
  WebSocketEvent,
  NetworkBodyPayload,
  EnhancedAction,
  PerformanceSnapshot
} from '../types'

/**
 * Pending request statistics
 */
export interface PendingRequestStats {
  readonly highlight: number
  readonly execute: number
  readonly a11y: number
  readonly dom: number
}

/**
 * Page message event data from inject.js
 */
export interface PageMessageEventData {
  type?: PageMessageType
  requestId?: number
  result?: unknown
  payload?: unknown
}

/**
 * Setting message to be posted to page context
 */
export interface SettingMessage {
  type: 'GASOLINE_SETTING'
  setting: string
  enabled?: boolean
  mode?: WebSocketCaptureMode
  url?: string
}

/**
 * Highlight request message to page context
 */
export interface HighlightRequestMessage {
  type: 'GASOLINE_HIGHLIGHT_REQUEST'
  requestId: number
  params: {
    selector: string
    duration_ms?: number
  }
}

/**
 * Execute JS request message to page context
 */
export interface ExecuteJsRequestMessage {
  type: 'GASOLINE_EXECUTE_JS'
  requestId: number
  script: string
  timeoutMs: number
}

/**
 * A11y query request message to page context
 */
export interface A11yQueryRequestMessage {
  type: 'GASOLINE_A11Y_QUERY'
  requestId: number
  params: Record<string, unknown>
}

/**
 * DOM query request message to page context
 */
export interface DomQueryRequestMessage {
  type: 'GASOLINE_DOM_QUERY'
  requestId: number
  params: Record<string, unknown>
}

/**
 * Get waterfall request message to page context
 */
export interface GetWaterfallRequestMessage {
  type: 'GASOLINE_GET_WATERFALL'
  requestId: number
}

/**
 * State command message to page context
 */
export interface StateCommandMessage {
  type: 'GASOLINE_STATE_COMMAND'
  messageId: string
  action?: StateAction
  name?: string
  state?: BrowserStateSnapshot
  include_url?: boolean
}

/**
 * Union of all messages posted to page context
 */
export type PagePostMessage =
  | SettingMessage
  | HighlightRequestMessage
  | ExecuteJsRequestMessage
  | A11yQueryRequestMessage
  | DomQueryRequestMessage
  | GetWaterfallRequestMessage
  | StateCommandMessage

/**
 * Background message types sent from content script
 */
export interface LogMessageToBackground {
  type: 'log'
  payload: LogEntry
  tabId: number | null
}

export interface WsEventMessageToBackground {
  type: 'ws_event'
  payload: WebSocketEvent
  tabId: number | null
}

export interface NetworkBodyMessageToBackground {
  type: 'network_body'
  payload: NetworkBodyPayload
  tabId: number | null
}

export interface EnhancedActionMessageToBackground {
  type: 'enhanced_action'
  payload: EnhancedAction
  tabId: number | null
}

export interface PerformanceSnapshotMessageToBackground {
  type: 'performance_snapshot'
  payload: PerformanceSnapshot
  tabId: number | null
}

export type BackgroundMessageFromContent =
  | LogMessageToBackground
  | WsEventMessageToBackground
  | NetworkBodyMessageToBackground
  | EnhancedActionMessageToBackground
  | PerformanceSnapshotMessageToBackground
