/**
 * @fileoverview Message Handlers Module
 * Handles messages from background script
 */

import type {
  ContentMessage,
  ContentPingResponse,
  WebSocketCaptureMode,
  HighlightResponse,
  WaterfallEntry,
  StateAction,
  BrowserStateSnapshot,
  A11yAuditResult,
} from '../types'
import type {
  SettingMessage,
  HighlightRequestMessage,
  ExecuteJsRequestMessage,
  A11yQueryRequestMessage,
  DomQueryRequestMessage,
  GetWaterfallRequestMessage,
  StateCommandMessage,
} from './types'
import {
  registerHighlightRequest,
  hasHighlightRequest,
  deleteHighlightRequest,
  registerExecuteRequest,
  registerA11yRequest,
  registerDomRequest,
} from './request-tracking'
import { createDeferredPromise, promiseRaceWithCleanup } from './timeout-utils'
import { isInjectScriptLoaded } from './script-injection'

// Feature toggle message types forwarded from background to inject.js
export const TOGGLE_MESSAGES = new Set([
  'setNetworkWaterfallEnabled',
  'setPerformanceMarksEnabled',
  'setActionReplayEnabled',
  'setWebSocketCaptureEnabled',
  'setWebSocketCaptureMode',
  'setPerformanceSnapshotEnabled',
  'setDeferralEnabled',
  'setNetworkBodyCaptureEnabled',
  'setServerUrl',
])

/**
 * Security: Validate sender is from the extension background script
 * Prevents content script from trusting messages from compromised page context
 */
export function isValidBackgroundSender(sender: chrome.runtime.MessageSender): boolean {
  // Messages from background should NOT have a tab (or have tab with chrome-extension:// url)
  // Messages from content scripts have tab.id
  // We only want messages from the background service worker
  return typeof sender.id === 'string' && sender.id === chrome.runtime.id
}

/**
 * Create a timeout handler that cleans up a pending request from a Map
 */
function createRequestTimeoutCleanup<T extends { error: string }>(
  requestId: number,
  pendingMap: Map<number, (result: T) => void>,
  errorResponse: T,
): () => void {
  return () => {
    if (pendingMap.has(requestId)) {
      const cb = pendingMap.get(requestId)
      pendingMap.delete(requestId)
      if (cb) {
        cb(errorResponse)
      }
    }
  }
}

/**
 * Forward a highlight message from background to inject.js
 */
export function forwardHighlightMessage(message: {
  params: { selector: string; duration_ms?: number }
}): Promise<HighlightResponse> {
  const requestId = registerHighlightRequest((result) => deferred.resolve(result))
  const deferred = createDeferredPromise<HighlightResponse>()

  // Post message to page context (inject.js)
  window.postMessage(
    {
      type: 'GASOLINE_HIGHLIGHT_REQUEST',
      requestId,
      params: message.params,
    } satisfies HighlightRequestMessage,
    window.location.origin,
  )

  // Timeout fallback + cleanup stale entries after 30 seconds
  return promiseRaceWithCleanup(deferred.promise, 30000, { success: false, error: 'timeout' }, () => {
    if (hasHighlightRequest(requestId)) {
      deleteHighlightRequest(requestId)
    }
  })
}

/**
 * Handle state capture/restore commands
 */
export async function handleStateCommand(
  params:
    | {
        action?: StateAction
        name?: string
        state?: BrowserStateSnapshot
        include_url?: boolean
      }
    | undefined,
): Promise<{ error?: string; [key: string]: unknown }> {
  const { action, name, state, include_url } = params || {}

  // Create a promise to receive response from inject.js
  const messageId = `state_${Date.now()}_${Math.random().toString(36).slice(2)}`
  const deferred = createDeferredPromise<{ error?: string; [key: string]: unknown }>()

  // Set up listener for response from inject.js
  const responseHandler = (
    event: MessageEvent<{ type?: string; messageId?: string; result?: { error?: string; [key: string]: unknown } }>,
  ) => {
    if (event.source !== window) return
    if (event.data?.type === 'GASOLINE_STATE_RESPONSE' && event.data?.messageId === messageId) {
      window.removeEventListener('message', responseHandler)
      deferred.resolve(event.data.result || { error: 'No result from state command' })
    }
  }
  window.addEventListener('message', responseHandler)

  // Send command to inject.js (include state for restore action)
  window.postMessage(
    {
      type: 'GASOLINE_STATE_COMMAND',
      messageId,
      action,
      name,
      state,
      include_url,
    } satisfies StateCommandMessage,
    window.location.origin,
  )

  // Timeout after 5 seconds with cleanup
  return promiseRaceWithCleanup(deferred.promise, 5000, { error: 'State command timeout' }, () =>
    window.removeEventListener('message', responseHandler),
  )
}

/**
 * Handle GASOLINE_PING message
 */
export function handlePing(sendResponse: (response: ContentPingResponse) => void): boolean {
  sendResponse({ status: 'alive', timestamp: Date.now() })
  return true
}

/**
 * Handle toggle messages
 */
export function handleToggleMessage(
  message: ContentMessage & { enabled?: boolean; mode?: WebSocketCaptureMode; url?: string },
): void {
  if (!TOGGLE_MESSAGES.has(message.type)) return

  const payload: SettingMessage = { type: 'GASOLINE_SETTING', setting: message.type }
  if (message.type === 'setWebSocketCaptureMode') {
    payload.mode = message.mode
  } else if (message.type === 'setServerUrl') {
    payload.url = message.url
  } else {
    payload.enabled = message.enabled
  }
  // SECURITY: Use explicit targetOrigin (window.location.origin) not "*"
  window.postMessage(payload, window.location.origin)
}

// ============================================
// Execute JS Handlers (MAIN world via inject script)
// Background handles world routing and fallback to chrome.scripting API.
// ============================================

type ExecuteJsResponse = { success: boolean; error?: string; message?: string; result?: unknown; stack?: string }

/**
 * Execute JS in the MAIN world via inject script, with safety timeout.
 */
function executeInMainWorld(
  params: { script?: string; timeout_ms?: number },
  sendResponse: (result: ExecuteJsResponse) => void,
): void {
  const timeoutMs = params.timeout_ms || 5000
  const requestId = registerExecuteRequest(sendResponse)

  // Safety timeout: user's timeout + 2s buffer (NOT fixed 30s)
  // If inject script responds, its own timeout handles slow scripts.
  // This only fires if inject script never responds at all.
  const safetyTimeoutMs = timeoutMs + 2000
  setTimeout(
    createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), {
      success: false,
      error: 'inject_not_responding',
      message: `Inject script did not respond within ${safetyTimeoutMs}ms. The tab may not be tracked or the inject script failed to load.`,
    }),
    safetyTimeoutMs,
  )

  window.postMessage(
    {
      type: 'GASOLINE_EXECUTE_JS',
      requestId,
      script: params.script || '',
      timeoutMs,
    } satisfies ExecuteJsRequestMessage,
    window.location.origin,
  )
}

/**
 * Handle GASOLINE_EXECUTE_JS message.
 * Always executes in MAIN world via inject script.
 * Returns inject_not_loaded error if inject script isn't available,
 * so background can fallback to chrome.scripting API.
 */
export function handleExecuteJs(
  params: { script?: string; timeout_ms?: number },
  sendResponse: (result: ExecuteJsResponse) => void,
): boolean {
  if (!isInjectScriptLoaded()) {
    sendResponse({
      success: false,
      error: 'inject_not_loaded',
      message: 'Inject script not loaded in page context. Tab may not be tracked.',
    })
    return true
  }

  executeInMainWorld(params, sendResponse)
  return true
}

/**
 * Handle GASOLINE_EXECUTE_QUERY message (async command path)
 */
export function handleExecuteQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: ExecuteJsResponse) => void,
): boolean {
  let parsedParams: { script?: string; timeout_ms?: number } = {}
  if (typeof params === 'string') {
    try {
      parsedParams = JSON.parse(params)
    } catch {
      parsedParams = {}
    }
  } else if (typeof params === 'object') {
    parsedParams = params as { script?: string; timeout_ms?: number }
  }

  return handleExecuteJs(parsedParams, sendResponse)
}

/**
 * Handle A11Y_QUERY message
 */
export function handleA11yQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: A11yAuditResult | { error: string }) => void,
): boolean {
  // Parse params if it's a string (from JSON)
  let parsedParams: Record<string, unknown> = {}
  if (typeof params === 'string') {
    try {
      parsedParams = JSON.parse(params)
    } catch {
      parsedParams = {}
    }
  } else if (typeof params === 'object') {
    parsedParams = params
  }

  const requestId = registerA11yRequest(sendResponse)

  // Timeout fallback: respond with error and cleanup after 30 seconds
  setTimeout(
    createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), {
      error: 'Accessibility audit timeout',
    }),
    30000,
  )

  // Forward to inject.js via postMessage
  window.postMessage(
    {
      type: 'GASOLINE_A11Y_QUERY',
      requestId,
      params: parsedParams,
    } satisfies A11yQueryRequestMessage,
    window.location.origin,
  )

  return true
}

/**
 * Handle DOM_QUERY message
 */
export function handleDomQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: { error?: string; matches?: unknown[] }) => void,
): boolean {
  // Parse params if it's a string (from JSON)
  let parsedParams: Record<string, unknown> = {}
  if (typeof params === 'string') {
    try {
      parsedParams = JSON.parse(params)
    } catch {
      parsedParams = {}
    }
  } else if (typeof params === 'object') {
    parsedParams = params
  }

  const requestId = registerDomRequest(sendResponse)

  // Timeout fallback: respond with error and cleanup after 30 seconds
  setTimeout(
    createRequestTimeoutCleanup(requestId, new Map([[requestId, sendResponse]]), { error: 'DOM query timeout' }),
    30000,
  )

  // Forward to inject.js via postMessage
  window.postMessage(
    {
      type: 'GASOLINE_DOM_QUERY',
      requestId,
      params: parsedParams,
    } satisfies DomQueryRequestMessage,
    window.location.origin,
  )

  return true
}

/**
 * Handle GET_NETWORK_WATERFALL message
 */
export function handleGetNetworkWaterfall(sendResponse: (result: { entries: WaterfallEntry[] }) => void): boolean {
  const requestId = Date.now()
  const deferred = createDeferredPromise<{ entries: WaterfallEntry[] }>()

  // Set up a one-time listener for the response
  const responseHandler = (event: MessageEvent<{ type?: string; entries?: WaterfallEntry[] }>) => {
    if (event.source !== window) return
    if (event.data?.type === 'GASOLINE_WATERFALL_RESPONSE') {
      window.removeEventListener('message', responseHandler)
      deferred.resolve({ entries: event.data.entries || [] })
    }
  }

  window.addEventListener('message', responseHandler)

  // Post message to page context
  window.postMessage(
    {
      type: 'GASOLINE_GET_WATERFALL',
      requestId,
    } satisfies GetWaterfallRequestMessage,
    window.location.origin,
  )

  // Timeout fallback: respond with empty array after 5 seconds
  promiseRaceWithCleanup(deferred.promise, 5000, { entries: [] }, () => {
    window.removeEventListener('message', responseHandler)
  }).then((result) => {
    sendResponse(result)
  })

  return true
}

/**
 * Handle LINK_HEALTH_QUERY message
 */
export function handleLinkHealthQuery(
  params: string | Record<string, unknown>,
  sendResponse: (result: unknown) => void,
): boolean {
  // Parse params if it's a string (from JSON)
  let parsedParams: Record<string, unknown> = {}
  if (typeof params === 'string') {
    try {
      parsedParams = JSON.parse(params)
    } catch {
      parsedParams = {}
    }
  } else if (typeof params === 'object') {
    parsedParams = params
  }

  const requestId = Date.now()
  const deferred = createDeferredPromise<unknown>()

  // Set up a one-time listener for the response
  const responseHandler = (event: MessageEvent<{ type?: string; result?: unknown }>) => {
    if (event.source !== window) return
    if (event.data?.type === 'GASOLINE_LINK_HEALTH_RESPONSE') {
      window.removeEventListener('message', responseHandler)
      deferred.resolve(event.data.result || { error: 'No result from link health check' })
    }
  }

  window.addEventListener('message', responseHandler)

  // Forward to inject.js via postMessage
  window.postMessage(
    {
      type: 'GASOLINE_LINK_HEALTH_QUERY',
      requestId,
      params: parsedParams,
    },
    window.location.origin,
  )

  // Timeout fallback: respond with error after 30 seconds
  promiseRaceWithCleanup(deferred.promise, 30000, { error: 'Link health check timeout' }, () => {
    window.removeEventListener('message', responseHandler)
  }).then((result) => {
    sendResponse(result)
  })

  return true
}
