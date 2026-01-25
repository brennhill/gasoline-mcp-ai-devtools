// @ts-nocheck
/**
 * @fileoverview content.js — Message bridge between page and extension contexts.
 * Injects inject.js into the page as a module script, then listens for
 * window.postMessage events (GASOLINE_LOG, GASOLINE_WS, GASOLINE_NETWORK_BODY,
 * GASOLINE_ENHANCED_ACTION, GASOLINE_PERF_SNAPSHOT) and forwards them to the
 * background service worker via chrome.runtime.sendMessage.
 * Also handles chrome.runtime messages for on-demand queries (DOM, a11y, perf).
 * Design: Minimal footprint — no state, just message routing. Validates message
 * origin (event.source === window) to prevent cross-frame injection.
 */

// Inject the capture script into the page
function injectScript() {
  const script = document.createElement('script')
  script.src = chrome.runtime.getURL('inject.js')
  script.type = 'module'
  script.onload = () => script.remove()
  ;(document.head || document.documentElement).appendChild(script)
}

// Dispatch table: page postMessage type -> background message type
const MESSAGE_MAP = {
  GASOLINE_LOG: 'log',
  GASOLINE_WS: 'ws_event',
  GASOLINE_NETWORK_BODY: 'network_body',
  GASOLINE_ENHANCED_ACTION: 'enhanced_action',
  GASOLINE_PERFORMANCE_SNAPSHOT: 'performance_snapshot',
}

// Track whether the extension context is still valid
let contextValid = true

function safeSendMessage(msg) {
  if (!contextValid) return
  try {
    chrome.runtime.sendMessage(msg)
  } catch (e) {
    if (e.message?.includes('Extension context invalidated')) {
      contextValid = false
      console.warn(
        '[Gasoline] Please refresh this page. The Gasoline extension was reloaded ' +
          'and this page still has the old content script. A page refresh will ' +
          'reconnect capture automatically.',
      )
    }
  }
}

// Listen for messages from the injected script
window.addEventListener('message', (event) => {
  // Only accept messages from this window
  if (event.source !== window) return

  const mapped = MESSAGE_MAP[event.data?.type]
  if (mapped && event.data.payload && typeof event.data.payload === 'object') {
    safeSendMessage({ type: mapped, payload: event.data.payload })
  }
})

// Feature toggle message types forwarded from background to inject.js
const TOGGLE_MESSAGES = new Set([
  'setNetworkWaterfallEnabled',
  'setPerformanceMarksEnabled',
  'setActionReplayEnabled',
  'setWebSocketCaptureEnabled',
  'setWebSocketCaptureMode',
  'setPerformanceSnapshotEnabled',
  'setDeferralEnabled',
  'setNetworkBodyCaptureEnabled',
])

// Pending execute requests waiting for responses from inject.js
const pendingExecuteRequests = new Map()
let executeRequestId = 0

// Listen for messages from background (feature toggles and pilot commands)
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  if (TOGGLE_MESSAGES.has(message.type)) {
    const payload = { type: 'GASOLINE_SETTING', setting: message.type }
    if (message.type === 'setWebSocketCaptureMode') {
      payload.mode = message.mode
    } else {
      payload.enabled = message.enabled
    }
    window.postMessage(payload, '*')
  }

  // Handle GASOLINE_EXECUTE_JS from background (direct pilot command)
  if (message.type === 'GASOLINE_EXECUTE_JS') {
    const requestId = ++executeRequestId
    const params = message.params || {}

    // Store the sendResponse callback for when we get the result
    pendingExecuteRequests.set(requestId, sendResponse)

    // Forward to inject.js via postMessage
    window.postMessage(
      {
        type: 'GASOLINE_EXECUTE_JS',
        requestId,
        script: params.script,
        timeoutMs: params.timeout_ms || 5000,
      },
      '*',
    )

    // Return true to indicate we'll respond asynchronously
    return true
  }

  // Handle GASOLINE_EXECUTE_QUERY from background (polling system)
  if (message.type === 'GASOLINE_EXECUTE_QUERY') {
    const requestId = ++executeRequestId
    const params = message.params || {}

    // Parse params if it's a string (from JSON)
    let parsedParams = params
    if (typeof params === 'string') {
      try {
        parsedParams = JSON.parse(params)
      } catch {
        parsedParams = {}
      }
    }

    // Store the sendResponse callback for when we get the result
    pendingExecuteRequests.set(requestId, sendResponse)

    // Forward to inject.js via postMessage
    window.postMessage(
      {
        type: 'GASOLINE_EXECUTE_JS',
        requestId,
        script: parsedParams.script,
        timeoutMs: parsedParams.timeout_ms || 5000,
      },
      '*',
    )

    // Return true to indicate we'll respond asynchronously
    return true
  }
})

// Listen for execute results from inject.js
window.addEventListener('message', (event) => {
  if (event.source !== window) return

  if (event.data?.type === 'GASOLINE_EXECUTE_JS_RESULT') {
    const { requestId, result } = event.data
    const sendResponse = pendingExecuteRequests.get(requestId)
    if (sendResponse) {
      pendingExecuteRequests.delete(requestId)
      sendResponse(result)
    }
  }
})

// Inject when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', injectScript)
} else {
  injectScript()
}
