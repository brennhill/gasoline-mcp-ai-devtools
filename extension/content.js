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

// ============================================================================
// AI WEB PILOT: HIGHLIGHT MESSAGE FORWARDING
// ============================================================================

// Pending highlight response resolver
let pendingHighlightResolve = null

/**
 * Forward a highlight message from background to inject.js
 * @param {Object} message - The GASOLINE_HIGHLIGHT message
 * @returns {Promise<Object>} Result from inject.js
 */
function forwardHighlightMessage(message) {
  return new Promise((resolve) => {
    pendingHighlightResolve = resolve

    // Post message to page context (inject.js)
    window.postMessage(
      {
        type: 'GASOLINE_HIGHLIGHT_REQUEST',
        params: message.params,
      },
      '*',
    )

    // Timeout fallback
    setTimeout(() => {
      if (pendingHighlightResolve) {
        pendingHighlightResolve({ success: false, error: 'timeout' })
        pendingHighlightResolve = null
      }
    }, 5000)
  })
}

// Listen for highlight responses from inject.js
window.addEventListener('message', (event) => {
  if (event.source !== window) return
  if (event.data?.type === 'GASOLINE_HIGHLIGHT_RESPONSE' && pendingHighlightResolve) {
    pendingHighlightResolve(event.data.result)
    pendingHighlightResolve = null
  }
})

// ============================================================================
// AI WEB PILOT: EXECUTE JS REQUEST TRACKING
// ============================================================================

// Pending execute requests waiting for responses from inject.js
const pendingExecuteRequests = new Map()
let executeRequestId = 0

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

// ============================================================================
// MESSAGE HANDLERS FROM BACKGROUND
// ============================================================================

// Listen for messages from background (feature toggles and pilot commands)
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  // Handle ping to check if content script is loaded
  if (message.type === 'GASOLINE_PING') {
    sendResponse({ status: 'alive', timestamp: Date.now() })
    return true
  }

  if (TOGGLE_MESSAGES.has(message.type)) {
    const payload = { type: 'GASOLINE_SETTING', setting: message.type }
    if (message.type === 'setWebSocketCaptureMode') {
      payload.mode = message.mode
    } else {
      payload.enabled = message.enabled
    }
    window.postMessage(payload, '*')
  }

  // Handle GASOLINE_HIGHLIGHT from background
  if (message.type === 'GASOLINE_HIGHLIGHT') {
    forwardHighlightMessage(message).then((result) => {
      sendResponse(result)
    })
    return true // Will respond asynchronously
  }

  // Handle state management commands from background
  if (message.type === 'GASOLINE_MANAGE_STATE') {
    handleStateCommand(message.params)
      .then((result) => sendResponse(result))
      .catch((err) => sendResponse({ error: err.message }))
    return true // Keep channel open for async response
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

// Handle state capture/restore commands
async function handleStateCommand(params) {
  const { action, name, state, include_url } = params || {}

  // Create a promise to receive response from inject.js
  return new Promise((resolve) => {
    const messageId = `state_${Date.now()}_${Math.random().toString(36).slice(2)}`

    // Set up listener for response from inject.js
    const responseHandler = (event) => {
      if (event.source !== window) return
      if (event.data?.type === 'GASOLINE_STATE_RESPONSE' && event.data?.messageId === messageId) {
        window.removeEventListener('message', responseHandler)
        resolve(event.data.result)
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
      },
      '*',
    )

    // Timeout after 5 seconds
    setTimeout(() => {
      window.removeEventListener('message', responseHandler)
      resolve({ error: 'State command timeout' })
    }, 5000)
  })
}

// Inject when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', injectScript)
} else {
  injectScript()
}
