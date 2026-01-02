/**
 * @fileoverview Content script that injects the capture script into pages
 * Runs in the content script context, bridges page and extension
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
  if (event.data?.type === 'DEV_CONSOLE_LOG') {
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
  }
})

// Listen for feature toggle messages from background
chrome.runtime.onMessage.addListener((message) => {
  // Forward feature toggle messages to inject.js via postMessage
  if (
    message.type === 'setNetworkWaterfallEnabled' ||
    message.type === 'setPerformanceMarksEnabled' ||
    message.type === 'setActionReplayEnabled'
  ) {
    window.postMessage(
      {
        type: 'DEV_CONSOLE_SETTING',
        setting: message.type,
        enabled: message.enabled,
      },
      '*'
    )
  }
})

// Inject when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', injectScript)
} else {
  injectScript()
}
