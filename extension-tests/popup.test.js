/**
 * @fileoverview Tests for popup UI functionality
 * TDD: These tests are written BEFORE implementation
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    sendMessage: mock.fn(),
    onMessage: {
      addListener: mock.fn(),
    },
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) =>
        callback({
          logLevel: 'error',
          domainFilters: [],
        })
      ),
      set: mock.fn((data, callback) => callback && callback()),
    },
  },
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
    addEventListener: mock.fn(),
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
    toggle: mock.fn(),
  },
  style: {},
  addEventListener: mock.fn(),
  setAttribute: mock.fn(),
  getAttribute: mock.fn(),
  value: '',
  checked: false,
  disabled: false,
})

let mockDocument

describe('Popup State Display', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.resetCalls()
    mockChrome.storage.local.get.mock.resetCalls()
  })

  test('should display connected status when server is up', async () => {
    const { updateConnectionStatus } = await import('../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      entries: 42,
      maxEntries: 1000,
    })

    const statusEl = mockDocument.getElementById('status')
    const entriesEl = mockDocument.getElementById('entries-count')

    assert.ok(statusEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'connected'))
    assert.ok(statusEl.textContent.toLowerCase().includes('connected'))
    assert.strictEqual(entriesEl.textContent, '42 / 1000')
  })

  test('should display disconnected status when server is down', async () => {
    const { updateConnectionStatus } = await import('../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Connection refused',
    })

    const statusEl = mockDocument.getElementById('status')

    assert.ok(statusEl.classList.add.mock.calls.some((c) => c.arguments[0] === 'disconnected'))
    assert.ok(statusEl.textContent.toLowerCase().includes('disconnected'))
  })

  test('should show error message when disconnected', async () => {
    const { updateConnectionStatus } = await import('../extension/popup.js')

    updateConnectionStatus({
      connected: false,
      error: 'Connection refused',
    })

    const errorEl = mockDocument.getElementById('error-message')
    assert.ok(errorEl.textContent.includes('Connection refused'))
  })

  test('should request status on popup open', async () => {
    const { initPopup } = await import('../extension/popup.js')

    await initPopup()

    // Should have sent getStatus message
    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0]?.type === 'getStatus')
    )
  })
})

describe('Log Level Selector', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.storage.local.set.mock.resetCalls()
  })

  test('should load saved log level on init', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({ logLevel: 'warn' })
    })

    const { initLogLevelSelector } = await import('../extension/popup.js')

    await initLogLevelSelector()

    const levelSelect = mockDocument.getElementById('log-level')
    assert.strictEqual(levelSelect.value, 'warn')
  })

  test('should save log level on change', async () => {
    const { handleLogLevelChange } = await import('../extension/popup.js')

    await handleLogLevelChange('error')

    assert.ok(
      mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0]?.logLevel === 'error')
    )
  })

  test('should notify background when level changes', async () => {
    const { handleLogLevelChange } = await import('../extension/popup.js')

    await handleLogLevelChange('all')

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some(
        (c) => c.arguments[0]?.type === 'setLogLevel' && c.arguments[0]?.level === 'all'
      )
    )
  })

  test('should default to "error" level', async () => {
    mockChrome.storage.local.get.mock.mockImplementation((keys, callback) => {
      callback({}) // No saved value
    })

    const { initLogLevelSelector } = await import('../extension/popup.js')

    await initLogLevelSelector()

    const levelSelect = mockDocument.getElementById('log-level')
    assert.strictEqual(levelSelect.value, 'error')
  })
})

describe('Clear Logs Button', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.resetCalls()
    // Default mock that calls callback
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (callback) callback({ success: true })
    })
  })

  test('should send clearLogs message when clicked', async () => {
    const { handleClearLogs } = await import('../extension/popup.js')

    await handleClearLogs()

    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0]?.type === 'clearLogs')
    )
  })

  test('should update UI after clearing logs', async () => {
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'clearLogs') {
        callback({ success: true })
      }
    })

    const { handleClearLogs } = await import('../extension/popup.js')

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

    const { handleClearLogs } = await import('../extension/popup.js')

    const clearBtn = mockDocument.getElementById('clear-btn')

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

    const { handleClearLogs } = await import('../extension/popup.js')

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
  })

  test('should listen for status updates from background', async () => {
    const { initPopup } = await import('../extension/popup.js')

    await initPopup()

    // Should have registered message listener
    assert.ok(mockChrome.runtime.onMessage.addListener.mock.calls.length > 0)
  })

  test('should update display when status message received', async () => {
    let messageHandler

    mockChrome.runtime.onMessage.addListener.mock.mockImplementation((handler) => {
      messageHandler = handler
    })

    const { initPopup } = await import('../extension/popup.js')

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

    const { initPopup } = await import('../extension/popup.js')

    await initPopup()

    messageHandler({
      type: 'statusUpdate',
      status: {
        connected: true,
        errorCount: 5,
      },
    })

    const errorCountEl = mockDocument.getElementById('error-count')
    assert.strictEqual(errorCountEl.textContent, '5')
  })
})

describe('Quick Actions', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.resetCalls()
  })

  test('should have link to open log file', async () => {
    const { initPopup } = await import('../extension/popup.js')

    await initPopup()

    const openLogLink = mockDocument.getElementById('open-log-file')
    assert.ok(openLogLink)
  })

  test('should have link to options page', async () => {
    const { initPopup } = await import('../extension/popup.js')

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
  })

  test('should display server URL', async () => {
    const { updateConnectionStatus } = await import('../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      serverUrl: 'http://localhost:7890',
    })

    const serverUrlEl = mockDocument.getElementById('server-url')
    assert.ok(serverUrlEl.textContent.includes('localhost:7890'))
  })

  test('should display log file path when connected', async () => {
    const { updateConnectionStatus } = await import('../extension/popup.js')

    updateConnectionStatus({
      connected: true,
      logFile: '/Users/dev/dev-console-logs.jsonl',
    })

    const logFileEl = mockDocument.getElementById('log-file-path')
    assert.ok(logFileEl.textContent.includes('dev-console-logs.jsonl'))
  })
})

describe('Debug Logging', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.runtime.sendMessage.mock.resetCalls()
  })

  test('should export debug log when button clicked', async () => {
    // Mock the debug log response
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'getDebugLog') {
        callback({
          log: JSON.stringify({
            exportedAt: '2024-01-22T12:00:00Z',
            version: '2.0.0',
            entries: [{ ts: '2024-01-22T12:00:00Z', category: 'lifecycle', message: 'Test' }],
          }),
        })
      }
    })

    // Mock URL and createElement for download
    const mockUrl = 'blob:mock-url'
    const mockAnchor = {
      href: '',
      download: '',
      click: mock.fn(),
    }

    globalThis.URL = {
      createObjectURL: mock.fn(() => mockUrl),
      revokeObjectURL: mock.fn(),
    }

    const originalCreateElement = globalThis.document.createElement
    globalThis.document.createElement = mock.fn((tag) => {
      if (tag === 'a') return mockAnchor
      return originalCreateElement?.(tag)
    })
    globalThis.document.body = {
      appendChild: mock.fn(),
      removeChild: mock.fn(),
    }
    globalThis.Blob = function (content, options) {
      this.content = content
      this.type = options?.type
    }

    const { handleExportDebugLog } = await import('../extension/popup.js')

    const result = await handleExportDebugLog()

    // Should have sent getDebugLog message
    assert.ok(mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0].type === 'getDebugLog'))

    // Should have triggered download
    assert.ok(mockAnchor.click.mock.calls.length > 0)
    assert.ok(mockAnchor.download.startsWith('gasoline-debug-'))
    assert.ok(result.success)
  })

  test('should clear debug log when button clicked', async () => {
    mockChrome.runtime.sendMessage.mock.mockImplementation((msg, callback) => {
      if (msg.type === 'clearDebugLog') {
        callback({ success: true })
      }
    })

    const { handleClearDebugLog } = await import('../extension/popup.js')

    const result = await handleClearDebugLog()

    // Should have sent clearDebugLog message
    assert.ok(mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0].type === 'clearDebugLog'))
    assert.ok(result?.success)
  })

  test('should toggle debug mode', async () => {
    const { handleFeatureToggle } = await import('../extension/popup.js')

    handleFeatureToggle('debugMode', 'setDebugMode', true)

    // Should have saved to storage
    assert.ok(mockChrome.storage.local.set.mock.calls.some((c) => c.arguments[0].debugMode === true))

    // Should have sent message to background
    assert.ok(
      mockChrome.runtime.sendMessage.mock.calls.some((c) => c.arguments[0].type === 'setDebugMode' && c.arguments[0].enabled === true)
    )
  })
})
