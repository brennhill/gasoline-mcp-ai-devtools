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

// Listen for messages from the injected script
window.addEventListener('message', (event) => {
  // Only accept messages from this window
  if (event.source !== window) return

  // Only handle our messages
  if (event.data?.type === 'GASOLINE_LOG') {
    // Forward to background service worker
    chrome.runtime.sendMessage({
      type: 'log',
      payload: event.data.payload,
    })
  } else if (event.data?.type === 'GASOLINE_WS') {
    // Forward WebSocket events to background service worker
    chrome.runtime.sendMessage({
      type: 'ws_event',
      payload: event.data.payload,
    })
  } else if (event.data?.type === 'GASOLINE_NETWORK_BODY') {
    // Forward network body captures to background service worker
    chrome.runtime.sendMessage({
      type: 'network_body',
      payload: event.data.payload,
    })
  } else if (event.data?.type === 'GASOLINE_ENHANCED_ACTION') {
    // Forward enhanced action events to background service worker
    chrome.runtime.sendMessage({
      type: 'enhanced_action',
      payload: event.data.payload,
    })
  } else if (event.data?.type === 'GASOLINE_PERFORMANCE_SNAPSHOT') {
    // Forward performance snapshot to background service worker
    chrome.runtime.sendMessage({
      type: 'performance_snapshot',
      payload: event.data.payload,
    })
  }
})

// Listen for feature toggle messages from background
chrome.runtime.onMessage.addListener((message) => {
  // Forward feature toggle messages to inject.js via postMessage
  if (
    message.type === 'setNetworkWaterfallEnabled' ||
    message.type === 'setPerformanceMarksEnabled' ||
    message.type === 'setActionReplayEnabled' ||
    message.type === 'setWebSocketCaptureEnabled' ||
    message.type === 'setWebSocketCaptureMode' ||
    message.type === 'setPerformanceSnapshotEnabled' ||
    message.type === 'setDeferralEnabled' ||
    message.type === 'setNetworkBodyCaptureEnabled'
  ) {
    const payload = { type: 'GASOLINE_SETTING', setting: message.type }
    if (message.type === 'setWebSocketCaptureMode') {
      payload.mode = message.mode
    } else {
      payload.enabled = message.enabled
    }
    window.postMessage(payload, '*')
  }
})

// Inject when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', injectScript)
} else {
  injectScript()
}
