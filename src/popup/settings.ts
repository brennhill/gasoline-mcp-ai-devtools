/**
 * Purpose: Implements popup settings controls for websocket capture mode and safe log clearing actions.
 * Why: Keeps destructive and behavior-changing popup operations centralized with explicit UX safeguards.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

/**
 * @fileoverview Settings Module
 * Handles log level, WebSocket mode, and clear logs functionality
 */

import type { WebSocketCaptureMode } from '../types/index.js'
import { SettingName } from '../lib/constants.js'
import { setLocalValue, getLocalValue } from '../lib/storage-utils.js'

/**
 * Handle WebSocket mode change
 */
export function handleWebSocketModeChange(mode: WebSocketCaptureMode): void {
  setLocalValue('webSocketCaptureMode', mode)
  chrome.runtime.sendMessage({ type: SettingName.WEBSOCKET_CAPTURE_MODE, mode })
}

/**
 * Initialize the WebSocket mode selector
 */
export async function initWebSocketModeSelector(): Promise<void> {
  const modeSelect = document.getElementById('ws-mode') as HTMLSelectElement | null
  if (!modeSelect) return

  return new Promise((resolve) => {
    getLocalValue('webSocketCaptureMode', (value: unknown) => {
      modeSelect.value = (value as WebSocketCaptureMode) || 'medium'
      resolve()
    })
  })
}

// Track clear-logs confirmation state
let clearConfirmPending = false
let clearConfirmTimer: ReturnType<typeof setTimeout> | null = null

/**
 * Reset clear confirmation state (exported for testing)
 */
export function resetClearConfirm(): void {
  clearConfirmPending = false
  if (clearConfirmTimer) {
    clearTimeout(clearConfirmTimer)
    clearConfirmTimer = null
  }
}

/**
 * Handle clear logs button click (with confirmation)
 */
export async function handleClearLogs(): Promise<{ success?: boolean; error?: string } | null> {
  const clearBtn = document.getElementById('clear-btn') as HTMLButtonElement | null
  const entriesEl = document.getElementById('entries-count')

  // Two-click confirmation: first click changes to "Confirm?", second click clears
  if (clearBtn && !clearConfirmPending) {
    clearConfirmPending = true
    clearBtn.textContent = 'Confirm Clear?'
    // Reset after 3 seconds if not confirmed
    clearConfirmTimer = setTimeout(() => {
      clearConfirmPending = false
      if (clearBtn) clearBtn.textContent = 'Clear Logs'
    }, 3000)
    return Promise.resolve(null)
  }

  // Second click: actually clear
  clearConfirmPending = false
  if (clearConfirmTimer) {
    clearTimeout(clearConfirmTimer)
    clearConfirmTimer = null
  }
  if (clearBtn) {
    clearBtn.disabled = true
    clearBtn.textContent = 'Clearing...'
  }

  return new Promise((resolve) => {
    chrome.runtime.sendMessage({ type: 'clear_logs' }, (response: { success?: boolean; error?: string } | undefined) => {
      if (clearBtn) {
        clearBtn.disabled = false
        clearBtn.textContent = 'Clear Logs'
      }

      if (response?.success) {
        if (entriesEl) {
          entriesEl.textContent = '0 / 1000'
        }
      } else if (response?.error) {
        const errorEl = document.getElementById('error-message')
        if (errorEl) {
          errorEl.textContent = response.error
        }
      }

      resolve(response || null)
    })
  })
}
