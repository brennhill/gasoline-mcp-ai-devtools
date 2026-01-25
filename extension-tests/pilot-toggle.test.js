// @ts-nocheck
/**
 * @fileoverview pilot-toggle.test.js — Tests for AI Web Pilot toggle infrastructure.
 * Covers toggle default state, persistence, and pilot command gating.
 * The toggle controls whether AI can execute page interactions.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    sendMessage: mock.fn(() => Promise.resolve()),
    onMessage: {
      addListener: mock.fn(),
    },
  },
  storage: {
    sync: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
    },
    local: {
      get: mock.fn((keys, callback) => callback({})),
      set: mock.fn((data, callback) => callback && callback()),
    },
  },
  tabs: {
    query: mock.fn((query, callback) => callback([{ id: 1 }])),
    sendMessage: mock.fn(() => Promise.resolve()),
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
    readyState: 'complete',
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

describe('AI Web Pilot Toggle Default State', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.storage.sync.get.mock.resetCalls()
    mockChrome.storage.sync.set.mock.resetCalls()
  })

  test('toggle should default to false (disabled)', async () => {
    // Mock no saved value
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({}) // Empty - no saved value
    })

    const { initAiWebPilotToggle } = await import('../extension/popup.js')

    await initAiWebPilotToggle()

    const toggle = mockDocument.getElementById('aiWebPilotEnabled')
    assert.strictEqual(toggle.checked, false, 'AI Web Pilot toggle should default to OFF')
  })

  test('toggle should load saved state from chrome.storage.sync', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: true })
    })

    const { initAiWebPilotToggle } = await import('../extension/popup.js')

    await initAiWebPilotToggle()

    const toggle = mockDocument.getElementById('aiWebPilotEnabled')
    assert.strictEqual(toggle.checked, true, 'Toggle should reflect saved state')
  })
})

describe('AI Web Pilot Toggle Persistence', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.storage.sync.get.mock.resetCalls()
    mockChrome.storage.sync.set.mock.resetCalls()
  })

  test('should save state to chrome.storage.sync when toggled on', async () => {
    const { handleAiWebPilotToggle } = await import('../extension/popup.js')

    await handleAiWebPilotToggle(true)

    assert.ok(
      mockChrome.storage.sync.set.mock.calls.some((c) => c.arguments[0]?.aiWebPilotEnabled === true),
      'Should save aiWebPilotEnabled=true to storage.sync',
    )
  })

  test('should save state to chrome.storage.sync when toggled off', async () => {
    const { handleAiWebPilotToggle } = await import('../extension/popup.js')

    await handleAiWebPilotToggle(false)

    assert.ok(
      mockChrome.storage.sync.set.mock.calls.some((c) => c.arguments[0]?.aiWebPilotEnabled === false),
      'Should save aiWebPilotEnabled=false to storage.sync',
    )
  })
})

describe('AI Web Pilot Command Gating', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.storage.sync.get.mock.resetCalls()
    mockChrome.runtime.sendMessage.mock.resetCalls()
  })

  test('isAiWebPilotEnabled should return false when toggle is off', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: false })
    })

    const { isAiWebPilotEnabled } = await import('../extension/background.js')

    const enabled = await isAiWebPilotEnabled()
    assert.strictEqual(enabled, false, 'Should return false when toggle is off')
  })

  test('isAiWebPilotEnabled should return false when toggle is undefined', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({}) // No value set
    })

    const { isAiWebPilotEnabled } = await import('../extension/background.js')

    const enabled = await isAiWebPilotEnabled()
    assert.strictEqual(enabled, false, 'Should return false when toggle is undefined')
  })

  test('isAiWebPilotEnabled should return true when toggle is on', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: true })
    })

    const { isAiWebPilotEnabled } = await import('../extension/background.js')

    const enabled = await isAiWebPilotEnabled()
    assert.strictEqual(enabled, true, 'Should return true when toggle is on')
  })
})

describe('Pilot Commands Rejection When Disabled', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.storage.sync.get.mock.resetCalls()
  })

  test('GASOLINE_HIGHLIGHT command should return error when pilot disabled', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: false })
    })

    const { handlePilotCommand } = await import('../extension/background.js')

    const result = await handlePilotCommand('GASOLINE_HIGHLIGHT', { selector: '.test' })

    assert.ok(result.error, 'Should return an error')
    assert.strictEqual(result.error, 'ai_web_pilot_disabled', 'Should return ai_web_pilot_disabled error')
  })

  test('GASOLINE_MANAGE_STATE command should return error when pilot disabled', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: false })
    })

    const { handlePilotCommand } = await import('../extension/background.js')

    const result = await handlePilotCommand('GASOLINE_MANAGE_STATE', { action: 'save' })

    assert.ok(result.error, 'Should return an error')
    assert.strictEqual(result.error, 'ai_web_pilot_disabled', 'Should return ai_web_pilot_disabled error')
  })

  test('GASOLINE_EXECUTE_JS command should return error when pilot disabled', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: false })
    })

    const { handlePilotCommand } = await import('../extension/background.js')

    const result = await handlePilotCommand('GASOLINE_EXECUTE_JS', { script: 'console.log("test")' })

    assert.ok(result.error, 'Should return an error')
    assert.strictEqual(result.error, 'ai_web_pilot_disabled', 'Should return ai_web_pilot_disabled error')
  })
})

describe('Pilot Commands Acceptance When Enabled', () => {
  beforeEach(() => {
    mock.reset()
    mockDocument = createMockDocument()
    globalThis.document = mockDocument
    mockChrome.storage.sync.get.mock.resetCalls()
    mockChrome.tabs.sendMessage.mock.resetCalls()
  })

  test('GASOLINE_HIGHLIGHT command should be accepted when pilot enabled', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: true })
    })

    // Mock tabs.sendMessage to simulate successful forwarding
    mockChrome.tabs.sendMessage.mock.mockImplementation(() =>
      Promise.resolve({ success: true }),
    )

    const { handlePilotCommand } = await import('../extension/background.js')

    const result = await handlePilotCommand('GASOLINE_HIGHLIGHT', { selector: '.test' })

    assert.ok(!result.error, 'Should not return an error when enabled')
  })

  test('GASOLINE_MANAGE_STATE command should be accepted when pilot enabled', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: true })
    })

    mockChrome.tabs.sendMessage.mock.mockImplementation(() =>
      Promise.resolve({ success: true }),
    )

    const { handlePilotCommand } = await import('../extension/background.js')

    const result = await handlePilotCommand('GASOLINE_MANAGE_STATE', { action: 'list' })

    assert.ok(!result.error, 'Should not return an error when enabled')
  })

  test('GASOLINE_EXECUTE_JS command should be accepted when pilot enabled', async () => {
    mockChrome.storage.sync.get.mock.mockImplementation((keys, callback) => {
      callback({ aiWebPilotEnabled: true })
    })

    mockChrome.tabs.sendMessage.mock.mockImplementation(() =>
      Promise.resolve({ result: 'executed' }),
    )

    const { handlePilotCommand } = await import('../extension/background.js')

    const result = await handlePilotCommand('GASOLINE_EXECUTE_JS', { script: 'return 1+1' })

    assert.ok(!result.error, 'Should not return an error when enabled')
  })
})
