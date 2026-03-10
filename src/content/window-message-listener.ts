/**
 * Purpose: Listens for window.postMessage events from inject.js, resolves pending request promises, and forwards telemetry to the background via chrome.runtime.sendMessage.
 * Why: Consolidates message forwarding and message listening into one module since they share the same data flow.
 * Docs: docs/features/feature/observe/index.md
 */

import type { HighlightResponse, ExecuteJsResult, A11yAuditResult, DomQueryResult } from '../types/index.js'
import type { PageMessageEventData, BackgroundMessageFromContent } from './types.js'
import {
  resolveHighlightRequest,
  resolveExecuteRequest,
  resolveA11yRequest,
  resolveDomRequest
} from './request-tracking.js'
import { getIsTrackedTab, getCurrentTabId } from './tab-tracking.js'
import { getPageNonce } from './script-injection.js'

// =============================================================================
// MESSAGE FORWARDING — page postMessage → background chrome.runtime.sendMessage
// =============================================================================

/** Dispatch table: page postMessage type -> background message type */
export const MESSAGE_MAP: Record<string, string> = {
  gasoline_log: 'log',
  gasoline_ws: 'ws_event',
  gasoline_network_body: 'network_body',
  gasoline_enhanced_action: 'enhanced_action',
  gasoline_performance_snapshot: 'performance_snapshot'
} as const

// Track whether the extension context is still valid
let contextValid = true

/**
 * Safely send a message to the background script.
 * Handles extension context invalidation gracefully.
 */
export function safeSendMessage(msg: BackgroundMessageFromContent): void {
  if (!contextValid) return
  try {
    chrome.runtime.sendMessage(msg)
  } catch (e) {
    if (e instanceof Error && e.message?.includes('Extension context invalidated')) {
      contextValid = false
      console.warn(
        '[Gasoline] Please refresh this page. The Gasoline extension was reloaded ' +
          'and this page still has the old content script. A page refresh will ' +
          'reconnect capture automatically.'
      )
    }
  }
}

/**
 * Check if the extension context is still valid
 */
export function isContextValid(): boolean {
  return contextValid
}

// =============================================================================
// WINDOW MESSAGE LISTENER — dispatches incoming postMessage events
// =============================================================================

type ResponseResolver = (requestId: number | string, result: unknown) => void

const RESPONSE_HANDLERS: Record<string, ResponseResolver> = {
  gasoline_highlight_response: (id, result) => resolveHighlightRequest(id as number, result as HighlightResponse),
  gasoline_execute_js_result: (id, result) => resolveExecuteRequest(id as number, result as ExecuteJsResult),
  gasoline_a11y_query_response: (id, result) => resolveA11yRequest(id as number, result as A11yAuditResult),
  gasoline_dom_query_response: (id, result) => resolveDomRequest(id as number, result as DomQueryResult)
}

export function initWindowMessageListener(): void {
  window.addEventListener('message', (event: MessageEvent<PageMessageEventData>) => {
    if (event.source !== window || event.origin !== window.location.origin) return

    const { type: messageType, requestId, result, payload } = event.data || {}

    const responseHandler = messageType ? RESPONSE_HANDLERS[messageType] : undefined
    if (responseHandler) {
      // Validate nonce on response messages (spoofing prevention).
      // Accept responses with no nonce for backwards compat during migration.
      const nonce = (event.data as { _nonce?: string })?._nonce
      if (nonce && nonce !== getPageNonce()) return
      if (requestId !== undefined) responseHandler(requestId, result)
      return
    }

    // Tab isolation filter: only forward captured data from the tracked tab.
    // Response messages (highlight, execute JS, a11y) are NOT filtered because
    // they are responses to explicit commands from the background script.
    if (!getIsTrackedTab()) return

    if (messageType && messageType in MESSAGE_MAP && payload && typeof payload === 'object') {
      const mappedType = MESSAGE_MAP[messageType]
      if (mappedType) {
        safeSendMessage({
          type: mappedType,
          payload,
          tabId: getCurrentTabId()
        } as BackgroundMessageFromContent)
      }
    }
  })
}
