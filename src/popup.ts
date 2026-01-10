/**
 * @fileoverview popup.ts - Extension popup UI showing connection status and controls.
 * Displays server connection state, entry count, error count, log level selector,
 * and log file path. Polls the background worker for status updates and provides
 * a clear-logs button. Shows troubleshooting hints when disconnected.
 * Design: Pure DOM manipulation, no framework. Communicates with background.js
 * via chrome.runtime.sendMessage for status queries and log-level changes.
 */

import type {
  ConnectionStatus,
  MemoryPressureState,
  ContextWarning,
  WebSocketCaptureMode,
} from './types/index'

const DEFAULT_MAX_ENTRIES = 1000

// Extended connection status for popup
interface PopupConnectionStatus extends ConnectionStatus {
  serverUrl?: string
  circuitBreakerState?: 'closed' | 'open' | 'half-open'
  memoryPressure?: MemoryPressureState
  contextWarning?: ContextWarning
  error?: string
}

// Feature toggle configuration type
interface FeatureToggleConfig {
  id: string
  storageKey: string
  messageType: string
  default: boolean
}

// Toggle warning configuration
interface ToggleWarningConfig {
  toggleId: string
  warningId: string
}

/**
 * Format bytes into human-readable file size
 */
function formatFileSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / Math.pow(1024, i)
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[i]}`
}

/**
 * Update the connection status display
 */
export function updateConnectionStatus(status: PopupConnectionStatus): void {
  const statusEl = document.getElementById('status')
  const entriesEl = document.getElementById('entries-count')
  const errorEl = document.getElementById('error-message')
  const serverUrlEl = document.getElementById('server-url')
  const logFileEl = document.getElementById('log-file-path')
  const errorCountEl = document.getElementById('error-count')
  const troubleshootingEl = document.getElementById('troubleshooting')

  if (status.connected) {
    if (statusEl) {
      statusEl.textContent = 'Connected'
      statusEl.classList.remove('disconnected')
      statusEl.classList.add('connected')
    }

    const entries = status.entries || 0
    const maxEntries = status.maxEntries || DEFAULT_MAX_ENTRIES
    if (entriesEl) {
      entriesEl.textContent = `${entries} / ${maxEntries}`
    }

    if (errorEl) {
      errorEl.textContent = ''
    }
    if (troubleshootingEl) {
      troubleshootingEl.style.display = 'none'
    }
  } else {
    if (statusEl) {
      statusEl.textContent = 'Disconnected'
      statusEl.classList.remove('connected')
      statusEl.classList.add('disconnected')
    }

    if (errorEl && status.error) {
      errorEl.textContent = status.error
    }
    if (troubleshootingEl) {
      troubleshootingEl.style.display = 'block'
    }
  }

  // Version mismatch warning
  const versionWarningEl = document.getElementById('version-mismatch')
  if (versionWarningEl) {
    if (status.versionMismatch && status.serverVersion && status.extensionVersion) {
      versionWarningEl.style.display = 'block'
      const versionDetail = versionWarningEl.querySelector('.version-detail')
      if (versionDetail) {
        versionDetail.textContent = `Server: v${status.serverVersion} / Extension: v${status.extensionVersion}`
      }
    } else {
      versionWarningEl.style.display = 'none'
    }
  }

  if (serverUrlEl && status.serverUrl) {
    serverUrlEl.textContent = status.serverUrl
  }

  if (logFileEl && status.logFile) {
    logFileEl.textContent = status.logFile
  }

  if (errorCountEl && status.errorCount !== undefined) {
    errorCountEl.textContent = String(status.errorCount)
  }

  // Log file size
  const fileSizeEl = document.getElementById('log-file-size')
  if (fileSizeEl && status.logFileSize !== undefined) {
    fileSizeEl.textContent = formatFileSize(status.logFileSize)
  }

  // Health indicators (circuit breaker + memory pressure)
  const healthSection = document.getElementById('health-indicators')
  const cbEl = document.getElementById('health-circuit-breaker')
  const mpEl = document.getElementById('health-memory-pressure')

  if (healthSection && cbEl && mpEl) {
    const cbState = status.circuitBreakerState || 'closed'
    const mpState = status.memoryPressure?.memoryPressureLevel || 'normal'

    // Circuit breaker indicator
    cbEl.classList.remove('health-error', 'health-warning')
    if (!status.connected || cbState === 'closed') {
      cbEl.style.display = 'none'
      cbEl.textContent = ''
    } else if (cbState === 'open') {
      cbEl.style.display = ''
      cbEl.classList.add('health-error')
      cbEl.textContent = 'Server: open (paused)'
    } else if (cbState === 'half-open') {
      cbEl.style.display = ''
      cbEl.classList.add('health-warning')
      cbEl.textContent = 'Server: half-open (probing)'
    }

    // Memory pressure indicator
    mpEl.classList.remove('health-error', 'health-warning')
    if (!status.connected || mpState === 'normal') {
      mpEl.style.display = 'none'
      mpEl.textContent = ''
    } else if (mpState === 'soft') {
      mpEl.style.display = ''
      mpEl.classList.add('health-warning')
      mpEl.textContent = 'Memory: elevated (reduced capacities)'
    } else if (mpState === 'hard') {
      mpEl.style.display = ''
      mpEl.classList.add('health-error')
      mpEl.textContent = 'Memory: critical (bodies disabled)'
    }

    // Show/hide entire section
    const cbVisible = status.connected && cbState !== 'closed'
    const mpVisible = status.connected && mpState !== 'normal'
    healthSection.style.display = cbVisible || mpVisible ? '' : 'none'
  }

  // Context annotation warning
  const contextWarningEl = document.getElementById('context-warning')
  const contextWarningTextEl = document.getElementById('context-warning-text')
  if (contextWarningEl) {
    if (status.connected && status.contextWarning) {
      contextWarningEl.style.display = 'block'
      if (contextWarningTextEl) {
        contextWarningTextEl.textContent = `${status.contextWarning.count} recent entries have context annotations averaging ${status.contextWarning.sizeKB}KB. This may consume significant AI context window space.`
      }
    } else {
      contextWarningEl.style.display = 'none'
      if (contextWarningTextEl) {
        contextWarningTextEl.textContent = ''
      }
    }
  }
}

/**
 * Feature toggle configuration
 */
export const FEATURE_TOGGLES: readonly FeatureToggleConfig[] = [
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
 * Initialize all feature toggles
 */
export async function initFeatureToggles(): Promise<void> {
  // Load saved states
  const storageKeys = FEATURE_TOGGLES.map((t) => t.storageKey)

  return new Promise((resolve) => {
    chrome.storage.local.get(storageKeys, (result: Record<string, boolean | undefined>) => {
      for (const toggle of FEATURE_TOGGLES) {
        const checkbox = document.getElementById(toggle.id) as HTMLInputElement | null
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

/**
 * Handle feature toggle change
 * CRITICAL ARCHITECTURE: Popup NEVER writes storage directly.
 * It ONLY sends a message to background, which is the single writer.
 * This prevents desynchronization bugs where UI state diverges from actual state.
 */
export function handleFeatureToggle(storageKey: string, messageType: string, enabled: boolean): void {
  // Send message to background (DO NOT write storage directly)
  // Background will handle the write after updating its internal state
  chrome.runtime.sendMessage({ type: messageType, enabled }, (response: { success?: boolean } | undefined) => {
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
 * Initialize the AI Web Pilot toggle.
 * Read the current state from chrome.storage.local.
 */
export async function initAiWebPilotToggle(): Promise<void> {
  const toggle = document.getElementById('aiWebPilotEnabled') as HTMLInputElement | null
  if (!toggle) return

  return new Promise((resolve) => {
    // Read from chrome.storage.local (single source of truth)
    chrome.storage.local.get(['aiWebPilotEnabled'], (result: { aiWebPilotEnabled?: boolean }) => {
      toggle.checked = result.aiWebPilotEnabled === true

      // Set up change handler
      toggle.addEventListener('change', () => {
        handleAiWebPilotToggle(toggle.checked)
      })

      resolve()
    })
  })
}

/**
 * Check if a URL is an internal browser page that cannot be tracked.
 * Chrome blocks content scripts from these pages, so tracking is impossible.
 */
export function isInternalUrl(url: string | undefined): boolean {
  if (!url) return true
  const internalPrefixes = [
    'chrome://',
    'chrome-extension://',
    'about:',
    'edge://',
    'brave://',
    'devtools://',
  ]
  return internalPrefixes.some((prefix) => url.startsWith(prefix))
}

/**
 * Initialize the Track This Tab button.
 * Shows current tracking status and handles track/untrack.
 * Disables the button on internal Chrome pages where tracking is impossible.
 */
export async function initTrackPageButton(): Promise<void> {
  const btn = document.getElementById('track-page-btn') as HTMLButtonElement | null
  const info = document.getElementById('tracked-page-info')
  const urlEl = document.getElementById('tracked-url')
  if (!btn) return

  return new Promise((resolve) => {
    chrome.storage.local.get(['trackedTabId', 'trackedTabUrl'], async (result: { trackedTabId?: number; trackedTabUrl?: string }) => {
      // Check if current tab is an internal page
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs: chrome.tabs.Tab[]) => {
        const currentTab = tabs && tabs[0]
        const currentUrl = currentTab?.url

        if (isInternalUrl(currentUrl)) {
          // Disable button on internal pages
          btn.disabled = true
          btn.textContent = 'Cannot Track Internal Pages'
          btn.title = 'Chrome blocks extensions on internal pages like chrome:// and about:'
          btn.style.opacity = '0.5'
          btn.style.background = '#252525'
          btn.style.color = '#888'
          btn.style.borderColor = '#333'
          if (info) {
            info.style.display = 'block'
            info.textContent = 'Internal browser pages cannot be tracked'
          }
          resolve()
          return
        }

        // Update UI based on whether we're tracking a tab
        if (result.trackedTabId) {
          // Show tracking info
          btn.textContent = 'Stop Tracking'
          btn.style.background = '#f85149'
          btn.style.color = 'white'
          btn.style.borderColor = '#f85149'
          if (info) info.style.display = 'block'
          if (urlEl && result.trackedTabUrl) {
            urlEl.textContent = result.trackedTabUrl
          }
        } else {
          // Show track button - renamed from "Track This Page" to "Track This Tab"
          btn.textContent = 'Track This Tab'
          btn.style.background = '#252525'
          btn.style.color = '#58a6ff'
          btn.style.borderColor = '#58a6ff'
          if (info) {
            info.style.display = 'block'
            // Only set text on the info container if it's the full HTML element
          }
          // Show "no tracking" status
          const noTrackEl = document.getElementById('no-tracking-warning')
          if (noTrackEl) {
            noTrackEl.style.display = 'block'
          }
        }

        // Set up click handler
        btn.addEventListener('click', handleTrackPageClick)

        resolve()
      })
    })
  })
}

/**
 * Handle Track This Tab button click.
 * Toggles tracking on/off for the current tab.
 * Blocks tracking on internal Chrome pages.
 */
export async function handleTrackPageClick(): Promise<void> {
  const btn = document.getElementById('track-page-btn') as HTMLButtonElement | null
  const info = document.getElementById('tracked-page-info')
  const urlEl = document.getElementById('tracked-url')

  // Check if we're currently tracking
  chrome.storage.local.get(['trackedTabId'], async (result: { trackedTabId?: number }) => {
    if (result.trackedTabId) {
      // Untrack
      chrome.storage.local.remove(['trackedTabId', 'trackedTabUrl'], () => {
        if (btn) {
          btn.textContent = 'Track This Tab'
          btn.style.background = '#252525'
          btn.style.color = '#58a6ff'
          btn.style.borderColor = '#58a6ff'
        }
        if (info) info.style.display = 'none'
        // Show "no tracking" warning
        const noTrackEl = document.getElementById('no-tracking-warning')
        if (noTrackEl) noTrackEl.style.display = 'block'
        console.log('[Gasoline] Stopped tracking')
      })
    } else {
      // Track current tab
      chrome.tabs.query({ active: true, currentWindow: true }, (tabs: chrome.tabs.Tab[]) => {
        if (tabs[0]) {
          const tab = tabs[0]

          // Block tracking on internal Chrome pages
          if (isInternalUrl(tab.url)) {
            if (btn) {
              btn.disabled = true
              btn.textContent = 'Cannot Track Internal Pages'
              btn.style.opacity = '0.5'
            }
            return
          }

          chrome.storage.local.set({ trackedTabId: tab.id, trackedTabUrl: tab.url }, () => {
            if (btn) {
              btn.textContent = 'Stop Tracking'
              btn.style.background = '#f85149'
              btn.style.color = 'white'
              btn.style.borderColor = '#f85149'
            }
            if (info) info.style.display = 'block'
            if (urlEl) urlEl.textContent = tab.url || ''
            // Hide "no tracking" warning
            const noTrackEl = document.getElementById('no-tracking-warning')
            if (noTrackEl) noTrackEl.style.display = 'none'
            console.log('[Gasoline] Now tracking tab:', tab.id, tab.url)
          })
        }
      })
    }
  })
}

/**
 * Handle AI Web Pilot toggle change.
 *
 * CRITICAL: ONLY background.js updates the state via setAiWebPilotEnabled message.
 * Popup NEVER writes to chrome.storage directly.
 *
 * This ensures single source of truth. If popup wrote to storage directly:
 * 1. Popup updates storage
 * 2. Background cache doesn't update (no listener yet)
 * 3. Pilot command checks cache and gets wrong value
 * 4. User sees toggle "on" but commands fail saying "off"
 *
 * By routing through background, we guarantee:
 * 1. Popup sends message to background
 * 2. Background updates cache immediately
 * 3. Background writes to storage
 * 4. Pilot commands see correct cache state
 * 5. Everything is consistent
 */
export async function handleAiWebPilotToggle(enabled: boolean): Promise<void> {
  // ONLY communicate with background - do NOT write to storage directly
  chrome.runtime.sendMessage({ type: 'setAiWebPilotEnabled', enabled }, (response: { success?: boolean } | undefined) => {
    if (!response || !response.success) {
      console.error('[Gasoline] Failed to set AI Web Pilot toggle in background')
      // Revert the UI if background didn't accept the change
      const toggle = document.getElementById('aiWebPilotEnabled') as HTMLInputElement | null
      if (toggle) {
        toggle.checked = !enabled
      }
    }
  })
}

/**
 * Handle WebSocket mode change
 */
export function handleWebSocketModeChange(mode: WebSocketCaptureMode): void {
  chrome.storage.local.set({ webSocketCaptureMode: mode })
  chrome.runtime.sendMessage({ type: 'setWebSocketCaptureMode', mode })
}

/**
 * Initialize the WebSocket mode selector
 */
export async function initWebSocketModeSelector(): Promise<void> {
  const modeSelect = document.getElementById('ws-mode') as HTMLSelectElement | null
  if (!modeSelect) return

  return new Promise((resolve) => {
    chrome.storage.local.get(['webSocketCaptureMode'], (result: { webSocketCaptureMode?: WebSocketCaptureMode }) => {
      modeSelect.value = result.webSocketCaptureMode || 'lifecycle'
      resolve()
    })
  })
}

/**
 * Initialize the log level selector
 */
export async function initLogLevelSelector(): Promise<void> {
  const levelSelect = document.getElementById('log-level') as HTMLSelectElement | null
  if (!levelSelect) return

  // Load saved level
  return new Promise((resolve) => {
    chrome.storage.local.get(['logLevel'], (result: { logLevel?: string }) => {
      levelSelect.value = result.logLevel || 'error'
      resolve()
    })
  })
}

/**
 * Handle log level change
 */
export async function handleLogLevelChange(level: string): Promise<void> {
  chrome.storage.local.set({ logLevel: level })
  chrome.runtime.sendMessage({ type: 'setLogLevel', level })
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
    chrome.runtime.sendMessage({ type: 'clearLogs' }, (response: { success?: boolean; error?: string } | undefined) => {
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

  // Initialize log level selector
  await initLogLevelSelector()

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
    wsMessagesWarning.style.display = wsModeSelect.value === 'messages' ? 'block' : 'none'
    wsModeSelect.addEventListener('change', () => {
      wsMessagesWarning.style.display = wsModeSelect.value === 'messages' ? 'block' : 'none'
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

  // Set up log level change handler
  const levelSelect = document.getElementById('log-level') as HTMLSelectElement | null
  if (levelSelect) {
    levelSelect.addEventListener('change', (e: Event) => {
      const target = e.target as HTMLSelectElement
      handleLogLevelChange(target.value)
    })
  }

  // Set up clear button handler
  const clearBtn = document.getElementById('clear-btn')
  if (clearBtn) {
    clearBtn.addEventListener('click', handleClearLogs)
  }

  // Listen for status updates
  chrome.runtime.onMessage.addListener((message: { type: string; status?: PopupConnectionStatus; enabled?: boolean }) => {
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
  })
}

// Initialize when DOM is ready
if (typeof document !== 'undefined') {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initPopup)
  } else {
    initPopup()
  }
}
