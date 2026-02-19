/**
 * Purpose: Executes in-page actions and query handlers within the page context.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */

/**
 * @fileoverview Message Handlers - Handles messages from content script including
 * settings, state management, JavaScript execution, and DOM/accessibility queries.
 */

import type { BrowserStateSnapshot, StateAction, ExecuteJsResult, WebSocketCaptureMode } from '../types/index'

import { createDeferredPromise, TimeoutError } from '../lib/timeout-utils'
import { executeDOMQuery, runAxeAuditWithTimeout, type DOMQueryParams } from '../lib/dom-queries'
import { checkLinkHealth } from '../lib/link-health'
import {
  getNetworkWaterfall,
  setNetworkWaterfallEnabled,
  setNetworkBodyCaptureEnabled,
  setServerUrl
} from '../lib/network'
import { setPerformanceMarksEnabled, installPerformanceCapture, uninstallPerformanceCapture } from '../lib/performance'
import { setActionCaptureEnabled } from '../lib/actions'
import {
  setWebSocketCaptureEnabled,
  setWebSocketCaptureMode,
  installWebSocketCapture,
  uninstallWebSocketCapture
} from '../lib/websocket'
import { setPerformanceSnapshotEnabled } from '../lib/perf-snapshot'
import { setDeferralEnabled } from './observers'

/** Read the page nonce set by the content script on the inject script element */
let pageNonce = ''
if (typeof document !== 'undefined' && typeof document.querySelector === 'function') {
  const nonceEl = document.querySelector('script[data-gasoline-nonce]')
  if (nonceEl) {
    pageNonce = nonceEl.getAttribute('data-gasoline-nonce') || ''
  }
}

/**
 * Valid setting names from content script
 */
const VALID_SETTINGS = new Set([
  'setNetworkWaterfallEnabled',
  'setPerformanceMarksEnabled',
  'setActionReplayEnabled',
  'setWebSocketCaptureEnabled',
  'setWebSocketCaptureMode',
  'setPerformanceSnapshotEnabled',
  'setDeferralEnabled',
  'setNetworkBodyCaptureEnabled',
  'setServerUrl'
])

const VALID_STATE_ACTIONS = new Set<StateAction>(['capture', 'restore'])

/**
 * Setting message from content script
 */
interface SettingMessageData {
  type: 'GASOLINE_SETTING'
  setting: string
  enabled?: boolean
  mode?: string
  url?: string
}

/**
 * State command message from content script
 */
interface StateCommandMessageData {
  type: 'GASOLINE_STATE_COMMAND'
  messageId: string
  action: StateAction
  state?: BrowserStateSnapshot
  include_url?: boolean
}

/**
 * Execute JS request message from content script
 */
interface ExecuteJsRequestMessageData {
  type: 'GASOLINE_EXECUTE_JS'
  requestId: number | string
  script: string
  timeoutMs?: number
}

/**
 * A11y query request message from content script
 */
interface A11yQueryRequestMessageData {
  type: 'GASOLINE_A11Y_QUERY'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * DOM query request message from content script
 */
interface DomQueryRequestMessageData {
  type: 'GASOLINE_DOM_QUERY'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Highlight request message from content script
 */
interface HighlightRequestMessageData {
  type: 'GASOLINE_HIGHLIGHT_REQUEST'
  requestId: number | string
  params?: {
    selector: string
    duration_ms?: number
  }
}

/**
 * Get waterfall request message from content script
 */
interface GetWaterfallRequestMessageData {
  type: 'GASOLINE_GET_WATERFALL'
  requestId: number | string
}

/**
 * Link health query request message from content script
 */
interface LinkHealthQueryRequestMessageData {
  type: 'GASOLINE_LINK_HEALTH_QUERY'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Union of all page message data types
 */
type PageMessageData =
  | SettingMessageData
  | StateCommandMessageData
  | ExecuteJsRequestMessageData
  | A11yQueryRequestMessageData
  | DomQueryRequestMessageData
  | HighlightRequestMessageData
  | GetWaterfallRequestMessageData
  | LinkHealthQueryRequestMessageData

/**
 * Safe serialization for complex objects returned from executeJavaScript.
 */
// #lizard forgives
function serializeObject(obj: object, depth: number, seen: WeakSet<object>): unknown {
  if (seen.has(obj)) return '[Circular]'
  seen.add(obj)

  if (Array.isArray(obj)) return obj.slice(0, 100).map((v) => safeSerializeForExecute(v, depth + 1, seen))
  if (obj instanceof Error) return { error: obj.message, stack: obj.stack }
  if (obj instanceof Date) return obj.toISOString()
  if (obj instanceof RegExp) return obj.toString()
  if (typeof Node !== 'undefined' && obj instanceof Node) {
    const node = obj as Node & { id?: string }
    return `[${node.nodeName}${node.id ? '#' + node.id : ''}]`
  }

  const result: Record<string, unknown> = {}
  const keys = Object.keys(obj).slice(0, 50)
  for (const key of keys) {
    try {
      result[key] = safeSerializeForExecute((obj as Record<string, unknown>)[key], depth + 1, seen)
    } catch {
      result[key] = '[unserializable]'
    }
  }
  if (Object.keys(obj).length > 50) {
    result['...'] = `[${Object.keys(obj).length - 50} more keys]`
  }
  return result
}

export function safeSerializeForExecute(
  value: unknown,
  depth: number = 0,
  seen: WeakSet<object> = new WeakSet()
): unknown {
  if (depth > 10) return '[max depth exceeded]'
  if (value === null || value === undefined) return value

  const type = typeof value
  if (type === 'string' || type === 'number' || type === 'boolean') return value
  if (type === 'function') return `[Function: ${(value as (...args: unknown[]) => unknown).name || 'anonymous'}]`
  if (type === 'symbol') return (value as symbol).toString()
  if (type === 'object') return serializeObject(value as object, depth, seen)

  return String(value)
}

/**
 * Execute arbitrary JavaScript in the page context with timeout handling.
 */
export function executeJavaScript(script: string, timeoutMs: number = 5000): Promise<ExecuteJsResult> {
  const deferred = createDeferredPromise<ExecuteJsResult>()

  // #lizard forgives
  const executeWithTimeoutProtection = async (): Promise<void> => {
    const timeoutHandle = setTimeout(() => {
      deferred.resolve({
        success: false,
        error: 'execution_timeout',
        message: `Script exceeded ${timeoutMs}ms timeout. RECOMMENDED ACTIONS:

1. Check for infinite loops or blocking operations in your script
2. Break the task into smaller pieces (< 2s execution time works best)
3. Verify the script logic - test with simpler operations first

Tip: Run small test scripts to isolate the issue, then build up complexity.`
      })
    }, timeoutMs)

    try {
      const cleanScript = script.trim()

      // Try expression form first (captures return values from IIFEs, expressions).
      // If it throws SyntaxError (statements like try/catch, if/else), fall back to statement form.
      let fn: () => unknown
      try {
        // eslint-disable-next-line no-new-func
        fn = new Function(`"use strict"; return (${cleanScript});`) as () => unknown // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
      } catch {
        // eslint-disable-next-line no-new-func
        fn = new Function(`"use strict"; ${cleanScript}`) as () => unknown // nosemgrep: javascript.lang.security.eval.rule-eval-with-expression -- Function() constructor for controlled sandbox execution
      }

      const result = fn()

      // Handle promises
      if (result && typeof (result as Promise<unknown>).then === 'function') {
        ;(result as Promise<unknown>)
          .then((value) => {
            clearTimeout(timeoutHandle)
            deferred.resolve({ success: true, result: safeSerializeForExecute(value) })
          })
          .catch((err: Error) => {
            clearTimeout(timeoutHandle)
            deferred.resolve({
              success: false,
              error: 'promise_rejected',
              message: err.message,
              stack: err.stack
            })
          })
      } else {
        clearTimeout(timeoutHandle)
        deferred.resolve({ success: true, result: safeSerializeForExecute(result) })
      }
    } catch (err) {
      clearTimeout(timeoutHandle)

      const error = err as Error
      if (
        error.message &&
        (error.message.includes('Content Security Policy') ||
          error.message.includes('unsafe-eval') ||
          error.message.includes('Trusted Type'))
      ) {
        deferred.resolve({
          success: false,
          error: 'csp_blocked',
          message:
            'This page has a Content Security Policy that blocks script execution in the MAIN world. ' +
            'Use world: "isolated" to bypass CSP (DOM access only, no page JS globals). ' +
            'With world: "auto" (default), this fallback happens automatically.'
        })
      } else {
        deferred.resolve({
          success: false,
          error: 'execution_error',
          message: error.message,
          stack: error.stack
        })
      }
    }
  }

  executeWithTimeoutProtection().catch((err) => {
    console.error('[Gasoline] Unexpected error in executeJavaScript:', err)
    deferred.resolve({
      success: false,
      error: 'execution_error',
      message: 'Unexpected error during script execution'
    })
  })

  return deferred.promise
}

/**
 * Handle link health check request from content script
 */
export async function handleLinkHealthQuery(data: LinkHealthQueryRequestMessageData): Promise<unknown> {
  try {
    const params = data.params || {}
    const result = await checkLinkHealth(params)
    return result
  } catch (err) {
    return {
      error: 'link_health_error',
      message: (err as Error).message || 'Failed to check link health'
    }
  }
}

/**
 * Install message listener for handling content script messages
 */
function isValidSettingPayload(data: SettingMessageData): boolean {
  if (!VALID_SETTINGS.has(data.setting)) {
    console.warn('[Gasoline] Invalid setting:', data.setting)
    return false
  }
  if (data.setting === 'setWebSocketCaptureMode') return typeof data.mode === 'string'
  if (data.setting === 'setServerUrl') return typeof data.url === 'string'
  // Boolean settings
  if (typeof data.enabled !== 'boolean') {
    console.warn('[Gasoline] Invalid enabled value type')
    return false
  }
  return true
}

function handleLinkHealthMessage(data: LinkHealthQueryRequestMessageData): void {
  handleLinkHealthQuery(data)
    .then((result) => {
      window.postMessage(
        { type: 'GASOLINE_LINK_HEALTH_RESPONSE', requestId: data.requestId, result },
        window.location.origin
      )
    })
    .catch((err: Error) => {
      window.postMessage(
        {
          type: 'GASOLINE_LINK_HEALTH_RESPONSE',
          requestId: data.requestId,
          result: { error: 'link_health_error', message: err.message || 'Failed to check link health' }
        },
        window.location.origin
      )
    })
}

export function installMessageListener(
  captureStateFn: () => BrowserStateSnapshot,
  restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown
): void {
  if (typeof window === 'undefined') return

  const messageHandlers: Record<string, (data: PageMessageData) => void> = {
    GASOLINE_SETTING: (data) => {
      const settingData = data as SettingMessageData
      if (isValidSettingPayload(settingData)) handleSetting(settingData)
    },
    GASOLINE_STATE_COMMAND: (data) =>
      handleStateCommand(data as StateCommandMessageData, captureStateFn, restoreStateFn),
    GASOLINE_EXECUTE_JS: (data) => handleExecuteJs(data as ExecuteJsRequestMessageData),
    GASOLINE_A11Y_QUERY: (data) => handleA11yQuery(data as A11yQueryRequestMessageData),
    GASOLINE_DOM_QUERY: (data) => handleDomQuery(data as DomQueryRequestMessageData),
    GASOLINE_GET_WATERFALL: (data) => handleGetWaterfall(data as GetWaterfallRequestMessageData),
    GASOLINE_LINK_HEALTH_QUERY: (data) => handleLinkHealthMessage(data as LinkHealthQueryRequestMessageData)
  }

  window.addEventListener('message', (event: MessageEvent<PageMessageData>) => {
    if (event.source !== window || event.origin !== window.location.origin) return
    if (pageNonce && (event.data as unknown as { _nonce?: string })?._nonce !== pageNonce) return

    const msgType = event.data?.type
    if (!msgType) return

    const handler = messageHandlers[msgType] // nosemgrep: unsafe-dynamic-method
    if (handler) handler(event.data)
  })
}

type SettingHandler = (data: SettingMessageData) => void

const SETTING_HANDLERS: Record<string, SettingHandler> = {
  setNetworkWaterfallEnabled: (data) => setNetworkWaterfallEnabled(data.enabled!),
  setPerformanceMarksEnabled: (data) => {
    setPerformanceMarksEnabled(data.enabled!)
    if (data.enabled) installPerformanceCapture()
    else uninstallPerformanceCapture()
  },
  setActionReplayEnabled: (data) => setActionCaptureEnabled(data.enabled!),
  setWebSocketCaptureEnabled: (data) => {
    setWebSocketCaptureEnabled(data.enabled!)
    if (data.enabled) installWebSocketCapture()
    else uninstallWebSocketCapture()
  },
  setWebSocketCaptureMode: (data) => setWebSocketCaptureMode((data.mode || 'medium') as WebSocketCaptureMode),
  setPerformanceSnapshotEnabled: (data) => setPerformanceSnapshotEnabled(data.enabled!),
  setDeferralEnabled: (data) => setDeferralEnabled(data.enabled!),
  setNetworkBodyCaptureEnabled: (data) => setNetworkBodyCaptureEnabled(data.enabled!),
  setServerUrl: (data) => setServerUrl(data.url!)
}

function handleSetting(data: SettingMessageData): void {
  const handler = SETTING_HANDLERS[data.setting]
  if (handler) handler(data)
}

function handleStateCommand(
  data: StateCommandMessageData,
  captureStateFn: () => BrowserStateSnapshot,
  restoreStateFn: (state: BrowserStateSnapshot, includeUrl: boolean) => unknown
): void {
  const { messageId, action, state } = data

  // Validate action
  if (!VALID_STATE_ACTIONS.has(action)) {
    console.warn('[Gasoline] Invalid state action:', action)
    window.postMessage(
      {
        type: 'GASOLINE_STATE_RESPONSE',
        messageId,
        result: { error: `Invalid action: ${action}` }
      },
      window.location.origin
    )
    return
  }

  // Validate state object for restore action
  if (action === 'restore' && (!state || typeof state !== 'object')) {
    console.warn('[Gasoline] Invalid state object for restore')
    window.postMessage(
      {
        type: 'GASOLINE_STATE_RESPONSE',
        messageId,
        result: { error: 'Invalid state object' }
      },
      window.location.origin
    )
    return
  }

  let result: BrowserStateSnapshot | unknown

  try {
    if (action === 'capture') {
      result = captureStateFn()
    } else if (action === 'restore') {
      const includeUrl = data.include_url !== false
      result = restoreStateFn(state!, includeUrl)
    } else {
      result = { error: `Unknown action: ${action}` }
    }
  } catch (err) {
    result = { error: (err as Error).message }
  }

  // Send response back to content script
  window.postMessage(
    {
      type: 'GASOLINE_STATE_RESPONSE',
      messageId,
      result
    },
    window.location.origin
  )
}

function handleExecuteJs(data: ExecuteJsRequestMessageData): void {
  const { requestId, script, timeoutMs } = data

  // Validate parameters
  if (typeof script !== 'string') {
    console.warn('[Gasoline] Script must be a string')
    window.postMessage(
      {
        type: 'GASOLINE_EXECUTE_JS_RESULT',
        requestId,
        result: { success: false, error: 'invalid_script', message: 'Script must be a string' }
      },
      window.location.origin
    )
    return
  }

  if (typeof requestId !== 'number' && typeof requestId !== 'string') {
    console.warn('[Gasoline] Invalid requestId type')
    return
  }

  executeJavaScript(script, timeoutMs)
    .then((result) => {
      window.postMessage(
        {
          type: 'GASOLINE_EXECUTE_JS_RESULT',
          requestId,
          result
        },
        window.location.origin
      )
    })
    .catch((err: Error) => {
      console.error('[Gasoline] Failed to execute JS:', err)
      window.postMessage(
        {
          type: 'GASOLINE_EXECUTE_JS_RESULT',
          requestId,
          result: { success: false, error: 'execution_failed', message: err.message }
        },
        window.location.origin
      )
    })
}

function handleA11yQuery(data: A11yQueryRequestMessageData): void {
  const { requestId, params } = data

  if (typeof runAxeAuditWithTimeout !== 'function') {
    window.postMessage(
      {
        type: 'GASOLINE_A11Y_QUERY_RESPONSE',
        requestId,
        result: {
          error: 'runAxeAuditWithTimeout not available - try reloading the extension'
        }
      },
      window.location.origin
    )
    return
  }

  try {
    runAxeAuditWithTimeout(params || {})
      .then((result) => {
        window.postMessage(
          {
            type: 'GASOLINE_A11Y_QUERY_RESPONSE',
            requestId,
            result
          },
          window.location.origin
        )
      })
      .catch((err: Error) => {
        console.error('[Gasoline] Accessibility audit error:', err)
        window.postMessage(
          {
            type: 'GASOLINE_A11Y_QUERY_RESPONSE',
            requestId,
            result: { error: err.message || 'Accessibility audit failed' }
          },
          window.location.origin
        )
      })
  } catch (err) {
    console.error('[Gasoline] Failed to run accessibility audit:', err)
    window.postMessage(
      {
        type: 'GASOLINE_A11Y_QUERY_RESPONSE',
        requestId,
        result: { error: (err as Error).message || 'Failed to run accessibility audit' }
      },
      window.location.origin
    )
  }
}

function handleDomQuery(data: DomQueryRequestMessageData): void {
  const { requestId, params } = data

  if (typeof executeDOMQuery !== 'function') {
    window.postMessage(
      {
        type: 'GASOLINE_DOM_QUERY_RESPONSE',
        requestId,
        result: {
          error: 'executeDOMQuery not available - try reloading the extension'
        }
      },
      window.location.origin
    )
    return
  }

  try {
    executeDOMQuery((params || {}) as unknown as DOMQueryParams)
      .then((result) => {
        window.postMessage(
          {
            type: 'GASOLINE_DOM_QUERY_RESPONSE',
            requestId,
            result
          },
          window.location.origin
        )
      })
      .catch((err: Error) => {
        console.error('[Gasoline] DOM query error:', err)
        window.postMessage(
          {
            type: 'GASOLINE_DOM_QUERY_RESPONSE',
            requestId,
            result: { error: err.message || 'DOM query failed' }
          },
          window.location.origin
        )
      })
  } catch (err) {
    console.error('[Gasoline] Failed to run DOM query:', err)
    window.postMessage(
      {
        type: 'GASOLINE_DOM_QUERY_RESPONSE',
        requestId,
        result: { error: (err as Error).message || 'Failed to run DOM query' }
      },
      window.location.origin
    )
  }
}

function handleGetWaterfall(data: GetWaterfallRequestMessageData): void {
  const { requestId } = data

  try {
    const entries = getNetworkWaterfall({})

    window.postMessage(
      {
        type: 'GASOLINE_WATERFALL_RESPONSE',
        requestId,
        entries: entries || [],
        page_url: window.location.href
      },
      window.location.origin
    )
  } catch (err) {
    console.error('[Gasoline] Failed to get network waterfall:', err)
    window.postMessage(
      {
        type: 'GASOLINE_WATERFALL_RESPONSE',
        requestId,
        entries: []
      },
      window.location.origin
    )
  }
}
