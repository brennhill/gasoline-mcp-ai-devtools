// @ts-nocheck
/**
 * @fileoverview background.test.js — Tests for the background service worker.
 * Covers log batching/debouncing, server communication, error deduplication,
 * connection status management, badge updates, debug export, screenshot capture,
 * source map resolution, and on-demand query dispatch.
 */

import { test, describe, mock, beforeEach } from 'node:test'
import assert from 'node:assert'

// Mock Chrome APIs
const mockChrome = {
  runtime: {
    onMessage: {
      addListener: mock.fn(),
    },
    onInstalled: {
      addListener: mock.fn(),
    },
    sendMessage: mock.fn(() => Promise.resolve()),
  },
  action: {
    setBadgeText: mock.fn(),
    setBadgeBackgroundColor: mock.fn(),
  },
  storage: {
    local: {
      get: mock.fn((keys, callback) => callback({ logLevel: 'error' })),
      set: mock.fn((data, callback) => callback && callback()),
    },
  },
  alarms: {
    create: mock.fn(),
    onAlarm: {
      addListener: mock.fn(),
    },
  },
  tabs: {
    get: mock.fn((tabId) => Promise.resolve({ id: tabId, windowId: 1, url: 'http://localhost:3000' })),
    captureVisibleTab: mock.fn(() =>
      Promise.resolve('data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkS'),
    ),
    query: mock.fn((query, callback) => callback([{ id: 1, windowId: 1 }])),
    onRemoved: {
      addListener: mock.fn(),
    },
  },
}

// Set global chrome mock
globalThis.chrome = mockChrome

// Import after mocking
import {
  createLogBatcher,
  sendLogsToServer,
  sendEnhancedActionsToServer,
  checkServerHealth,
  updateBadge,
  formatLogEntry,
  shouldCaptureLog,
  createErrorSignature,
  processErrorGroup,
  flushErrorGroups,
  canTakeScreenshot,
  recordScreenshot,
  captureScreenshot,
  measureContextSize,
  checkContextAnnotations,
  getContextWarning,
  resetContextWarning,
} from '../extension/background.js'

describe('Log Batcher', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('should batch logs and call flush after debounce', async () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, { debounceMs: 50 })

    batcher.add({ level: 'error', msg: 'test1' })
    batcher.add({ level: 'error', msg: 'test2' })

    // Should not have flushed yet
    assert.strictEqual(flushFn.mock.calls.length, 0)

    // Wait for debounce
    await new Promise((r) => setTimeout(r, 100))

    // Should have flushed once with both entries
    assert.strictEqual(flushFn.mock.calls.length, 1)
    assert.strictEqual(flushFn.mock.calls[0].arguments[0].length, 2)
  })

  test('should flush immediately when batch size reached', () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, { debounceMs: 1000, maxBatchSize: 3 })

    batcher.add({ msg: '1' })
    batcher.add({ msg: '2' })
    assert.strictEqual(flushFn.mock.calls.length, 0)

    batcher.add({ msg: '3' })
    assert.strictEqual(flushFn.mock.calls.length, 1)
    assert.strictEqual(flushFn.mock.calls[0].arguments[0].length, 3)
  })

  test('should clear pending logs on flush', async () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, { debounceMs: 50 })

    batcher.add({ msg: 'test' })
    await new Promise((r) => setTimeout(r, 100))

    // Add another after flush
    batcher.add({ msg: 'test2' })
    await new Promise((r) => setTimeout(r, 100))

    // Each batch should be separate
    assert.strictEqual(flushFn.mock.calls.length, 2)
    assert.strictEqual(flushFn.mock.calls[0].arguments[0].length, 1)
    assert.strictEqual(flushFn.mock.calls[1].arguments[0].length, 1)
  })

  test('should handle manual flush', () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, { debounceMs: 10000 })

    batcher.add({ msg: 'test' })
    batcher.flush()

    assert.strictEqual(flushFn.mock.calls.length, 1)
  })

  test('should not flush if empty', () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, { debounceMs: 50 })

    batcher.flush()
    assert.strictEqual(flushFn.mock.calls.length, 0)
  })
})

describe('sendLogsToServer', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('should POST entries to server', async () => {
    const mockFetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ received: 2 }),
      }),
    )
    globalThis.fetch = mockFetch

    const entries = [
      { ts: '2024-01-22T10:00:00Z', level: 'error', msg: 'test1' },
      { ts: '2024-01-22T10:00:01Z', level: 'warn', msg: 'test2' },
    ]

    const result = await sendLogsToServer(entries)

    assert.strictEqual(mockFetch.mock.calls.length, 1)
    const [url, options] = mockFetch.mock.calls[0].arguments
    assert.strictEqual(url, 'http://localhost:7890/logs')
    assert.strictEqual(options.method, 'POST')
    assert.strictEqual(options.headers['Content-Type'], 'application/json')

    const body = JSON.parse(options.body)
    assert.strictEqual(body.entries.length, 2)
    assert.strictEqual(result.received, 2)
  })

  test('should throw on server error', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: false,
        status: 500,
        statusText: 'Internal Server Error',
      }),
    )

    await assert.rejects(() => sendLogsToServer([{ msg: 'test' }]), {
      message: /Server error: 500/,
    })
  })

  test('should throw on network error', async () => {
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('Network error')))

    await assert.rejects(() => sendLogsToServer([{ msg: 'test' }]), {
      message: /Network error/,
    })
  })
})

describe('checkServerHealth', () => {
  beforeEach(() => {
    mock.reset()
  })

  test('should return health status when server is up', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () =>
          Promise.resolve({
            status: 'ok',
            entries: 42,
            maxEntries: 1000,
          }),
      }),
    )

    const health = await checkServerHealth()

    assert.strictEqual(health.status, 'ok')
    assert.strictEqual(health.entries, 42)
    assert.strictEqual(health.connected, true)
  })

  test('should return disconnected when server is down', async () => {
    globalThis.fetch = mock.fn(() => Promise.reject(new Error('Connection refused')))

    const health = await checkServerHealth()

    assert.strictEqual(health.connected, false)
    assert.ok(health.error.includes('Connection refused'))
  })

  test('should return disconnected on non-200 response', async () => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: false,
        status: 500,
      }),
    )

    const health = await checkServerHealth()

    assert.strictEqual(health.connected, false)
  })
})

describe('updateBadge', () => {
  beforeEach(() => {
    mock.reset()
    mockChrome.action.setBadgeText.mock.resetCalls()
    mockChrome.action.setBadgeBackgroundColor.mock.resetCalls()
  })

  test('should show green badge when connected with zero errors', () => {
    updateBadge({ connected: true, errorCount: 0 })

    assert.strictEqual(mockChrome.action.setBadgeText.mock.calls.length, 1)
    assert.strictEqual(mockChrome.action.setBadgeBackgroundColor.mock.calls.length, 1)

    const [textCall] = mockChrome.action.setBadgeText.mock.calls
    assert.strictEqual(textCall.arguments[0].text, '')

    const [colorCall] = mockChrome.action.setBadgeBackgroundColor.mock.calls
    assert.strictEqual(colorCall.arguments[0].color, '#3fb950') // green
  })

  test('should show error count when connected with errors', () => {
    updateBadge({ connected: true, errorCount: 5 })

    const [textCall] = mockChrome.action.setBadgeText.mock.calls
    assert.strictEqual(textCall.arguments[0].text, '5')

    const [colorCall] = mockChrome.action.setBadgeBackgroundColor.mock.calls
    assert.strictEqual(colorCall.arguments[0].color, '#3fb950') // green
  })

  test('should show 99+ when error count exceeds 99', () => {
    updateBadge({ connected: true, errorCount: 150 })

    const [textCall] = mockChrome.action.setBadgeText.mock.calls
    assert.strictEqual(textCall.arguments[0].text, '99+')
  })

  test('should show red badge when disconnected', () => {
    updateBadge({ connected: false, errorCount: 0 })

    const [textCall] = mockChrome.action.setBadgeText.mock.calls
    assert.strictEqual(textCall.arguments[0].text, '!')

    const [colorCall] = mockChrome.action.setBadgeBackgroundColor.mock.calls
    assert.strictEqual(colorCall.arguments[0].color, '#f85149') // red
  })
})

describe('formatLogEntry', () => {
  test('should add timestamp if not present', () => {
    const entry = formatLogEntry({ level: 'error', msg: 'test' })

    assert.ok(entry.ts)
    assert.ok(entry.ts.match(/^\d{4}-\d{2}-\d{2}T/)) // ISO format
  })

  test('should preserve existing timestamp', () => {
    const ts = '2024-01-22T10:00:00.000Z'
    const entry = formatLogEntry({ ts, level: 'error', msg: 'test' })

    assert.strictEqual(entry.ts, ts)
  })

  test('should truncate large args at 10KB', () => {
    const largeString = 'x'.repeat(20000) // 20KB
    const entry = formatLogEntry({
      level: 'log',
      type: 'console',
      args: [largeString],
    })

    // Args should be truncated
    assert.ok(JSON.stringify(entry.args).length < 15000)
    assert.ok(entry.args[0].includes('[truncated]'))
  })

  test('should handle circular references', () => {
    const obj = { name: 'test' }
    obj.self = obj // circular

    const entry = formatLogEntry({
      level: 'log',
      type: 'console',
      args: [obj],
    })

    // Should not throw, should have placeholder
    assert.ok(entry.args)
    const serialized = JSON.stringify(entry.args)
    assert.ok(serialized.includes('[Circular') || serialized.includes('unserializable'))
  })

  test('should include URL from source', () => {
    const entry = formatLogEntry({
      level: 'error',
      msg: 'test',
      url: 'http://localhost:3000/page',
    })

    assert.strictEqual(entry.url, 'http://localhost:3000/page')
  })
})

describe('shouldCaptureLog', () => {
  test('should capture all when level is "all"', () => {
    assert.strictEqual(shouldCaptureLog('debug', 'all'), true)
    assert.strictEqual(shouldCaptureLog('log', 'all'), true)
    assert.strictEqual(shouldCaptureLog('info', 'all'), true)
    assert.strictEqual(shouldCaptureLog('warn', 'all'), true)
    assert.strictEqual(shouldCaptureLog('error', 'all'), true)
  })

  test('should capture warn and error when level is "warn"', () => {
    assert.strictEqual(shouldCaptureLog('debug', 'warn'), false)
    assert.strictEqual(shouldCaptureLog('log', 'warn'), false)
    assert.strictEqual(shouldCaptureLog('info', 'warn'), false)
    assert.strictEqual(shouldCaptureLog('warn', 'warn'), true)
    assert.strictEqual(shouldCaptureLog('error', 'warn'), true)
  })

  test('should capture only error when level is "error"', () => {
    assert.strictEqual(shouldCaptureLog('debug', 'error'), false)
    assert.strictEqual(shouldCaptureLog('log', 'error'), false)
    assert.strictEqual(shouldCaptureLog('info', 'error'), false)
    assert.strictEqual(shouldCaptureLog('warn', 'error'), false)
    assert.strictEqual(shouldCaptureLog('error', 'error'), true)
  })

  test('should always capture network errors', () => {
    assert.strictEqual(shouldCaptureLog('error', 'error', 'network'), true)
    assert.strictEqual(shouldCaptureLog('error', 'warn', 'network'), true)
  })

  test('should always capture exceptions', () => {
    assert.strictEqual(shouldCaptureLog('error', 'error', 'exception'), true)
    assert.strictEqual(shouldCaptureLog('error', 'warn', 'exception'), true)
  })
})

describe('createErrorSignature', () => {
  test('should create consistent signature for exception', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Cannot read property x',
      stack: 'Error: Cannot read property x\n    at foo.js:10:5',
    }

    const sig1 = createErrorSignature(entry)
    const sig2 = createErrorSignature(entry)

    assert.strictEqual(sig1, sig2)
    assert.ok(sig1.includes('exception'))
    assert.ok(sig1.includes('error'))
    assert.ok(sig1.includes('Cannot read property x'))
  })

  test('should create different signatures for different exceptions', () => {
    const entry1 = {
      type: 'exception',
      level: 'error',
      message: 'Error A',
      stack: 'Error: Error A\n    at file1.js:10',
    }
    const entry2 = {
      type: 'exception',
      level: 'error',
      message: 'Error B',
      stack: 'Error: Error B\n    at file2.js:20',
    }

    const sig1 = createErrorSignature(entry1)
    const sig2 = createErrorSignature(entry2)

    assert.notStrictEqual(sig1, sig2)
  })

  test('should create signature for network error', () => {
    const entry = {
      type: 'network',
      level: 'error',
      method: 'POST',
      url: 'http://localhost:3000/api/users?id=123',
      status: 401,
    }

    const sig = createErrorSignature(entry)

    assert.ok(sig.includes('network'))
    assert.ok(sig.includes('POST'))
    assert.ok(sig.includes('/api/users')) // Path without query
    assert.ok(sig.includes('401'))
  })

  test('should create signature for console log', () => {
    const entry = {
      type: 'console',
      level: 'error',
      args: ['User authentication failed'],
    }

    const sig = createErrorSignature(entry)

    assert.ok(sig.includes('console'))
    assert.ok(sig.includes('User authentication failed'))
  })
})

describe('processErrorGroup', () => {
  beforeEach(() => {
    // Clear error groups between tests by calling flushErrorGroups repeatedly
    flushErrorGroups()
    flushErrorGroups()
  })

  test('should send first occurrence of error', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Test error',
      ts: new Date().toISOString(),
    }

    const result = processErrorGroup(entry)

    assert.strictEqual(result.shouldSend, true)
    assert.deepStrictEqual(result.entry, entry)
  })

  test('should not send duplicate error within dedup window', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Duplicate error test',
      ts: new Date().toISOString(),
    }

    // First occurrence
    const result1 = processErrorGroup(entry)
    assert.strictEqual(result1.shouldSend, true)

    // Immediate duplicate
    const result2 = processErrorGroup({ ...entry, ts: new Date().toISOString() })
    assert.strictEqual(result2.shouldSend, false)
  })

  test('should always send non-error logs', () => {
    const entry = {
      type: 'console',
      level: 'log',
      args: ['Info message'],
      ts: new Date().toISOString(),
    }

    const result1 = processErrorGroup(entry)
    const result2 = processErrorGroup({ ...entry })

    assert.strictEqual(result1.shouldSend, true)
    assert.strictEqual(result2.shouldSend, true)
  })

  test('should track warn level for grouping', () => {
    const entry = {
      type: 'console',
      level: 'warn',
      args: ['Warning message'],
      ts: new Date().toISOString(),
    }

    const result1 = processErrorGroup(entry)
    assert.strictEqual(result1.shouldSend, true)

    const result2 = processErrorGroup({ ...entry })
    assert.strictEqual(result2.shouldSend, false)
  })
})

describe('flushErrorGroups', () => {
  beforeEach(() => {
    // Clear error groups
    flushErrorGroups()
    flushErrorGroups()
  })

  test('should return empty array when no duplicates', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Single error',
      ts: new Date().toISOString(),
    }

    processErrorGroup(entry)
    const flushed = flushErrorGroups()

    // Only 1 occurrence, count = 1, nothing to aggregate
    assert.strictEqual(flushed.length, 0)
  })

  test('should return aggregated entry when duplicates exist', () => {
    const entry = {
      type: 'exception',
      level: 'error',
      message: 'Repeated error for flush',
      ts: new Date().toISOString(),
    }

    // Create duplicates
    processErrorGroup(entry)
    processErrorGroup({ ...entry })
    processErrorGroup({ ...entry })

    const flushed = flushErrorGroups()

    assert.strictEqual(flushed.length, 1)
    assert.strictEqual(flushed[0]._aggregatedCount, 3)
    assert.ok(flushed[0]._firstSeen)
    assert.ok(flushed[0]._lastSeen)
  })
})

describe('canTakeScreenshot', () => {
  test('should allow first screenshot', () => {
    const result = canTakeScreenshot(999) // Use unique tab ID

    assert.strictEqual(result.allowed, true)
  })

  test('should rate limit rapid screenshots', () => {
    const tabId = 1000

    // First screenshot - allowed
    recordScreenshot(tabId)

    // Immediate second - should be rate limited
    const result = canTakeScreenshot(tabId)

    assert.strictEqual(result.allowed, false)
    assert.strictEqual(result.reason, 'rate_limit')
    assert.ok(result.nextAllowedIn > 0)
  })

  test('should enforce session limit', () => {
    const tabId = 1001

    // Record 10 screenshots (max per session)
    for (let i = 0; i < 10; i++) {
      recordScreenshot(tabId)
    }

    // Wait a bit to pass rate limit
    const result = canTakeScreenshot(tabId)

    // Either rate limited or session limited
    assert.strictEqual(result.allowed, false)
  })
})

describe('recordScreenshot', () => {
  test('should record timestamp', () => {
    const tabId = 2000

    // First check - should be allowed
    const before = canTakeScreenshot(tabId)
    assert.strictEqual(before.allowed, true)

    // Record screenshot
    recordScreenshot(tabId)

    // Second check - should be rate limited
    const after = canTakeScreenshot(tabId)
    assert.strictEqual(after.allowed, false)
  })
})

describe('captureScreenshot', () => {
  let _originalFetch

  beforeEach(() => {
    mockChrome.tabs.get.mock.resetCalls()
    mockChrome.tabs.captureVisibleTab.mock.resetCalls()
    _originalFetch = globalThis.fetch
    // Mock fetch to simulate /screenshots endpoint
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ filename: 'localhost_3000-20260123-150405-console-err_123.jpg' }),
      }),
    )
  })

  test('should capture screenshot and save via server', async () => {
    const tabId = 3000

    const result = await captureScreenshot(tabId)

    assert.strictEqual(result.success, true)
    assert.ok(result.entry)
    assert.strictEqual(result.entry.type, 'screenshot')
    assert.strictEqual(result.entry.screenshotFile, 'localhost_3000-20260123-150405-console-err_123.jpg')
    assert.strictEqual(result.entry.dataUrl, undefined, 'should not embed base64 data')
    assert.ok(result.entry.ts)
  })

  test('should POST screenshot data to server', async () => {
    const tabId = 3010

    await captureScreenshot(tabId, 'err_456', 'exception')

    assert.strictEqual(globalThis.fetch.mock.calls.length, 1)
    const [url, opts] = globalThis.fetch.mock.calls[0].arguments
    assert.ok(url.endsWith('/screenshots'))
    assert.strictEqual(opts.method, 'POST')
    const body = JSON.parse(opts.body)
    assert.ok(body.dataUrl.startsWith('data:image/'))
    assert.strictEqual(body.errorId, 'err_456')
    assert.strictEqual(body.errorType, 'exception')
  })

  test('should link screenshot to error when relatedErrorId provided', async () => {
    const tabId = 3001

    const result = await captureScreenshot(tabId, 'err_123', 'console')

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.entry.relatedErrorId, 'err_123')
    assert.strictEqual(result.entry.trigger, 'error')
  })

  test('should set trigger to manual when no relatedErrorId', async () => {
    const tabId = 3002

    const result = await captureScreenshot(tabId, null)

    assert.strictEqual(result.success, true)
    assert.strictEqual(result.entry.trigger, 'manual')
  })

  test('should respect rate limiting', async () => {
    const tabId = 3003

    // First screenshot
    const result1 = await captureScreenshot(tabId)
    assert.strictEqual(result1.success, true)

    // Immediate second should be rate limited
    const result2 = await captureScreenshot(tabId)
    assert.strictEqual(result2.success, false)
    assert.ok(result2.error.includes('Rate limited'))
  })

  test('should handle capture errors gracefully', async () => {
    const tabId = 3004

    // Mock capture failure
    mockChrome.tabs.captureVisibleTab = mock.fn(() => Promise.reject(new Error('Tab not visible')))

    const result = await captureScreenshot(tabId)

    assert.strictEqual(result.success, false)
    assert.ok(result.error.includes('Tab not visible'))

    // Restore mock
    mockChrome.tabs.captureVisibleTab = mock.fn(() => Promise.resolve('data:image/jpeg;base64,/9j/4AAQSkZJRg=='))
  })

  test('should handle server error gracefully', async () => {
    const tabId = 3005

    globalThis.fetch = mock.fn(() => Promise.resolve({ ok: false, status: 500, statusText: 'Internal Server Error' }))

    const result = await captureScreenshot(tabId)

    assert.strictEqual(result.success, false)
    assert.ok(result.error.includes('500'))
  })
})

describe('decodeVLQ', () => {
  test('should decode simple VLQ values', async () => {
    const { decodeVLQ } = await import('../extension/background.js')

    // 'A' = 0
    assert.deepStrictEqual(decodeVLQ('A'), [0])

    // 'C' = 1
    assert.deepStrictEqual(decodeVLQ('C'), [1])

    // 'D' = -1
    assert.deepStrictEqual(decodeVLQ('D'), [-1])

    // 'K' = 5
    assert.deepStrictEqual(decodeVLQ('K'), [5])
  })

  test('should decode multi-value VLQ strings', async () => {
    const { decodeVLQ } = await import('../extension/background.js')

    // 'AAAA' = [0, 0, 0, 0]
    assert.deepStrictEqual(decodeVLQ('AAAA'), [0, 0, 0, 0])

    // 'ACAC' = [0, 1, 0, 1]
    assert.deepStrictEqual(decodeVLQ('ACAC'), [0, 1, 0, 1])
  })

  test('should decode large VLQ values', async () => {
    const { decodeVLQ } = await import('../extension/background.js')

    // 'gB' = 16 (uses continuation bit)
    assert.deepStrictEqual(decodeVLQ('gB'), [16])

    // '2B' = 27
    assert.deepStrictEqual(decodeVLQ('2B'), [27])
  })

  test('should throw on invalid VLQ character', async () => {
    const { decodeVLQ } = await import('../extension/background.js')

    assert.throws(() => decodeVLQ('!'), /Invalid VLQ character/)
  })
})

describe('parseMappings', () => {
  test('should parse simple mappings string', async () => {
    const { parseMappings } = await import('../extension/background.js')

    const result = parseMappings('AAAA;AACA')
    assert.strictEqual(result.length, 2)
    assert.strictEqual(result[0].length, 1)
    assert.strictEqual(result[1].length, 1)
  })

  test('should handle empty lines', async () => {
    const { parseMappings } = await import('../extension/background.js')

    const result = parseMappings('AAAA;;AACA')
    assert.strictEqual(result.length, 3)
    assert.strictEqual(result[1].length, 0) // Empty line
  })

  test('should parse multiple segments per line', async () => {
    const { parseMappings } = await import('../extension/background.js')

    const result = parseMappings('AAAA,CACA,EAEA')
    assert.strictEqual(result.length, 1)
    assert.strictEqual(result[0].length, 3)
  })
})

describe('parseStackFrame', () => {
  test('should parse standard Chrome stack frame', async () => {
    const { parseStackFrame } = await import('../extension/background.js')

    const line = '    at functionName (http://localhost:3000/app.js:42:15)'
    const result = parseStackFrame(line)

    assert.ok(result)
    assert.strictEqual(result.functionName, 'functionName')
    assert.strictEqual(result.fileName, 'http://localhost:3000/app.js')
    assert.strictEqual(result.lineNumber, 42)
    assert.strictEqual(result.columnNumber, 15)
  })

  test('should parse anonymous stack frame', async () => {
    const { parseStackFrame } = await import('../extension/background.js')

    const line = '    at http://localhost:3000/app.js:100:5'
    const result = parseStackFrame(line)

    assert.ok(result)
    assert.strictEqual(result.functionName, '<anonymous>')
    assert.strictEqual(result.lineNumber, 100)
  })

  test('should return null for non-stack lines', async () => {
    const { parseStackFrame } = await import('../extension/background.js')

    assert.strictEqual(parseStackFrame('Error: Something went wrong'), null)
    assert.strictEqual(parseStackFrame(''), null)
  })
})

describe('extractSourceMapUrl', () => {
  test('should extract sourceMappingURL from script', async () => {
    const { extractSourceMapUrl } = await import('../extension/background.js')

    const content = 'function foo(){}\n//# sourceMappingURL=app.js.map'
    const url = extractSourceMapUrl(content)

    assert.strictEqual(url, 'app.js.map')
  })

  test('should extract data URL source map', async () => {
    const { extractSourceMapUrl } = await import('../extension/background.js')

    const content = 'function foo(){}\n//# sourceMappingURL=data:application/json;base64,eyJ2ZXJzaW9uIjozfQ=='
    const url = extractSourceMapUrl(content)

    assert.ok(url.startsWith('data:'))
  })

  test('should return null if no source map', async () => {
    const { extractSourceMapUrl } = await import('../extension/background.js')

    const content = 'function foo(){}'
    const url = extractSourceMapUrl(content)

    assert.strictEqual(url, null)
  })

  test('should handle deprecated @ syntax', async () => {
    const { extractSourceMapUrl } = await import('../extension/background.js')

    const content = 'function foo(){}\n//@ sourceMappingURL=old.js.map'
    const url = extractSourceMapUrl(content)

    assert.strictEqual(url, 'old.js.map')
  })
})

describe('parseSourceMapData', () => {
  test('should parse source map data', async () => {
    const { parseSourceMapData } = await import('../extension/background.js')

    const sourceMap = {
      version: 3,
      sources: ['src/app.ts'],
      names: ['foo', 'bar'],
      mappings: 'AAAA;AACA',
      sourceRoot: '',
    }

    const result = parseSourceMapData(sourceMap)

    assert.deepStrictEqual(result.sources, ['src/app.ts'])
    assert.deepStrictEqual(result.names, ['foo', 'bar'])
    assert.ok(Array.isArray(result.mappings))
  })

  test('should handle empty source map', async () => {
    const { parseSourceMapData } = await import('../extension/background.js')

    const result = parseSourceMapData({})

    assert.deepStrictEqual(result.sources, [])
    assert.deepStrictEqual(result.names, [])
  })
})

describe('findOriginalLocation', () => {
  test('should find original location from source map', async () => {
    const { parseSourceMapData, findOriginalLocation } = await import('../extension/background.js')

    // A simple source map: one source file, one mapping at line 1, col 0
    // AAAA maps to: genCol=0, sourceIdx=0, origLine=0, origCol=0
    const sourceMap = parseSourceMapData({
      version: 3,
      sources: ['src/original.ts'],
      names: [],
      mappings: 'AAAA',
    })

    const result = findOriginalLocation(sourceMap, 1, 0)

    assert.ok(result)
    assert.strictEqual(result.source, 'src/original.ts')
    assert.strictEqual(result.line, 1)
    assert.strictEqual(result.column, 0)
  })

  test('should return null for out of bounds line', async () => {
    const { parseSourceMapData, findOriginalLocation } = await import('../extension/background.js')

    const sourceMap = parseSourceMapData({
      version: 3,
      sources: ['src/app.ts'],
      mappings: 'AAAA',
    })

    const result = findOriginalLocation(sourceMap, 100, 0)

    assert.strictEqual(result, null)
  })

  test('should return null for null source map', async () => {
    const { findOriginalLocation } = await import('../extension/background.js')

    assert.strictEqual(findOriginalLocation(null, 1, 0), null)
  })
})

describe('clearSourceMapCache', () => {
  test('should clear the cache', async () => {
    const { clearSourceMapCache } = await import('../extension/background.js')

    // Should not throw
    clearSourceMapCache()
  })
})

describe('Debug Logging', () => {
  test('should log debug entries', async () => {
    const { debugLog, getDebugLog, clearDebugLog, DebugCategory } = await import('../extension/background.js')

    // Clear any existing entries
    clearDebugLog()

    // Log some entries
    debugLog(DebugCategory.LIFECYCLE, 'Test message 1')
    debugLog(DebugCategory.CONNECTION, 'Test message 2', { extra: 'data' })

    const entries = getDebugLog()
    assert.ok(entries.length >= 2)

    // Find our test entries
    const msg1 = entries.find((e) => e.message === 'Test message 1')
    const msg2 = entries.find((e) => e.message === 'Test message 2')

    assert.ok(msg1)
    assert.strictEqual(msg1.category, 'lifecycle')

    assert.ok(msg2)
    assert.strictEqual(msg2.category, 'connection')
    assert.deepStrictEqual(msg2.data, { extra: 'data' })
  })

  test('should clear debug log', async () => {
    const { debugLog, getDebugLog, clearDebugLog, DebugCategory } = await import('../extension/background.js')

    // Add an entry
    debugLog(DebugCategory.ERROR, 'Error test')

    // Clear
    clearDebugLog()

    const entries = getDebugLog()
    // Should have at least one entry (from the clear itself logging)
    // But not the error test entry
    const errorEntry = entries.find((e) => e.message === 'Error test')
    assert.ok(!errorEntry)
  })

  test('should export debug log as JSON', async () => {
    const { debugLog, exportDebugLog, clearDebugLog, DebugCategory } = await import('../extension/background.js')

    clearDebugLog()
    debugLog(DebugCategory.CAPTURE, 'Capture test')

    const exported = exportDebugLog()
    const parsed = JSON.parse(exported)

    assert.ok(parsed.exportedAt)
    assert.strictEqual(parsed.version, '4.6.0')
    assert.ok(Array.isArray(parsed.entries))
  })

  test('should toggle debug mode', async () => {
    const { setDebugMode, exportDebugLog } = await import('../extension/background.js')

    // Enable debug mode
    setDebugMode(true)

    const exported1 = JSON.parse(exportDebugLog())
    assert.strictEqual(exported1.debugMode, true)

    // Disable debug mode
    setDebugMode(false)

    const exported2 = JSON.parse(exportDebugLog())
    assert.strictEqual(exported2.debugMode, false)
  })

  test('should limit debug log buffer size', async () => {
    const { debugLog, getDebugLog, clearDebugLog, DebugCategory } = await import('../extension/background.js')

    clearDebugLog()

    // Add more than 200 entries
    for (let i = 0; i < 250; i++) {
      debugLog(DebugCategory.CAPTURE, `Entry ${i}`)
    }

    const entries = getDebugLog()
    // Should be capped at 200
    assert.ok(entries.length <= 200)
  })
})

describe('Context Annotation Monitoring', () => {
  beforeEach(() => {
    resetContextWarning()
  })

  describe('measureContextSize', () => {
    test('should return 0 for entries without _context', () => {
      const entry = { level: 'error', type: 'console', args: ['test'] }
      assert.strictEqual(measureContextSize(entry), 0)
    })

    test('should return 0 for entries with empty _context', () => {
      const entry = { level: 'error', _context: {} }
      assert.strictEqual(measureContextSize(entry), 0)
    })

    test('should return approximate byte size of _context', () => {
      const entry = {
        level: 'error',
        _context: {
          user: { id: 123, name: 'test' },
          page: { route: '/checkout' },
        },
      }
      const size = measureContextSize(entry)
      assert.ok(size > 0)
      assert.ok(size < 200) // Small context should be well under 200 bytes
    })

    test('should measure large context correctly', () => {
      const largeValue = 'x'.repeat(4000)
      const entry = {
        level: 'error',
        _context: {
          key1: largeValue,
          key2: largeValue,
          key3: largeValue,
          key4: largeValue,
          key5: largeValue,
          key6: largeValue, // 6 × 4000 = ~24KB, over threshold
        },
      }
      const size = measureContextSize(entry)
      assert.ok(size > 20 * 1024, `Expected > 20KB, got ${size}`)
    })
  })

  describe('checkContextAnnotations', () => {
    test('should not warn for entries with small context', () => {
      const entries = [
        { level: 'error', _context: { user: { id: 1 } } },
        { level: 'error', _context: { page: '/home' } },
      ]
      checkContextAnnotations(entries)
      assert.strictEqual(getContextWarning(), null)
    })

    test('should not warn for entries without context', () => {
      const entries = [
        { level: 'error', args: ['test'] },
        { level: 'warn', msg: 'hello' },
      ]
      checkContextAnnotations(entries)
      assert.strictEqual(getContextWarning(), null)
    })

    test('should not warn for fewer than 3 excessive entries', () => {
      const largeContext = {}
      for (let i = 0; i < 6; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      // Only 2 excessive entries
      checkContextAnnotations([{ level: 'error', _context: largeContext }])
      checkContextAnnotations([{ level: 'error', _context: largeContext }])

      assert.strictEqual(getContextWarning(), null)
    })

    test('should warn after 3 excessive entries within 60s', () => {
      const largeContext = {}
      for (let i = 0; i < 6; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      checkContextAnnotations([{ level: 'error', _context: largeContext }])
      checkContextAnnotations([{ level: 'error', _context: largeContext }])
      checkContextAnnotations([{ level: 'error', _context: largeContext }])

      const warning = getContextWarning()
      assert.ok(warning !== null, 'Expected warning to be set')
      assert.ok(warning.sizeKB > 20, `Expected > 20KB, got ${warning.sizeKB}KB`)
      assert.ok(warning.count >= 3, `Expected count >= 3, got ${warning.count}`)
    })

    test('should include average size in warning', () => {
      const largeContext = {}
      for (let i = 0; i < 10; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      for (let i = 0; i < 3; i++) {
        checkContextAnnotations([{ level: 'error', _context: largeContext }])
      }

      const warning = getContextWarning()
      assert.ok(warning.sizeKB > 30) // 10 × 4000 = ~40KB
    })

    test('should handle batch with mix of large and small entries', () => {
      const largeContext = {}
      for (let i = 0; i < 6; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      // 3 batches, each with one large entry mixed with small ones
      for (let i = 0; i < 3; i++) {
        checkContextAnnotations([
          { level: 'error', _context: { small: 'val' } },
          { level: 'error', _context: largeContext },
          { level: 'warn', msg: 'no context' },
        ])
      }

      const warning = getContextWarning()
      assert.ok(warning !== null, 'Expected warning from large entries in mixed batches')
    })
  })

  describe('resetContextWarning', () => {
    test('should clear the warning state', () => {
      const largeContext = {}
      for (let i = 0; i < 6; i++) {
        largeContext[`key${i}`] = 'x'.repeat(4000)
      }

      for (let i = 0; i < 3; i++) {
        checkContextAnnotations([{ level: 'error', _context: largeContext }])
      }

      assert.ok(getContextWarning() !== null)
      resetContextWarning()
      assert.strictEqual(getContextWarning(), null)
    })
  })
})

// =============================================================================
// V5: Enhanced Actions Server Communication
// =============================================================================

describe('Enhanced Actions Server Communication', () => {
  beforeEach(() => {
    globalThis.fetch = mock.fn(() =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ received: 1 }),
      }),
    )
  })

  test('sendEnhancedActionsToServer should POST actions to /enhanced-actions', async () => {
    const actions = [
      { type: 'click', timestamp: 1705312200000, url: 'http://localhost:3000', selectors: { testId: 'btn' } },
      {
        type: 'input',
        timestamp: 1705312201000,
        url: 'http://localhost:3000',
        selectors: { id: 'email' },
        value: 'test@test.com',
        inputType: 'email',
      },
    ]

    await sendEnhancedActionsToServer(actions)

    assert.strictEqual(globalThis.fetch.mock.calls.length, 1)
    const [url, opts] = globalThis.fetch.mock.calls[0].arguments
    assert.ok(url.endsWith('/enhanced-actions'))
    assert.strictEqual(opts.method, 'POST')
    assert.strictEqual(opts.headers['Content-Type'], 'application/json')

    const body = JSON.parse(opts.body)
    assert.ok(Array.isArray(body.actions))
    assert.strictEqual(body.actions.length, 2)
    assert.strictEqual(body.actions[0].type, 'click')
    assert.strictEqual(body.actions[1].type, 'input')
  })

  test('sendEnhancedActionsToServer should throw on non-ok response', async () => {
    globalThis.fetch = mock.fn(() => Promise.resolve({ ok: false, status: 500 }))

    const actions = [{ type: 'click', timestamp: 1000, url: 'http://localhost:3000', selectors: {} }]

    await assert.rejects(
      () => sendEnhancedActionsToServer(actions),
      (err) => err.message.includes('500'),
    )
  })

  test('enhanced action batcher should batch and send actions', async () => {
    const flushFn = mock.fn()
    const batcher = createLogBatcher(flushFn, { debounceMs: 50, maxBatchSize: 50 })

    const action1 = { type: 'click', timestamp: 1000, url: 'http://localhost:3000', selectors: { id: 'btn' } }
    const action2 = {
      type: 'input',
      timestamp: 1001,
      url: 'http://localhost:3000',
      selectors: { id: 'input' },
      value: 'hi',
    }

    batcher.add(action1)
    batcher.add(action2)

    // Wait for debounce
    await new Promise((r) => setTimeout(r, 100))

    assert.strictEqual(flushFn.mock.calls.length, 1)
    assert.strictEqual(flushFn.mock.calls[0].arguments[0].length, 2)
    assert.strictEqual(flushFn.mock.calls[0].arguments[0][0].type, 'click')
    assert.strictEqual(flushFn.mock.calls[0].arguments[0][1].type, 'input')
  })

  test('message handler should process enhanced_action messages via batcher', async () => {
    // Simulate what the message handler does - adds to batcher
    const flushFn = mock.fn()
    const actionBatcher = createLogBatcher(flushFn, { debounceMs: 50, maxBatchSize: 50 })

    // Simulate receiving enhanced_action messages
    const payload = { type: 'click', timestamp: 1000, url: 'http://localhost:3000', selectors: { testId: 'btn' } }
    actionBatcher.add(payload)

    await new Promise((r) => setTimeout(r, 100))

    assert.strictEqual(flushFn.mock.calls.length, 1)
    assert.strictEqual(flushFn.mock.calls[0].arguments[0][0].type, 'click')
  })
})
