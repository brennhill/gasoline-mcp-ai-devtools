/**
 * @fileoverview popup.ts - Extension popup UI showing connection status and controls.
 * Displays server connection state, entry count, error count, log level selector,
 * and log file path. Polls the background worker for status updates and provides
 * a clear-logs button. Shows troubleshooting hints when disconnected.
 * Design: Pure DOM manipulation, no framework. Communicates with background.js
 * via chrome.runtime.sendMessage for status queries and log-level changes.
 */

import type { WebSocketCaptureMode } from './types'
import type { PopupConnectionStatus, ToggleWarningConfig } from './popup/types'
import { updateConnectionStatus } from './popup/status-display'
import { initFeatureToggles } from './popup/feature-toggles'
import { initTrackPageButton } from './popup/tab-tracking'
import { initAiWebPilotToggle } from './popup/ai-web-pilot'
import {
  initWebSocketModeSelector,
  handleWebSocketModeChange,
  handleClearLogs,
  resetClearConfirm,
} from './popup/settings'

// Re-export for testing
export { resetClearConfirm, handleClearLogs }
export { updateConnectionStatus }
export { FEATURE_TOGGLES, initFeatureToggles } from './popup/feature-toggles'
export { handleFeatureToggle } from './popup/feature-toggles'
export { initAiWebPilotToggle, handleAiWebPilotToggle } from './popup/ai-web-pilot'
export { initTrackPageButton, handleTrackPageClick } from './popup/tab-tracking'
export { handleWebSocketModeChange } from './popup/settings'
export { initWebSocketModeSelector } from './popup/settings'
export { isInternalUrl } from './popup/ui-utils'

const DEFAULT_MAX_ENTRIES = 1000

/**
 * Initialize the popup
 */
export async function initPopup(): Promise<void> {
  // Request current status from background - may fail if service worker is inactive
  try {
    chrome.runtime.sendMessage({ type: 'getStatus' }, (status: PopupConnectionStatus | undefined) => {
      if (chrome.runtime.lastError) {
        // Background service worker may be inactive or restarting
        updateConnectionStatus({
          connected: false,
          entries: 0,
          maxEntries: DEFAULT_MAX_ENTRIES,
          errorCount: 0,
          logFile: '',
          error: 'Extension restarting - please wait a moment and reopen popup',
        })
        return
      }
      if (status) {
        updateConnectionStatus(status)
      }
    })
  } catch {
    // Extension context invalidated or other critical error
    updateConnectionStatus({
      connected: false,
      entries: 0,
      maxEntries: DEFAULT_MAX_ENTRIES,
      errorCount: 0,
      logFile: '',
      error: 'Extension error - try reloading the extension',
    })
  }

  // Initialize feature toggles
  await initFeatureToggles()

  // Initialize WebSocket mode selector
  await initWebSocketModeSelector()

  // Initialize AI Web Pilot toggle
  await initAiWebPilotToggle()

  // Initialize Track This Page button
  await initTrackPageButton()

  // Show/hide WebSocket mode selector based on toggle
  const wsToggle = document.getElementById('toggle-websocket') as HTMLInputElement | null
  const wsModeContainer = document.getElementById('ws-mode-container')
  if (wsToggle && wsModeContainer) {
    wsModeContainer.style.display = wsToggle.checked ? 'block' : 'none'
    wsToggle.addEventListener('change', () => {
      wsModeContainer.style.display = wsToggle.checked ? 'block' : 'none'
    })
  }

  // Set up WebSocket mode change handler
  const wsModeSelect = document.getElementById('ws-mode') as HTMLSelectElement | null
  if (wsModeSelect) {
    wsModeSelect.addEventListener('change', (e: Event) => {
      const target = e.target as HTMLSelectElement
      handleWebSocketModeChange(target.value as WebSocketCaptureMode)
    })
  }

  // Show/hide WebSocket messages warning based on mode
  const wsMessagesWarning = document.getElementById('ws-messages-warning')
  if (wsModeSelect && wsMessagesWarning) {
    wsMessagesWarning.style.display = wsModeSelect.value === 'all' ? 'block' : 'none'
    wsModeSelect.addEventListener('change', () => {
      wsMessagesWarning.style.display = wsModeSelect.value === 'all' ? 'block' : 'none'
    })
  }

  // Show/hide toggle warnings when features are enabled
  const toggleWarnings: ToggleWarningConfig[] = [
    { toggleId: 'toggle-screenshot', warningId: 'screenshot-warning' },
    { toggleId: 'toggle-network-waterfall', warningId: 'waterfall-warning' },
    { toggleId: 'toggle-performance-marks', warningId: 'perfmarks-warning' },
  ]
  for (const { toggleId, warningId } of toggleWarnings) {
    const toggle = document.getElementById(toggleId) as HTMLInputElement | null
    const warning = document.getElementById(warningId)
    if (toggle && warning) {
      warning.style.display = toggle.checked ? 'block' : 'none'
      toggle.addEventListener('change', () => {
        warning.style.display = toggle.checked ? 'block' : 'none'
      })
    }
  }

  // Set up clear button handler
  const clearBtn = document.getElementById('clear-btn')
  if (clearBtn) {
    clearBtn.addEventListener('click', handleClearLogs)
  }

  // Listen for status updates
  chrome.runtime.onMessage.addListener(
    (message: { type: string; status?: PopupConnectionStatus; enabled?: boolean }) => {
      if (message.type === 'statusUpdate' && message.status) {
        updateConnectionStatus(message.status)
      } else if (message.type === 'pilotStatusChanged') {
        // Update toggle to reflect confirmed state from background
        const toggle = document.getElementById('aiWebPilotEnabled') as HTMLInputElement | null
        if (toggle) {
          toggle.checked = message.enabled === true
          console.log('[Gasoline] Pilot status confirmed:', message.enabled)
        }
      }
    },
  )

  // Listen for storage changes (e.g., tracked tab URL updates)
  chrome.storage.onChanged.addListener((changes, areaName) => {
    if (areaName === 'local' && changes.trackedTabUrl) {
      const urlEl = document.getElementById('tracked-url')
      if (urlEl && changes.trackedTabUrl.newValue) {
        urlEl.textContent = changes.trackedTabUrl.newValue as string
        console.log('[Gasoline] Tracked tab URL updated in popup:', changes.trackedTabUrl.newValue)
      }
    }
  })
}

// Initialize when DOM is ready
if (typeof document !== 'undefined' && typeof (globalThis as Record<string, unknown>).process === 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initPopup)
  } else {
    initPopup()
  }
}
