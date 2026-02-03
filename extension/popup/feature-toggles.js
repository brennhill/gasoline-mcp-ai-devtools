/**
 * @fileoverview Feature Toggles Module
 * Manages feature toggle configuration and initialization
 */
/**
 * Feature toggle configuration
 */
export const FEATURE_TOGGLES = [
  {
    id: 'toggle-websocket',
    storageKey: 'webSocketCaptureEnabled',
    messageType: 'setWebSocketCaptureEnabled',
    default: false,
  },
  {
    id: 'toggle-network-waterfall',
    storageKey: 'networkWaterfallEnabled',
    messageType: 'setNetworkWaterfallEnabled',
    default: false,
  },
  {
    id: 'toggle-performance-marks',
    storageKey: 'performanceMarksEnabled',
    messageType: 'setPerformanceMarksEnabled',
    default: false,
  },
  {
    id: 'toggle-action-replay',
    storageKey: 'actionReplayEnabled',
    messageType: 'setActionReplayEnabled',
    default: true,
  },
  { id: 'toggle-screenshot', storageKey: 'screenshotOnError', messageType: 'setScreenshotOnError', default: false },
  { id: 'toggle-source-maps', storageKey: 'sourceMapEnabled', messageType: 'setSourceMapEnabled', default: false },
  {
    id: 'toggle-network-body-capture',
    storageKey: 'networkBodyCaptureEnabled',
    messageType: 'setNetworkBodyCaptureEnabled',
    default: true,
  },
]
/**
 * Handle feature toggle change
 * CRITICAL ARCHITECTURE: Popup NEVER writes storage directly.
 * It ONLY sends a message to background, which is the single writer.
 * This prevents desynchronization bugs where UI state diverges from actual state.
 */
export function handleFeatureToggle(storageKey, messageType, enabled) {
  // Send message to background (DO NOT write storage directly)
  // Background will handle the write after updating its internal state
  chrome.runtime.sendMessage({ type: messageType, enabled }, (response) => {
    if (chrome.runtime.lastError) {
      console.error(`[Gasoline] Message error for ${messageType}:`, chrome.runtime.lastError.message)
    } else if (response?.success) {
      console.log(`[Gasoline] ${messageType} acknowledged by background`)
    } else {
      console.warn(`[Gasoline] ${messageType} - no response from background`)
    }
  })
}
/**
 * Initialize all feature toggles
 */
export async function initFeatureToggles() {
  // Load saved states
  const storageKeys = FEATURE_TOGGLES.map((t) => t.storageKey)
  return new Promise((resolve) => {
    chrome.storage.local.get(storageKeys, (result) => {
      for (const toggle of FEATURE_TOGGLES) {
        const checkbox = document.getElementById(toggle.id)
        if (checkbox) {
          // Use saved value or default
          const savedValue = result[toggle.storageKey]
          checkbox.checked = savedValue !== undefined ? savedValue : toggle.default
          // Set up change handler
          checkbox.addEventListener('change', () => {
            handleFeatureToggle(toggle.storageKey, toggle.messageType, checkbox.checked)
          })
        }
      }
      resolve()
    })
  })
}
//# sourceMappingURL=feature-toggles.js.map
