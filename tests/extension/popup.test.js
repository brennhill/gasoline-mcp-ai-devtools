// @ts-nocheck
/**
 * @fileoverview popup.test.js — Tests for the extension popup UI.
 * Covers connection status display, entry count formatting, error messaging,
 * log level selector, clear-logs button, and troubleshooting hint visibility.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    sendMessage: mock.fn(() => Promise.resolve()),
    onMessage: {
      addListener: mock.fn()
    }
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
      remove: mock.fn((keys, callback) => callback && callback())
    },
    sync: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback())
    },
    onChanged: {
      addListener: mock.fn()
    }
  },
  tabs: {
    query: mock.fn((queryInfo, callback) => callback([{ id: 1, url: 'http://localhost:3000' }]))
  }
}

globalThis.chrome = mockChrome

// Mock DOM elements
const createMockDocument = () => {
  const elements = {}

  return {
    getElementById: mock.fn((id) => {
      if (!elements[id]) {
        elements[id] = createMockElement(id)
      }
      return elements[id]
    }),
    querySelector: mock.fn(),
    querySelectorAll: mock.fn(() => []),
    addEventListener: mock.fn()
  }
}

const createMockElement = (id) => ({
  id,
  textContent: '',
  innerHTML: '',
  className: '',
  classList: {
    add: mock.fn(),
    remove: mock.fn(),
    toggle: mock.fn()
  },
  style: {},
  addEventListener: mock.fn(),
  setAttribute: mock.fn(),
  getAttribute: mock.fn(),
  value: '',
  checked: false,
  disabled: false
})

let mockDocument

describe('Popup State Display', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should display connected status when server is up', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 42,
      maxEntries: 1000
    })

    const statusEl = mockDocument.getElementById('status')
    const entriesEl = mockDocument.getElementById('entries-count')

    assert.ok(statusEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'connected'))
    assert.ok(statusEl.textContent.toLowerCase().includes('connected'))
    assert.strictEqual(entriesEl.textContent, '42 / 1000')
  })

  test('should display disconnected status when server is down', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Connection refused'
    })

    const statusEl = mockDocument.getElementById('status')

    assert.ok(statusEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'disconnected'))
    assert.ok(statusEl.textContent.toLowerCase().includes('disconnected'))
  })

  test('should show error message when disconnected', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Connection refused'
    })

    const errorEl = mockDocument.getElementById('error-message')
    assert.ok(errorEl.textContent.includes('Connection refused'))
  })

  test('should request status on popup open', async () => {
    const { initPopup } = await import('../../extension/popup.js')

    await initPopup()

    // Should have sent getStatus message
    assert.ok(mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0]?.type === 'getStatus'))
  })
})

// Log Level Selector tests removed — log level dropdown was removed from popup UI.
// Log level is now hardcoded to 'all' in background/init.ts.

describe('Clear Logs Button', () => {
  beforeEach(async () => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Default mock that calls callback and returns Promise
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (callback) callback({ success: true })
      return Promise.resolve()
    })
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    // Reset confirmation state between tests
    const { resetClearConfirm } = await import('../../extension/popup.js')
    resetClearConfirm()
  })

  test('should require confirmation before clearing', async () => {
    const { handleClearLogs } = await import('../../extension/popup.js')

    // First click: shows confirmation prompt
    await handleClearLogs()

    const clearBtn = mockDocument.getElementById('clear-btn')
    assert.strictEqual(clearBtn.textContent, 'Confirm Clear?')

    // No clearLogs message sent yet
    assert.ok(!mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0]?.type === 'clearLogs'))
  })

  test('should send clearLogs message on second click', async () => {
    const { handleClearLogs } = await import('../../extension/popup.js')

    // First click: confirmation
    await handleClearLogs()
    // Second click: actually clears
    await handleClearLogs()

    assert.ok(mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0]?.type === 'clearLogs'))
  })

  test('should update UI after clearing logs', async () => {
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'clearLogs') {
        callback({ success: true })
      }
    })

    const { handleClearLogs } = await import('../../extension/popup.js')

    // First click: confirmation, second click: clear
    await handleClearLogs()
    await handleClearLogs()

    const entriesEl = mockDocument.getElementById('entries-count')
    assert.strictEqual(entriesEl.textContent, '0 / 1000')
  })

  test('should disable button while clearing', async () => {
    let resolvePromise
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'clearLogs') {
        return new Promise((resolve) => {
          resolvePromise = () => {
            callback({ success: true })
            resolve()
          }
        })
      }
    })

    const { handleClearLogs } = await import('../../extension/popup.js')

    const clearBtn = mockDocument.getElementById('clear-btn')

    // First click: confirmation
    await handleClearLogs()
    // Second click: clear
    const promise = handleClearLogs()

    // Button should be disabled during operation
    assert.strictEqual(clearBtn.disabled, true)

    resolvePromise()
    await promise

    // Button should be re-enabled after
    assert.strictEqual(clearBtn.disabled, false)
  })

  test('should show error if clear fails', async () => {
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'clearLogs') {
        callback({ success: false, error: 'Server not responding' })
      }
    })

    const { handleClearLogs } = await import('../../extension/popup.js')

    // First click: confirmation, second click: clear
    await handleClearLogs()
    await handleClearLogs()

    const errorEl = mockDocument.getElementById('error-message')
    assert.ok(errorEl.textContent.includes('Server not responding'))
  })
})

describe('Status Updates', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should listen for status updates from background', async () => {
    const { initPopup } = await import('../../extension/popup.js')

    await initPopup()

    // Should have registered message listener
    assert.ok(mockChrome.runtime.onMessage.addListener.mock.calls.length > 0)
  })

  test('should update display when status message received', async () => {
    let messageHandler

    mockChrome.runtime.onMessage.addListener.mock.mockImplementation((handler) => {
      messageHandler = handler
    })

    const { initPopup } = await import('../../extension/popup.js')

    await initPopup()

    // Simulate status update from background
    messageHandler({ type: 'statusUpdate', status: { connected: true, entries: 100 } })

    const statusEl = mockDocument.getElementById('status')
    assert.ok(statusEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'connected'))
  })

  test('should update error count badge display', async () => {
    let messageHandler

    mockChrome.runtime.onMessage.addListener.mock.mockImplementation((handler) => {
      messageHandler = handler
    })

    const { initPopup } = await import('../../extension/popup.js')

    await initPopup()

    messageHandler({
      type: 'statusUpdate',
      status: {
        connected: true,
        errorCount: 5
      }
    })

    const errorCountEl = mockDocument.getElementById('error-count')
    assert.strictEqual(errorCountEl.textContent, '5')
  })
})

describe('Context Annotation Warning', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should show context warning when status has contextWarning', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 10,
      contextWarning: {
        sizeKB: 25,
        count: 4,
        triggeredAt: Date.now()
      }
    })

    const warningEl = mockDocument.getElementById('context-warning')
    assert.strictEqual(warningEl.style.display, 'block')
  })

  test('should populate warning text with size and count info', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 10,
      contextWarning: {
        sizeKB: 30,
        count: 5,
        triggeredAt: Date.now()
      }
    })

    const warningTextEl = mockDocument.getElementById('context-warning-text')
    assert.ok(warningTextEl.textContent.includes('30'))
    assert.ok(warningTextEl.textContent.includes('5'))
  })

  test('should hide context warning when contextWarning is null', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 10,
      contextWarning: null
    })

    const warningEl = mockDocument.getElementById('context-warning')
    assert.strictEqual(warningEl.style.display, 'none')
  })

  test('should hide context warning when contextWarning is undefined', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 10
    })

    const warningEl = mockDocument.getElementById('context-warning')
    assert.strictEqual(warningEl.style.display, 'none')
  })

  test('should hide context warning when disconnected even if warning exists', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Connection refused',
      contextWarning: {
        sizeKB: 25,
        count: 3,
        triggeredAt: Date.now()
      }
    })

    const warningEl = mockDocument.getElementById('context-warning')
    assert.strictEqual(warningEl.style.display, 'none')
  })
})

describe('Quick Actions', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should have link to open log file', async () => {
    const { initPopup } = await import('../../extension/popup.js')

    await initPopup()

    const openLogLink = mockDocument.getElementById('open-log-file')
    assert.ok(openLogLink)
  })

  test('should have link to options page', async () => {
    const { initPopup } = await import('../../extension/popup.js')

    await initPopup()

    const optionsLink = mockDocument.getElementById('options-link')
    assert.ok(optionsLink)
  })
})

describe('Server URL Display', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  test('should display server URL', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      serverUrl: 'http://localhost:7890'
    })

    const serverUrlEl = mockDocument.getElementById('server-url')
    assert.ok(serverUrlEl.textContent.includes('localhost:7890'))
  })

  test('should display log file path when connected', async () => {
    const { updateConnectionStatus } = await import('../../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      logFile: '/Users/dev/dev-console-logs.jsonl'
    })

    const logFileEl = mockDocument.getElementById('log-file-path')
    assert.ok(logFileEl.textContent.includes('dev-console-logs.jsonl'))
  })
})

describe('WebSocket Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should load saved WebSocket capture state on init', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ webSocketCaptureEnabled: true, webSocketCaptureMode: 'high' })
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')

    await initFeatureToggles()

    const wsToggle = mockDocument.getElementById('toggle-websocket')
    assert.strictEqual(wsToggle.checked, true)
  })

  test('should default WebSocket capture to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({}) // No saved value — defaults to ON
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')

    await initFeatureToggles()

    const wsToggle = mockDocument.getElementById('toggle-websocket')
    assert.strictEqual(wsToggle.checked, true)
  })

  test('should send message to background when WebSocket toggled', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('webSocketCaptureEnabled', 'setWebSocketCaptureEnabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setWebSocketCaptureEnabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send mode change message to background', async () => {
    const { handleWebSocketModeChange } = await import('../../extension/popup.js')

    handleWebSocketModeChange('high')

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setWebSocketCaptureMode' && c.arguments[0].mode === 'high'
      )
    )
  })

  test('should default mode to medium', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({}) // No saved value
    })

    const { initWebSocketModeSelector } = await import('../../extension/popup.js')

    await initWebSocketModeSelector()

    const modeSelect = mockDocument.getElementById('ws-mode')
    assert.strictEqual(modeSelect.value, 'medium')
  })

  test('should load saved mode on init', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ webSocketCaptureMode: 'high' })
    })

    const { initWebSocketModeSelector } = await import('../../extension/popup.js')

    await initWebSocketModeSelector()

    const modeSelect = mockDocument.getElementById('ws-mode')
    assert.strictEqual(modeSelect.value, 'high')
  })
})

describe('Debug Logging', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should toggle debug mode and send message to background', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('debugMode', 'setDebugMode', true)

    // Should have sent message to background (popup does not save to storage)
    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setDebugMode' && c.arguments[0].enabled === true
      )
    )
  })
})

describe('Health Indicators', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
  })

  describe('Circuit Breaker Status', () => {
    test('should hide circuit breaker indicator when state is "closed"', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const cbEl = mockDocument.getElementById('health-circuit-breaker')
      assert.strictEqual(cbEl.style.display, 'none')
    })

    test('should display circuit breaker "open" with error styling', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'open',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const cbEl = mockDocument.getElementById('health-circuit-breaker')
      assert.notStrictEqual(cbEl.style.display, 'none')
      assert.ok(cbEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'health-error'))
      assert.ok(cbEl.textContent.includes('paused'))
    })

    test('should display circuit breaker "half-open" with warning styling', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'half-open',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const cbEl = mockDocument.getElementById('health-circuit-breaker')
      assert.notStrictEqual(cbEl.style.display, 'none')
      assert.ok(cbEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'health-warning'))
      assert.ok(cbEl.textContent.includes('recovering'))
    })
  })

  describe('Memory Pressure Status', () => {
    test('should hide memory pressure indicator when level is "normal"', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const mpEl = mockDocument.getElementById('health-memory-pressure')
      assert.strictEqual(mpEl.style.display, 'none')
    })

    test('should display memory pressure "soft" with warning styling', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'soft', reducedCapacities: true }
      })

      const mpEl = mockDocument.getElementById('health-memory-pressure')
      assert.notStrictEqual(mpEl.style.display, 'none')
      assert.ok(mpEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'health-warning'))
      assert.ok(mpEl.textContent.includes('elevated'))
    })

    test('should display memory pressure "hard" with error styling', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'hard', networkBodyCaptureDisabled: true }
      })

      const mpEl = mockDocument.getElementById('health-memory-pressure')
      assert.notStrictEqual(mpEl.style.display, 'none')
      assert.ok(mpEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'health-error'))
      assert.ok(mpEl.textContent.includes('critical'))
    })
  })

  describe('Section Visibility', () => {
    test('should hide health section when all indicators are healthy', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const sectionEl = mockDocument.getElementById('health-indicators')
      assert.strictEqual(sectionEl.style.display, 'none')
    })

    test('should show health section when circuit breaker is unhealthy', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'open',
        memoryPressure: { memoryPressureLevel: 'normal' }
      })

      const sectionEl = mockDocument.getElementById('health-indicators')
      assert.notStrictEqual(sectionEl.style.display, 'none')
    })

    test('should show health section when memory pressure is elevated', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: true,
        entries: 5,
        circuitBreakerState: 'closed',
        memoryPressure: { memoryPressureLevel: 'soft' }
      })

      const sectionEl = mockDocument.getElementById('health-indicators')
      assert.notStrictEqual(sectionEl.style.display, 'none')
    })

    test('should hide health indicators when disconnected', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      updateConnectionStatus({
        connected: false,
        error: 'Connection refused',
        circuitBreakerState: 'open',
        memoryPressure: { memoryPressureLevel: 'hard' }
      })

      const sectionEl = mockDocument.getElementById('health-indicators')
      assert.strictEqual(sectionEl.style.display, 'none')
    })

    test('should handle missing health data gracefully', async () => {
      const { updateConnectionStatus } = await import('../../extension/popup.js')

      // No circuitBreakerState or memoryPressure in status
      assert.doesNotThrow(() => {
        updateConnectionStatus({
          connected: true,
          entries: 10
        })
      })
    })
  })
})

describe('Network Body Capture Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    // Restore default mock implementations after reset
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include network body capture in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-network-body-capture')
    assert.ok(toggle, 'Network body capture toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'networkBodyCaptureEnabled')
    assert.strictEqual(toggle.messageType, 'setNetworkBodyCaptureEnabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should send setNetworkBodyCaptureEnabled message when toggled', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('networkBodyCaptureEnabled', 'setNetworkBodyCaptureEnabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setNetworkBodyCaptureEnabled' && c.arguments[0].enabled === false
      )
    )
  })

  test('should send message when network body capture toggled', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('networkBodyCaptureEnabled', 'setNetworkBodyCaptureEnabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setNetworkBodyCaptureEnabled' && c.arguments[0].enabled === true
      )
    )
  })
})

describe('Network Waterfall Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include network waterfall in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-network-waterfall')
    assert.ok(toggle, 'Network waterfall toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'networkWaterfallEnabled')
    assert.strictEqual(toggle.messageType, 'setNetworkWaterfallEnabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default network waterfall to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({}) // No saved value — defaults to ON
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-network-waterfall')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved network waterfall state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ networkWaterfallEnabled: false })
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-network-waterfall')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setNetworkWaterfallEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('networkWaterfallEnabled', 'setNetworkWaterfallEnabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setNetworkWaterfallEnabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setNetworkWaterfallEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('networkWaterfallEnabled', 'setNetworkWaterfallEnabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setNetworkWaterfallEnabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Performance Marks Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include performance marks in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-performance-marks')
    assert.ok(toggle, 'Performance marks toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'performanceMarksEnabled')
    assert.strictEqual(toggle.messageType, 'setPerformanceMarksEnabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default performance marks to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-performance-marks')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved performance marks state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ performanceMarksEnabled: false })
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-performance-marks')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setPerformanceMarksEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('performanceMarksEnabled', 'setPerformanceMarksEnabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setPerformanceMarksEnabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setPerformanceMarksEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('performanceMarksEnabled', 'setPerformanceMarksEnabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setPerformanceMarksEnabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Action Replay Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include action replay in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-action-replay')
    assert.ok(toggle, 'Action replay toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'actionReplayEnabled')
    assert.strictEqual(toggle.messageType, 'setActionReplayEnabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default action replay to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-action-replay')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved action replay state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ actionReplayEnabled: false })
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-action-replay')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setActionReplayEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('actionReplayEnabled', 'setActionReplayEnabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setActionReplayEnabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setActionReplayEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('actionReplayEnabled', 'setActionReplayEnabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setActionReplayEnabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Screenshot on Error Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include screenshot in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-screenshot')
    assert.ok(toggle, 'Screenshot toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'screenshotOnError')
    assert.strictEqual(toggle.messageType, 'setScreenshotOnError')
    assert.strictEqual(toggle.default, true)
  })

  test('should default screenshot on error to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-screenshot')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved screenshot state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ screenshotOnError: false })
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-screenshot')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setScreenshotOnError message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('screenshotOnError', 'setScreenshotOnError', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setScreenshotOnError' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setScreenshotOnError message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('screenshotOnError', 'setScreenshotOnError', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setScreenshotOnError' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Source Maps Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include source maps in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-source-maps')
    assert.ok(toggle, 'Source maps toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'sourceMapEnabled')
    assert.strictEqual(toggle.messageType, 'setSourceMapEnabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default source maps to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-source-maps')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved source maps state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ sourceMapEnabled: false })
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-source-maps')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setSourceMapEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('sourceMapEnabled', 'setSourceMapEnabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setSourceMapEnabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setSourceMapEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('sourceMapEnabled', 'setSourceMapEnabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setSourceMapEnabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Action Toasts Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include action toasts in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-action-toasts')
    assert.ok(toggle, 'Action toasts toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'actionToastsEnabled')
    assert.strictEqual(toggle.messageType, 'setActionToastsEnabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default action toasts to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-action-toasts')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved action toasts state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ actionToastsEnabled: false })
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-action-toasts')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setActionToastsEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('actionToastsEnabled', 'setActionToastsEnabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setActionToastsEnabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setActionToastsEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('actionToastsEnabled', 'setActionToastsEnabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setActionToastsEnabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('Subtitles Toggle', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.mockImplementation(() => Promise.resolve())
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => callback({}))
    mockChrome.tabs.query.mock.mockImplementation((queryInfo, callback) =>
      callback([{ id: 1, url: 'http://localhost:3000' }])
    )
    mockChrome.storage.local.set.mock.mockImplementation((data, callback) => callback && callback())
  })

  test('should include subtitles in FEATURE_TOGGLES', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const toggle = FEATURE_TOGGLES.find((t) => t.id === 'toggle-subtitles')
    assert.ok(toggle, 'Subtitles toggle should exist in FEATURE_TOGGLES')
    assert.strictEqual(toggle.storageKey, 'subtitlesEnabled')
    assert.strictEqual(toggle.messageType, 'setSubtitlesEnabled')
    assert.strictEqual(toggle.default, true)
  })

  test('should default subtitles to ON', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({})
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-subtitles')
    assert.strictEqual(toggle.checked, true)
  })

  test('should load saved subtitles state', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ subtitlesEnabled: false })
    })

    const { initFeatureToggles } = await import('../../extension/popup.js')
    await initFeatureToggles()

    const toggle = mockDocument.getElementById('toggle-subtitles')
    assert.strictEqual(toggle.checked, false)
  })

  test('should send setSubtitlesEnabled message when toggled on', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('subtitlesEnabled', 'setSubtitlesEnabled', true)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setSubtitlesEnabled' && c.arguments[0].enabled === true
      )
    )
  })

  test('should send setSubtitlesEnabled message when toggled off', async () => {
    const { handleFeatureToggle } = await import('../../extension/popup.js')

    handleFeatureToggle('subtitlesEnabled', 'setSubtitlesEnabled', false)

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0].type === 'setSubtitlesEnabled' && c.arguments[0].enabled === false
      )
    )
  })
})

describe('FEATURE_TOGGLES Completeness', () => {
  test('should have exactly 9 feature toggles', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    assert.strictEqual(FEATURE_TOGGLES.length, 9, 'Should have 9 feature toggles')
  })

  test('all toggles should have required fields', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    for (const toggle of FEATURE_TOGGLES) {
      assert.ok(toggle.id, `Toggle missing id`)
      assert.ok(toggle.storageKey, `Toggle ${toggle.id} missing storageKey`)
      assert.ok(toggle.messageType, `Toggle ${toggle.id} missing messageType`)
      assert.strictEqual(typeof toggle.default, 'boolean', `Toggle ${toggle.id} default should be boolean`)
    }
  })

  test('all toggle IDs should be unique', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const ids = FEATURE_TOGGLES.map((t) => t.id)
    const uniqueIds = new Set(ids)
    assert.strictEqual(uniqueIds.size, ids.length, 'All toggle IDs should be unique')
  })

  test('all storage keys should be unique', async () => {
    const { FEATURE_TOGGLES } = await import('../../extension/popup.js')

    const keys = FEATURE_TOGGLES.map((t) => t.storageKey)
    const uniqueKeys = new Set(keys)
    assert.strictEqual(uniqueKeys.size, keys.length, 'All storage keys should be unique')
  })
})
