/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */

/**
 * @fileoverview Window Message Listener Module
 * Handles window.postMessage events from inject.js
 */

import type { HighlightResponse, ExecuteJsResult, A11yAuditResult, DomQueryResult } from '../types'
import type { PageMessageEventData, BackgroundMessageFromContent } from './types'
import {
  resolveHighlightRequest,
  resolveExecuteRequest,
  resolveA11yRequest,
  resolveDomRequest
} from './request-tracking'
import { MESSAGE_MAP, safeSendMessage } from './message-forwarding'
import { getIsTrackedTab, getCurrentTabId } from './tab-tracking'

/**
 * Initialize consolidated window message listener
 * Handles all messages from inject.js
 */
type ResponseResolver = (requestId: number | string, result: unknown) => void

const RESPONSE_HANDLERS: Record<string, ResponseResolver> = {
  GASOLINE_HIGHLIGHT_RESPONSE: (id, result) => resolveHighlightRequest(id as number, result as HighlightResponse),
  GASOLINE_EXECUTE_JS_RESULT: (id, result) => resolveExecuteRequest(id as number, result as ExecuteJsResult),
  GASOLINE_A11Y_QUERY_RESPONSE: (id, result) => resolveA11yRequest(id as number, result as A11yAuditResult),
  GASOLINE_DOM_QUERY_RESPONSE: (id, result) => resolveDomRequest(id as number, result as DomQueryResult)
}

export function initWindowMessageListener(): void {
  window.addEventListener('message', (event: MessageEvent<PageMessageEventData>) => {
    if (event.source !== window || event.origin !== window.location.origin) return

    const { type: messageType, requestId, result, payload } = event.data || {}

    const responseHandler = messageType ? RESPONSE_HANDLERS[messageType] : undefined
    if (responseHandler) {
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
