/**
 * Purpose: Orchestrates popup initialization and binds UI modules for tracking, recording, draw mode, and pilot controls.
 * Why: Keeps popup behavior consistent by coordinating status/state hydration in one lifecycle entrypoint.
 * Docs: docs/features/feature/ai-web-pilot/index.md
 * Docs: docs/features/feature/tab-recording/index.md
 * Docs: docs/features/feature/tab-tracking-ux/index.md
 */

/**
 * @fileoverview popup.ts - Extension popup UI showing connection status and controls.
 * Displays server connection state, entry count, error count, log level selector,
 * and log file path. Polls the background worker for status updates and provides
 * a clear-logs button. Shows troubleshooting hints when disconnected.
 * Design: Pure DOM manipulation, no framework. Communicates with background.js
 * via chrome.runtime.sendMessage for status queries and log-level changes.
 */

import type { WebSocketCaptureMode } from './types/index.js'
import type { PopupConnectionStatus, ToggleWarningConfig } from './popup/types.js'
import type { ShowTrackedHoverLauncherMessage } from './types/runtime-messages.js'
import { RuntimeMessageName, StorageKey } from './lib/constants.js'
import { updateConnectionStatus } from './popup/status-display.js'
import { setupRecordingUI } from './popup/recording.js'
import { setupDrawModeButton } from './popup/draw-mode.js'
import { setupActionRecordingUI } from './popup/action-recording.js'
import { initFeatureToggles } from './popup/feature-toggles.js'
import { initTrackPageButton } from './popup/tab-tracking.js'
import { initAiWebPilotToggle } from './popup/ai-web-pilot.js'
import {
  initWebSocketModeSelector,
  handleWebSocketModeChange,
  handleClearLogs,
  resetClearConfirm
} from './popup/settings.js'

// Re-export for testing
export { resetClearConfirm, handleClearLogs }
export { updateConnectionStatus }
export { FEATURE_TOGGLES, initFeatureToggles } from './popup/feature-toggles.js'
export { handleFeatureToggle } from './popup/feature-toggles.js'
export { initAiWebPilotToggle, handleAiWebPilotToggle } from './popup/ai-web-pilot.js'
export { initTrackPageButton, handleTrackPageClick } from './popup/tab-tracking.js'
export { handleWebSocketModeChange } from './popup/settings.js'
export { initWebSocketModeSelector } from './popup/settings.js'
export { isInternalUrl } from './popup/ui-utils.js'

// Apply theme early to prevent flash of unstyled content (moved from inline script for CSP compliance).
try {
  chrome.storage.local.get('theme', (r: Record<string, unknown>) => {
    void chrome.runtime.lastError
    if (r?.['theme'] === 'light') document.body.classList.add('light-theme')
  })
} catch { /* storage unavailable — default dark theme */ }

const DEFAULT_MAX_ENTRIES = 1000
const RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE: ShowTrackedHoverLauncherMessage = {
  type: RuntimeMessageName.SHOW_TRACKED_HOVER_LAUNCHER
}

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

function requestTrackedHoverLauncherReshow(): void {
  if (!chrome.tabs?.query || !chrome.tabs?.sendMessage) return
  chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
    const tabId = tabs[0]?.id
    if (!tabId) return
    chrome.tabs.sendMessage(tabId, RESHOW_TRACKED_HOVER_LAUNCHER_MESSAGE, () => {
      void chrome.runtime.lastError
    })
  })
}

/** Cache status to session storage so the popup renders instantly on next open. */
function cacheStatus(status: PopupConnectionStatus): void {
  try {
    chrome.storage.session.set({ [StorageKey.POPUP_LAST_STATUS]: status }, () => {
      void chrome.runtime.lastError
    })
  } catch { /* best-effort */ }
}

/**
 * Initialize the popup
 */
export function initPopup(): void {
  // Re-show tracked-tab quick launcher if user hid it from the page UI.
  requestTrackedHoverLauncherReshow()

  // 1) Hydrate immediately from cached status (local, no network, no IPC wait).
  try {
    chrome.storage.session.get([StorageKey.POPUP_LAST_STATUS], (result: Record<string, unknown>) => {
      void chrome.runtime.lastError
      const cached = result?.[StorageKey.POPUP_LAST_STATUS] as PopupConnectionStatus | undefined
      if (cached) updateConnectionStatus(cached)
    })
  } catch { /* session storage unavailable — will show defaults until fresh data arrives */ }

  // 2) Request fresh status from background worker (async — updates UI when ready).
  try {
    chrome.runtime.sendMessage({ type: 'getStatus' }, (status: PopupConnectionStatus | undefined) => {
      if (chrome.runtime.lastError) {
        updateConnectionStatus({
          connected: false,
          entries: 0,
          maxEntries: DEFAULT_MAX_ENTRIES,
          errorCount: 0,
          logFile: '',
          error: 'Extension restarting — please wait a moment and reopen popup'
        })
        return
      }
      if (status) {
        updateConnectionStatus(status)
        cacheStatus(status)
      }
    })
  } catch {
    updateConnectionStatus({
      connected: false,
      entries: 0,
      maxEntries: DEFAULT_MAX_ENTRIES,
      errorCount: 0,
      logFile: '',
      error: 'Extension error — try reloading the extension'
    })
  }

  // Initialize all UI synchronously — no awaits, no blocking.
  // Each init reads chrome.storage via callback and updates DOM when ready.
  // None depend on each other, so they all fire in parallel.
  setupRecordingUI()
  setupActionRecordingUI()
  initFeatureToggles()
  initWebSocketModeSelector()
  initAiWebPilotToggle()
  initTrackPageButton()
  setupWebSocketUI()
  setupToggleWarnings()
  setupDrawModeButton()

  const clearBtn = document.getElementById('clear-btn')
  if (clearBtn) clearBtn.addEventListener('click', handleClearLogs)

  // Check for pending recording that needs activeTab gesture (fire-and-forget).
  chrome.storage.local.get(StorageKey.PENDING_RECORDING, (result: Record<string, unknown>) => {
    if (result[StorageKey.PENDING_RECORDING]) {
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

  // Listen for status updates
  chrome.runtime.onMessage.addListener(
    (message: { type: string; status?: PopupConnectionStatus; enabled?: boolean }) => {
      if (message.type === 'statusUpdate' && message.status) {
        updateConnectionStatus(message.status)
        cacheStatus(message.status)
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
