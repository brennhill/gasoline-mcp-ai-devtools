/**
 * Purpose: Dispatches window.postMessage commands from the content script to specialized inject-context handlers (settings, state, JS execution, DOM/a11y queries).
 * Docs: docs/features/feature/interact-explore/index.md
 */

// message-handlers.ts — Message dispatch from content script to inject-context handlers.

/**
 * @fileoverview Message Handlers - Dispatches messages from content script to
 * specialized modules for settings, state management, JavaScript execution,
 * and DOM/accessibility queries.
 */

import type { BrowserStateSnapshot } from '../types/index.js'

import { executeDOMQuery, runAxeAuditWithTimeout, type DOMQueryParams } from '../lib/dom-queries.js'
import { checkLinkHealth } from '../lib/link-health.js'
import { queryComputedStyles } from './computed-styles.js'
import { discoverForms } from './form-discovery.js'
import { extractDataTables } from './data-table.js'
import { getNetworkWaterfall } from '../lib/network.js'

import { executeJavaScript } from './execute-js.js'
import { errorMessage } from '../lib/error-utils.js'
import {
  isValidSettingPayload,
  handleSetting,
  handleStateCommand,
  type SettingMessageData,
  type StateCommandMessageData
} from './settings.js'

// Re-export for barrel (src/inject/index.ts)
export { executeJavaScript, safeSerializeForExecute } from './execute-js.js'

/** Read the page nonce set by the content script on the inject script element */
let pageNonce = ''
if (typeof document !== 'undefined' && typeof document.querySelector === 'function') {
  const nonceEl = document.querySelector('script[data-gasoline-nonce]')
  if (nonceEl) {
    pageNonce = nonceEl.getAttribute('data-gasoline-nonce') || ''
  }
}

/** Send a nonce-authenticated response back to the content script */
function postResponse(data: Record<string, unknown>): void {
  window.postMessage({ ...data, _nonce: pageNonce }, window.location.origin)
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
 * Computed styles query request message from content script
 */
interface ComputedStylesQueryRequestMessageData {
  type: 'GASOLINE_COMPUTED_STYLES_QUERY'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Form discovery query request message from content script
 */
interface FormDiscoveryQueryRequestMessageData {
  type: 'GASOLINE_FORM_DISCOVERY_QUERY'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Form state query request message from content script
 */
interface FormStateQueryRequestMessageData {
  type: 'GASOLINE_FORM_STATE_QUERY'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Data table query request message from content script
 */
interface DataTableQueryRequestMessageData {
  type: 'GASOLINE_DATA_TABLE_QUERY'
  requestId: number | string
  params?: Record<string, unknown>
}

/**
 * Bridge readiness ping from content script to inject context
 */
interface BridgePingMessageData {
  type: 'GASOLINE_INJECT_BRIDGE_PING'
  requestId: number | string
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
  | ComputedStylesQueryRequestMessageData
  | FormDiscoveryQueryRequestMessageData
  | FormStateQueryRequestMessageData
  | DataTableQueryRequestMessageData
  | BridgePingMessageData

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
      message: errorMessage(err, 'Failed to check link health')
    }
  }
}

/**
 * Install message listener for handling content script messages
 */
function handleLinkHealthMessage(data: LinkHealthQueryRequestMessageData): void {
  handleLinkHealthQuery(data)
    .then((result) => {
      postResponse({ type: 'GASOLINE_LINK_HEALTH_RESPONSE', requestId: data.requestId, result })
    })
    .catch((err: Error) => {
      postResponse({
        type: 'GASOLINE_LINK_HEALTH_RESPONSE',
        requestId: data.requestId,
        result: { error: 'link_health_error', message: err.message || 'Failed to check link health' }
      })
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
    GASOLINE_LINK_HEALTH_QUERY: (data) => handleLinkHealthMessage(data as LinkHealthQueryRequestMessageData),
    GASOLINE_COMPUTED_STYLES_QUERY: (data) =>
      handleComputedStylesMessage(data as ComputedStylesQueryRequestMessageData),
    GASOLINE_FORM_DISCOVERY_QUERY: (data) => handleFormDiscoveryMessage(data as FormDiscoveryQueryRequestMessageData),
    GASOLINE_FORM_STATE_QUERY: (data) => handleFormStateMessage(data as FormStateQueryRequestMessageData),
    GASOLINE_DATA_TABLE_QUERY: (data) => handleDataTableMessage(data as DataTableQueryRequestMessageData),
    GASOLINE_INJECT_BRIDGE_PING: (data) => handleBridgePingMessage(data as BridgePingMessageData)
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

function handleBridgePingMessage(data: BridgePingMessageData): void {
  postResponse({
    type: 'GASOLINE_INJECT_BRIDGE_PONG',
    requestId: data.requestId
  })
}

function handleComputedStylesMessage(data: ComputedStylesQueryRequestMessageData): void {
  try {
    const params = (data.params || {}) as { selector?: string; properties?: string[] }
    const result = queryComputedStyles({
      selector: params.selector || '*',
      properties: params.properties
    })
    postResponse({
      type: 'GASOLINE_COMPUTED_STYLES_RESPONSE',
      requestId: data.requestId,
      result: { elements: result, count: result.length }
    })
  } catch (err) {
    postResponse({
      type: 'GASOLINE_COMPUTED_STYLES_RESPONSE',
      requestId: data.requestId,
      result: { error: 'computed_styles_error', message: errorMessage(err, 'Failed to query computed styles') }
    })
  }
}

function handleFormDiscoveryMessage(data: FormDiscoveryQueryRequestMessageData): void {
  try {
    const params = (data.params || {}) as { selector?: string; mode?: string }
    const result = discoverForms({
      selector: params.selector,
      mode: params.mode === 'validate' ? 'validate' : 'discover'
    })
    postResponse({
      type: 'GASOLINE_FORM_DISCOVERY_RESPONSE',
      requestId: data.requestId,
      result: { forms: result, count: result.length }
    })
  } catch (err) {
    postResponse({
      type: 'GASOLINE_FORM_DISCOVERY_RESPONSE',
      requestId: data.requestId,
      result: { error: 'form_discovery_error', message: errorMessage(err, 'Failed to discover forms') }
    })
  }
}

function handleFormStateMessage(data: FormStateQueryRequestMessageData): void {
  try {
    const params = (data.params || {}) as { selector?: string }
    const forms = discoverForms({
      selector: params.selector,
      mode: 'discover'
    })
    postResponse({
      type: 'GASOLINE_FORM_STATE_RESPONSE',
      requestId: data.requestId,
      result: { forms, count: forms.length }
    })
  } catch (err) {
    postResponse({
      type: 'GASOLINE_FORM_STATE_RESPONSE',
      requestId: data.requestId,
      result: { error: 'form_state_error', message: errorMessage(err, 'Failed to extract form state') }
    })
  }
}

function handleDataTableMessage(data: DataTableQueryRequestMessageData): void {
  try {
    const params = (data.params || {}) as { selector?: string; max_rows?: number; max_cols?: number }
    const result = extractDataTables({
      selector: params.selector,
      max_rows: params.max_rows,
      max_cols: params.max_cols
    })
    postResponse({
      type: 'GASOLINE_DATA_TABLE_RESPONSE',
      requestId: data.requestId,
      result
    })
  } catch (err) {
    postResponse({
      type: 'GASOLINE_DATA_TABLE_RESPONSE',
      requestId: data.requestId,
      result: { error: 'data_table_error', message: errorMessage(err, 'Failed to extract table data') }
    })
  }
}

function handleExecuteJs(data: ExecuteJsRequestMessageData): void {
  const { requestId, script, timeoutMs } = data

  // Validate parameters
  if (typeof script !== 'string') {
    console.warn('[Gasoline] Script must be a string')
    postResponse({
      type: 'GASOLINE_EXECUTE_JS_RESULT',
      requestId,
      result: { success: false, error: 'invalid_script', message: 'Script must be a string' }
    })
    return
  }

  if (typeof requestId !== 'number' && typeof requestId !== 'string') {
    console.warn('[Gasoline] Invalid requestId type')
    return
  }

  executeJavaScript(script, timeoutMs)
    .then((result) => {
      postResponse({
        type: 'GASOLINE_EXECUTE_JS_RESULT',
        requestId,
        result
      })
    })
    .catch((err: Error) => {
      console.error('[Gasoline] Failed to execute JS:', err)
      postResponse({
        type: 'GASOLINE_EXECUTE_JS_RESULT',
        requestId,
        result: { success: false, error: 'execution_failed', message: err.message }
      })
    })
}

function handleA11yQuery(data: A11yQueryRequestMessageData): void {
  const { requestId, params } = data

  if (typeof runAxeAuditWithTimeout !== 'function') {
    postResponse({
      type: 'GASOLINE_A11Y_QUERY_RESPONSE',
      requestId,
      result: {
        error: 'runAxeAuditWithTimeout not available - try reloading the extension'
      }
    })
    return
  }

  try {
    runAxeAuditWithTimeout(params || {})
      .then((result) => {
        postResponse({
          type: 'GASOLINE_A11Y_QUERY_RESPONSE',
          requestId,
          result
        })
      })
      .catch((err: Error) => {
        console.error('[Gasoline] Accessibility audit error:', err)
        postResponse({
          type: 'GASOLINE_A11Y_QUERY_RESPONSE',
          requestId,
          result: { error: err.message || 'Accessibility audit failed' }
        })
      })
  } catch (err) {
    console.error('[Gasoline] Failed to run accessibility audit:', err)
    postResponse({
      type: 'GASOLINE_A11Y_QUERY_RESPONSE',
      requestId,
      result: { error: errorMessage(err, 'Failed to run accessibility audit') }
    })
  }
}

function handleDomQuery(data: DomQueryRequestMessageData): void {
  const { requestId, params } = data

  if (typeof executeDOMQuery !== 'function') {
    postResponse({
      type: 'GASOLINE_DOM_QUERY_RESPONSE',
      requestId,
      result: {
        error: 'executeDOMQuery not available - try reloading the extension'
      }
    })
    return
  }

  try {
    executeDOMQuery((params || {}) as unknown as DOMQueryParams)
      .then((result) => {
        postResponse({
          type: 'GASOLINE_DOM_QUERY_RESPONSE',
          requestId,
          result
        })
      })
      .catch((err: Error) => {
        console.error('[Gasoline] DOM query error:', err)
        postResponse({
          type: 'GASOLINE_DOM_QUERY_RESPONSE',
          requestId,
          result: { error: err.message || 'DOM query failed' }
        })
      })
  } catch (err) {
    console.error('[Gasoline] Failed to run DOM query:', err)
    postResponse({
      type: 'GASOLINE_DOM_QUERY_RESPONSE',
      requestId,
      result: { error: errorMessage(err, 'Failed to run DOM query') }
    })
  }
}

function handleGetWaterfall(data: GetWaterfallRequestMessageData): void {
  const { requestId } = data

  try {
    const entries = getNetworkWaterfall({})

    postResponse({
      type: 'GASOLINE_WATERFALL_RESPONSE',
      requestId,
      entries: entries || [],
      page_url: window.location.href
    })
  } catch (err) {
    console.error('[Gasoline] Failed to get network waterfall:', err)
    postResponse({
      type: 'GASOLINE_WATERFALL_RESPONSE',
      requestId,
      entries: []
    })
  }
}
