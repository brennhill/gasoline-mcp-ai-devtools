// @ts-nocheck
/**
 * @fileoverview content.js — Message bridge between page and extension contexts.
 * Injects inject.js into the page as a module script, then listens for
 * window.postMessage events (GASOLINE_LOG, GASOLINE_WS, GASOLINE_NETWORK_BODY,
 * GASOLINE_ENHANCED_ACTION, GASOLINE_PERF_SNAPSHOT) and forwards them to the
 * background service worker via chrome.runtime.sendMessage.
 * Also handles chrome.runtime messages for on-demand queries (DOM, a11y, perf).
 * Design: Tab-scoped filtering — only forwards messages from the explicitly
 * tracked tab. Validates message origin (event.source === window) to prevent
 * cross-frame injection. Attaches tabId to all forwarded messages.
 */

// ============================================================================
// TAB TRACKING STATE
// ============================================================================

// Whether this content script's tab is the currently tracked tab
let isTrackedTab = false
// The tab ID of this content script's tab
let currentTabId = null

/**
 * Update tracking status by checking storage and current tab ID.
 * Called on script load, storage changes, and tab activation.
 */
async function updateTrackingStatus() {
  try {
    const storage = await chrome.storage.local.get(['trackedTabId'])

    // Request tab ID from background script (content scripts can't access chrome.tabs)
    const response = await chrome.runtime.sendMessage({ type: 'GET_TAB_ID' })
    currentTabId = response?.tabId

    isTrackedTab = (currentTabId !== null && currentTabId !== undefined && currentTabId === storage.trackedTabId)
  } catch {
    // Graceful degradation: if we can't check, assume not tracked
    isTrackedTab = false
  }
}

// Initialize tracking status on script load
updateTrackingStatus()

// Listen for tracking changes in storage
chrome.storage.onChanged.addListener((changes) => {
  if (changes.trackedTabId) {
    updateTrackingStatus()
  }
})

// Note: chrome.tabs is NOT available in content scripts.
// Tab activation re-checks happen via storage change events:
// when the popup tracks a new tab, it writes trackedTabId to storage,
// which triggers the storage.onChanged listener above.

// ============================================================================
// SCRIPT INJECTION
// ============================================================================

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

// Consolidated message listener for all injected script messages
window.addEventListener('message', (event) => {
  // Only accept messages from this window
  if (event.source !== window) return

  const { type: messageType, requestId, result, payload } = event.data || {}

  // Handle highlight responses
  if (messageType === 'GASOLINE_HIGHLIGHT_RESPONSE') {
    const resolve = pendingHighlightRequests.get(requestId)
    if (resolve) {
      pendingHighlightRequests.delete(requestId)
      resolve(result)
    }
    return
  }

  // Handle execute JS results
  if (messageType === 'GASOLINE_EXECUTE_JS_RESULT') {
    const sendResponse = pendingExecuteRequests.get(requestId)
    if (sendResponse) {
      pendingExecuteRequests.delete(requestId)
      sendResponse(result)
    }
    return
  }

  // Handle a11y audit results from inject.js
  if (messageType === 'GASOLINE_A11Y_QUERY_RESPONSE') {
    const sendResponse = pendingA11yRequests.get(requestId)
    if (sendResponse) {
      pendingA11yRequests.delete(requestId)
      sendResponse(result)
    }
    return
  }

  // Handle DOM query results from inject.js
  if (messageType === 'GASOLINE_DOM_QUERY_RESPONSE') {
    const sendResponse = pendingDomRequests.get(requestId)
    if (sendResponse) {
      pendingDomRequests.delete(requestId)
      sendResponse(result)
    }
    return
  }

  // Tab isolation filter: only forward captured data from the tracked tab.
  // Response messages (highlight, execute JS, a11y) are NOT filtered because
  // they are responses to explicit commands from the background script.
  if (!isTrackedTab) {
    return // Drop captured data from untracked tabs
  }

  // Handle MESSAGE_MAP forwarding — attach tabId to every message
  // eslint-disable-next-line security/detect-object-injection -- messageType is from trusted inject.js, MESSAGE_MAP is module constant
  const mapped = MESSAGE_MAP[messageType]
  if (mapped && payload && typeof payload === 'object') {
    safeSendMessage({ type: mapped, payload, tabId: currentTabId })
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
  'setServerUrl',
])

// ============================================================================
// AI WEB PILOT: HIGHLIGHT MESSAGE FORWARDING
// ============================================================================

// Pending highlight response resolvers (keyed by request ID)
const pendingHighlightRequests = new Map()
let highlightRequestId = 0

/**
 * Forward a highlight message from background to inject.js
 * @param {Object} message - The GASOLINE_HIGHLIGHT message
 * @returns {Promise<Object>} Result from inject.js
 */
function forwardHighlightMessage(message) {
  return new Promise((resolve) => {
    const requestId = ++highlightRequestId
    pendingHighlightRequests.set(requestId, resolve)

    // Post message to page context (inject.js)
    window.postMessage(
      {
        type: 'GASOLINE_HIGHLIGHT_REQUEST',
        requestId,
        params: message.params,
      },
      window.location.origin,
    )

    // Timeout fallback + cleanup stale entries after 30 seconds
    // Guarded against double-resolution: Both this timeout and the response handler check
    // has() before get(). JavaScript is single-threaded, so only the first to run will
    // delete the entry; the second's get() returns undefined, preventing double-callback.
    setTimeout(() => {
      if (pendingHighlightRequests.has(requestId)) {
        const callback = pendingHighlightRequests.get(requestId)
        if (callback) {
          pendingHighlightRequests.delete(requestId)
          callback({ success: false, error: 'timeout' })
        }
      }
    }, 30000)
  })
}

// ============================================================================
// AI WEB PILOT: EXECUTE JS REQUEST TRACKING
// ============================================================================

// Pending execute requests waiting for responses from inject.js
const pendingExecuteRequests = new Map()
let executeRequestId = 0

// Pending a11y audit requests waiting for responses from inject.js
const pendingA11yRequests = new Map()
let a11yRequestId = 0

// Pending DOM query requests waiting for responses from inject.js
const pendingDomRequests = new Map()
let domRequestId = 0

// ============================================================================
// ISSUE 2 FIX: PENDING REQUEST CLEANUP ON PAGE UNLOAD
// ============================================================================

/**
 * Clear all pending request Maps on page unload (Issue 2 fix).
 * Prevents memory leaks and stale request accumulation across navigations.
 */
export function clearPendingRequests() {
  pendingHighlightRequests.clear()
  pendingExecuteRequests.clear()
  pendingA11yRequests.clear()
  pendingDomRequests.clear()
}

/**
 * Get statistics about pending requests (for testing/debugging)
 * @returns {Object} Counts of pending requests by type
 */
export function getPendingRequestStats() {
  return {
    highlight: pendingHighlightRequests.size,
    execute: pendingExecuteRequests.size,
    a11y: pendingA11yRequests.size,
    dom: pendingDomRequests.size,
  }
}

// Register cleanup handlers for page unload/navigation (Issue 2 fix)
// Using 'pagehide' (modern, fires on both close and navigation) + 'beforeunload' (legacy fallback)
window.addEventListener('pagehide', clearPendingRequests)
window.addEventListener('beforeunload', clearPendingRequests)

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
    } else if (message.type === 'setServerUrl') {
      payload.url = message.url
    } else {
      payload.enabled = message.enabled
    }
    window.postMessage(payload, window.location.origin)
  }

  // Handle GASOLINE_HIGHLIGHT from background
  if (message.type === 'GASOLINE_HIGHLIGHT') {
    forwardHighlightMessage(message)
      .then((result) => {
        sendResponse(result)
      })
      .catch((err) => {
        sendResponse({ success: false, error: err.message })
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

    // Timeout fallback: respond with error and cleanup after 30 seconds
    setTimeout(() => {
      if (pendingExecuteRequests.has(requestId)) {
        const cb = pendingExecuteRequests.get(requestId)
        pendingExecuteRequests.delete(requestId)
        cb({ success: false, error: 'timeout', message: 'Execute request timed out after 30s' })
      }
    }, 30000)

    // Forward to inject.js via postMessage
    window.postMessage(
      {
        type: 'GASOLINE_EXECUTE_JS',
        requestId,
        script: params.script,
        timeoutMs: params.timeout_ms || 5000,
      },
      window.location.origin,
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

    // Timeout fallback: respond with error and cleanup after 30 seconds
    setTimeout(() => {
      if (pendingExecuteRequests.has(requestId)) {
        const cb = pendingExecuteRequests.get(requestId)
        pendingExecuteRequests.delete(requestId)
        cb({ success: false, error: 'timeout', message: 'Execute query timed out after 30s' })
      }
    }, 30000)

    // Forward to inject.js via postMessage
    window.postMessage(
      {
        type: 'GASOLINE_EXECUTE_JS',
        requestId,
        script: parsedParams.script,
        timeoutMs: parsedParams.timeout_ms || 5000,
      },
      window.location.origin,
    )

    // Return true to indicate we'll respond asynchronously
    return true
  }

  // Handle A11Y_QUERY from background (run accessibility audit in page context)
  if (message.type === 'A11Y_QUERY') {
    const requestId = ++a11yRequestId
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
    pendingA11yRequests.set(requestId, sendResponse)

    // Timeout fallback: respond with error and cleanup after 30 seconds (a11y audits take longer)
    setTimeout(() => {
      if (pendingA11yRequests.has(requestId)) {
        const cb = pendingA11yRequests.get(requestId)
        pendingA11yRequests.delete(requestId)
        cb({ error: 'Accessibility audit timeout' })
      }
    }, 30000)

    // Forward to inject.js via postMessage
    window.postMessage(
      {
        type: 'GASOLINE_A11Y_QUERY',
        requestId,
        params: parsedParams,
      },
      window.location.origin,
    )

    return true // Will respond asynchronously
  }

  // Handle DOM_QUERY from background (execute CSS selector query in page context)
  if (message.type === 'DOM_QUERY') {
    const requestId = ++domRequestId
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
    pendingDomRequests.set(requestId, sendResponse)

    // Timeout fallback: respond with error and cleanup after 30 seconds
    setTimeout(() => {
      if (pendingDomRequests.has(requestId)) {
        const cb = pendingDomRequests.get(requestId)
        pendingDomRequests.delete(requestId)
        cb({ error: 'DOM query timeout' })
      }
    }, 30000)

    // Forward to inject.js via postMessage
    window.postMessage(
      {
        type: 'GASOLINE_DOM_QUERY',
        requestId,
        params: parsedParams,
      },
      window.location.origin,
    )

    return true // Will respond asynchronously
  }

  // Handle GET_NETWORK_WATERFALL from background (collect PerformanceResourceTiming data)
  if (message.type === 'GET_NETWORK_WATERFALL') {
    // Query the injected gasoline API for waterfall data
    window.postMessage(
      {
        type: 'GASOLINE_GET_WATERFALL',
        requestId: Date.now(),
      },
      window.location.origin,
    )

    // Set up a one-time listener for the response
    const responseHandler = (event) => {
      if (event.source !== window) return
      if (event.data?.type === 'GASOLINE_WATERFALL_RESPONSE') {
        window.removeEventListener('message', responseHandler)
        sendResponse({ entries: event.data.entries || [] })
      }
    }

    window.addEventListener('message', responseHandler)

    // Timeout fallback: respond with empty array after 5 seconds
    setTimeout(() => {
      window.removeEventListener('message', responseHandler)
      sendResponse({ entries: [] })
    }, 5000)

    return true // Will respond asynchronously
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
      window.location.origin,
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
  document.addEventListener('DOMContentLoaded', injectScript, { once: true })
} else {
  injectScript()
}
