// message-handlers.ts — Message dispatch from content script to inject-context handlers.

/**
 * @fileoverview Message Handlers - Dispatches messages from content script to
 * specialized modules for settings, state management, JavaScript execution,
 * and DOM/accessibility queries.
 */

import type { BrowserStateSnapshot } from '../types/index'

import { executeDOMQuery, runAxeAuditWithTimeout, type DOMQueryParams } from '../lib/dom-queries'
import { checkLinkHealth } from '../lib/link-health'
import { getNetworkWaterfall } from '../lib/network'

import { executeJavaScript } from './execute-js'
import {
  isValidSettingPayload,
  handleSetting,
  handleStateCommand,
  type SettingMessageData,
  type StateCommandMessageData
} from './settings'

// Re-export for barrel (src/inject/index.ts)
export { executeJavaScript, safeSerializeForExecute } from './execute-js'

/** Read the page nonce set by the content script on the inject script element */
let pageNonce = ''
if (typeof document !== 'undefined' && typeof document.querySelector === 'function') {
  const nonceEl = document.querySelector('script[data-gasoline-nonce]')
  if (nonceEl) {
    pageNonce = nonceEl.getAttribute('data-gasoline-nonce') || ''
  }
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
