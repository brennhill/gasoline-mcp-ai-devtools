// @ts-nocheck
/**
 * @fileoverview Popup UI logic for Dev Console extension
 */

const DEFAULT_MAX_ENTRIES = 1000

/**
 * Update the connection status display
 */
export function updateConnectionStatus(status) {
  const statusEl = document.getElementById('status')
  const entriesEl = document.getElementById('entries-count')
  const errorEl = document.getElementById('error-message')
  const serverUrlEl = document.getElementById('server-url')
  const logFileEl = document.getElementById('log-file-path')
  const errorCountEl = document.getElementById('error-count')
  const troubleshootingEl = document.getElementById('troubleshooting')

  if (status.connected) {
    statusEl.textContent = 'Connected'
    statusEl.classList.remove('disconnected')
    statusEl.classList.add('connected')

    const entries = status.entries || 0
    const maxEntries = status.maxEntries || DEFAULT_MAX_ENTRIES
    entriesEl.textContent = `${entries} / ${maxEntries}`

    if (errorEl) {
      errorEl.textContent = ''
    }
    if (troubleshootingEl) {
      troubleshootingEl.style.display = 'none'
    }
  } else {
    statusEl.textContent = 'Disconnected'
    statusEl.classList.remove('connected')
    statusEl.classList.add('disconnected')

    if (errorEl && status.error) {
      errorEl.textContent = status.error
    }
    if (troubleshootingEl) {
      troubleshootingEl.style.display = 'block'
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
  { id: 'toggle-debug-mode', storageKey: 'debugMode', messageType: 'setDebugMode', default: false },
]

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

/**
 * Handle feature toggle change
 */
export function handleFeatureToggle(storageKey, messageType, enabled) {
  // Save to storage
  chrome.storage.local.set({ [storageKey]: enabled })

  // Send message to background
  chrome.runtime.sendMessage({ type: messageType, enabled })
}

/**
 * Handle WebSocket mode change
 */
export function handleWebSocketModeChange(mode) {
  chrome.storage.local.set({ webSocketCaptureMode: mode })
  chrome.runtime.sendMessage({ type: 'setWebSocketCaptureMode', mode })
}

/**
 * Initialize the WebSocket mode selector
 */
export async function initWebSocketModeSelector() {
  const modeSelect = document.getElementById('ws-mode')
  if (!modeSelect) return

  return new Promise((resolve) => {
    chrome.storage.local.get(['webSocketCaptureMode'], (result) => {
      modeSelect.value = result.webSocketCaptureMode || 'lifecycle'
      resolve()
    })
  })
}

/**
 * Initialize the log level selector
 */
export async function initLogLevelSelector() {
  const levelSelect = document.getElementById('log-level')
  if (!levelSelect) return

  // Load saved level
  return new Promise((resolve) => {
    chrome.storage.local.get(['logLevel'], (result) => {
      levelSelect.value = result.logLevel || 'error'
      resolve()
    })
  })
}

/**
 * Handle log level change
 */
export async function handleLogLevelChange(level) {
  chrome.storage.local.set({ logLevel: level })
  chrome.runtime.sendMessage({ type: 'setLogLevel', level })
}

/**
 * Export debug log to a downloadable file
 */
export async function handleExportDebugLog() {
  const exportBtn = document.getElementById('export-debug-btn')

  if (exportBtn) {
    exportBtn.disabled = true
    exportBtn.textContent = 'Exporting...'
  }

  return new Promise((resolve) => {
    chrome.runtime.sendMessage({ type: 'getDebugLog' }, (response) => {
      if (exportBtn) {
        exportBtn.disabled = false
        exportBtn.textContent = 'Export Debug Log'
      }

      if (response?.log) {
        // Create downloadable blob
        const blob = new Blob([response.log], { type: 'application/json' })
        const url = URL.createObjectURL(blob)
        const timestamp = new Date().toISOString().replace(/[:.]/g, '-')
        const filename = `gasoline-debug-${timestamp}.json`

        // Trigger download
        const a = document.createElement('a')
        a.href = url
        a.download = filename
        document.body.appendChild(a)
        a.click()
        document.body.removeChild(a)
        URL.revokeObjectURL(url)

        resolve({ success: true, filename })
      } else {
        resolve({ success: false, error: 'Failed to get debug log' })
      }
    })
  })
}

/**
 * Clear the debug log buffer
 */
export async function handleClearDebugLog() {
  return new Promise((resolve) => {
    chrome.runtime.sendMessage({ type: 'clearDebugLog' }, (response) => {
      resolve(response)
    })
  })
}

/**
 * Handle clear logs button click
 */
export async function handleClearLogs() {
  const clearBtn = document.getElementById('clear-btn')
  const entriesEl = document.getElementById('entries-count')

  if (clearBtn) {
    clearBtn.disabled = true
  }

  return new Promise((resolve) => {
    chrome.runtime.sendMessage({ type: 'clearLogs' }, (response) => {
      if (clearBtn) {
        clearBtn.disabled = false
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

      resolve(response)
    })
  })
}

/**
 * Initialize the popup
 */
export async function initPopup() {
  // Request current status
  chrome.runtime.sendMessage({ type: 'getStatus' }, (status) => {
    if (status) {
      updateConnectionStatus(status)
    }
  })

  // Initialize log level selector
  await initLogLevelSelector()

  // Initialize feature toggles
  await initFeatureToggles()

  // Initialize WebSocket mode selector
  await initWebSocketModeSelector()

  // Show/hide WebSocket mode selector based on toggle
  const wsToggle = document.getElementById('toggle-websocket')
  const wsModeContainer = document.getElementById('ws-mode-container')
  if (wsToggle && wsModeContainer) {
    wsModeContainer.style.display = wsToggle.checked ? 'block' : 'none'
    wsToggle.addEventListener('change', () => {
      wsModeContainer.style.display = wsToggle.checked ? 'block' : 'none'
    })
  }

  // Set up WebSocket mode change handler
  const wsModeSelect = document.getElementById('ws-mode')
  if (wsModeSelect) {
    wsModeSelect.addEventListener('change', (e) => {
      handleWebSocketModeChange(e.target.value)
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
  const toggleWarnings = [
    { toggleId: 'toggle-screenshot', warningId: 'screenshot-warning' },
    { toggleId: 'toggle-network-waterfall', warningId: 'waterfall-warning' },
    { toggleId: 'toggle-performance-marks', warningId: 'perfmarks-warning' },
  ]
  for (const { toggleId, warningId } of toggleWarnings) {
    const toggle = document.getElementById(toggleId)
    const warning = document.getElementById(warningId)
    if (toggle && warning) {
      warning.style.display = toggle.checked ? 'block' : 'none'
      toggle.addEventListener('change', () => {
        warning.style.display = toggle.checked ? 'block' : 'none'
      })
    }
  }

  // Set up log level change handler
  const levelSelect = document.getElementById('log-level')
  if (levelSelect) {
    levelSelect.addEventListener('change', (e) => {
      handleLogLevelChange(e.target.value)
    })
  }

  // Set up clear button handler
  const clearBtn = document.getElementById('clear-btn')
  if (clearBtn) {
    clearBtn.addEventListener('click', handleClearLogs)
  }

  // Set up debug log export button handler
  const exportDebugBtn = document.getElementById('export-debug-btn')
  if (exportDebugBtn) {
    exportDebugBtn.addEventListener('click', handleExportDebugLog)
  }

  // Set up debug log clear button handler
  const clearDebugBtn = document.getElementById('clear-debug-btn')
  if (clearDebugBtn) {
    clearDebugBtn.addEventListener('click', handleClearDebugLog)
  }

  // Listen for status updates
  chrome.runtime.onMessage.addListener((message) => {
    if (message.type === 'statusUpdate') {
      updateConnectionStatus(message.status)
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
