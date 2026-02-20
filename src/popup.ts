/**
 * Purpose: Owns popup.ts runtime behavior and integration logic.
 * Docs: docs/features/feature/observe/index.md
 */

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
import { StorageKey } from './lib/constants'
import { updateConnectionStatus } from './popup/status-display'
import { setupRecordingUI } from './popup/recording'
import { setupDrawModeButton } from './popup/draw-mode'
import { initFeatureToggles } from './popup/feature-toggles'
import { initTrackPageButton } from './popup/tab-tracking'
import { initAiWebPilotToggle } from './popup/ai-web-pilot'
import {
  initWebSocketModeSelector,
  handleWebSocketModeChange,
  handleClearLogs,
  resetClearConfirm
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
 * Bind a toggle element to show/hide a target element based on a condition.
 * Sets initial display state and adds a change listener.
 */
function bindToggleVisibility(
  toggle: HTMLInputElement | HTMLSelectElement,
  target: HTMLElement,
  isVisible: () => boolean
): void {
  target.style.display = isVisible() ? 'block' : 'none'
  toggle.addEventListener('change', () => {
    target.style.display = isVisible() ? 'block' : 'none'
  })
}

// #lizard forgives
function setupWebSocketUI(): void {
  const wsToggle = document.getElementById('toggle-websocket') as HTMLInputElement | null
  const wsModeContainer = document.getElementById('ws-mode-container')
  if (wsToggle && wsModeContainer) {
    bindToggleVisibility(wsToggle, wsModeContainer, () => wsToggle.checked)
  }

  const wsModeSelect = document.getElementById('ws-mode') as HTMLSelectElement | null
  if (wsModeSelect) {
    wsModeSelect.addEventListener('change', (e: Event) => {
      handleWebSocketModeChange((e.target as HTMLSelectElement).value as WebSocketCaptureMode)
    })
  }

  const wsMessagesWarning = document.getElementById('ws-messages-warning')
  if (wsModeSelect && wsMessagesWarning) {
    bindToggleVisibility(wsModeSelect, wsMessagesWarning, () => wsModeSelect.value === 'all')
  }
}

function setupToggleWarnings(): void {
  const toggleWarnings: ToggleWarningConfig[] = [
    { toggleId: 'toggle-screenshot', warningId: 'screenshot-warning' },
    { toggleId: 'toggle-network-waterfall', warningId: 'waterfall-warning' },
    { toggleId: 'toggle-performance-marks', warningId: 'perfmarks-warning' }
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
}

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
          error: 'Extension restarting - please wait a moment and reopen popup'
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
      error: 'Extension error - try reloading the extension'
    })
  }

  // Initialize recording UI
  setupRecordingUI()

  // Check for pending recording that needs activeTab gesture.
  // When the user clicks the extension icon, activeTab is granted for the active tab.
  // The popup auto-sends RECORDING_GESTURE_GRANTED to unblock the service worker,
  // and shows visual feedback so the user knows recording is starting.
  chrome.storage.local.get(StorageKey.PENDING_RECORDING, (result: Record<string, unknown>) => {
    if (result[StorageKey.PENDING_RECORDING]) {
      // Show immediate feedback in the recording row
      const recordLabel = document.getElementById('record-label')
      const recordStatus = document.getElementById('recording-status')
      const recordOptions = document.getElementById('record-options')
      if (recordLabel) recordLabel.textContent = 'Starting...'
      if (recordStatus) recordStatus.textContent = 'Permission granted'
      if (recordOptions) recordOptions.style.display = 'none'

      chrome.runtime.sendMessage({ type: 'RECORDING_GESTURE_GRANTED' })
      chrome.storage.local.remove(StorageKey.PENDING_RECORDING)
    }
  })

  // Initialize feature toggles
  await initFeatureToggles()

  // Initialize WebSocket mode selector
  await initWebSocketModeSelector()

  // Initialize AI Web Pilot toggle
  await initAiWebPilotToggle()

  // Initialize Track This Page button
  await initTrackPageButton()

  setupWebSocketUI()
  setupToggleWarnings()

  const clearBtn = document.getElementById('clear-btn')
  if (clearBtn) clearBtn.addEventListener('click', handleClearLogs)

  // Initialize draw mode button
  setupDrawModeButton()

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
    }
  )

  // Listen for storage changes (e.g., tracked tab URL updates)
  chrome.storage.onChanged.addListener((changes, areaName) => {
    if (areaName === 'local' && changes[StorageKey.TRACKED_TAB_URL]) {
      const urlEl = document.getElementById('tracking-bar-url')
      if (urlEl && changes[StorageKey.TRACKED_TAB_URL]!.newValue) {
        urlEl.textContent = changes[StorageKey.TRACKED_TAB_URL]!.newValue as string
        console.log('[Gasoline] Tracked tab URL updated in popup:', changes[StorageKey.TRACKED_TAB_URL]!.newValue)
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
