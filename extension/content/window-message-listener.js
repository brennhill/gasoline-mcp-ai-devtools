/**
 * @fileoverview Window Message Listener Module
 * Handles window.postMessage events from inject.js
 */
import {
  resolveHighlightRequest,
  resolveExecuteRequest,
  resolveA11yRequest,
  resolveDomRequest
} from './request-tracking.js'
import { MESSAGE_MAP, safeSendMessage } from './message-forwarding.js'
import { getIsTrackedTab, getCurrentTabId } from './tab-tracking.js'
const RESPONSE_HANDLERS = {
  GASOLINE_HIGHLIGHT_RESPONSE: (id, result) => resolveHighlightRequest(id, result),
  GASOLINE_EXECUTE_JS_RESULT: (id, result) => resolveExecuteRequest(id, result),
  GASOLINE_A11Y_QUERY_RESPONSE: (id, result) => resolveA11yRequest(id, result),
  GASOLINE_DOM_QUERY_RESPONSE: (id, result) => resolveDomRequest(id, result)
}
export function initWindowMessageListener() {
  window.addEventListener('message', (event) => {
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
        })
      }
    }
  })
}
//# sourceMappingURL=window-message-listener.js.map
