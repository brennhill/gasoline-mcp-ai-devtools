/**
 * @fileoverview Tests for WebSocket interception, adaptive sampling, schema detection, and binary handling
 * TDD: These tests are written BEFORE implementation (v4 feature)
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Mock window and crypto for browser environment
const createMockWindow = () => ({
  postMessage: mock.fn(),
  addEventListener: mock.fn(),
  WebSocket: class MockWebSocket {
    constructor(url, protocols) {
      this.url = url
      this.protocols = protocols
      this.readyState = 0
      this._listeners = {}
    }
    addEventListener(event, handler) {
      if (!this._listeners[event]) this._listeners[event] = []
      this._listeners[event].push(handler)
    }
    send(data) {}
    close(code, reason) {}
    // Helper for tests to simulate events
    _emit(event, data) {
      if (this._listeners[event]) {
        this._listeners[event].forEach(h => h(data))
      }
    }
  }
})

const createMockCrypto = () => ({
  randomUUID: mock.fn(() => 'test-uuid-' + Math.random().toString(36).slice(2))
})

let originalWindow, originalCrypto

describe('WebSocket Interception', () => {
  beforeEach(async () => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow()
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
    // Reset mode to lifecycle (default)
    const { setWebSocketCaptureMode } = await import('../extension/inject.js')
    setWebSocketCaptureMode('lifecycle')
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should replace WebSocket constructor', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../extension/inject.js')

    const OriginalWS = globalThis.window.WebSocket
    installWebSocketCapture()

    assert.notStrictEqual(globalThis.window.WebSocket, OriginalWS)

    uninstallWebSocketCapture()
    assert.strictEqual(globalThis.window.WebSocket, OriginalWS)
  })

  test('should preserve WebSocket.prototype', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../extension/inject.js')

    const OriginalWS = globalThis.window.WebSocket
    installWebSocketCapture()

    assert.strictEqual(globalThis.window.WebSocket.prototype, OriginalWS.prototype)

    uninstallWebSocketCapture()
  })

  test('should create WebSocket with correct URL and protocols', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../extension/inject.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws', ['chat'])
    assert.strictEqual(ws.url, 'wss://example.com/ws')

    uninstallWebSocketCapture()
  })

  test('should emit ws:open event on connection open', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../extension/inject.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    const calls = globalThis.window.postMessage.mock.calls
    const openMessage = calls.find(c => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_WS' && msg.payload.event === 'open'
    })

    assert.ok(openMessage, 'Expected ws:open event to be posted')
    assert.strictEqual(openMessage.arguments[0].payload.url, 'wss://example.com/ws')
    assert.ok(openMessage.arguments[0].payload.id, 'Expected connection ID')

    uninstallWebSocketCapture()
  })

  test('should emit ws:close event with code and reason', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../extension/inject.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('close', { code: 1000, reason: 'normal closure' })

    const calls = globalThis.window.postMessage.mock.calls
    const closeMessage = calls.find(c => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_WS' && msg.payload.event === 'close'
    })

    assert.ok(closeMessage, 'Expected ws:close event to be posted')
    assert.strictEqual(closeMessage.arguments[0].payload.code, 1000)
    assert.strictEqual(closeMessage.arguments[0].payload.reason, 'normal closure')

    uninstallWebSocketCapture()
  })

  test('should emit ws:error event', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../extension/inject.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('error', {})

    const calls = globalThis.window.postMessage.mock.calls
    const errorMessage = calls.find(c => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_WS' && msg.payload.event === 'error'
    })

    assert.ok(errorMessage, 'Expected ws:error event to be posted')

    uninstallWebSocketCapture()
  })

  test('should track incoming messages', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } = await import('../extension/inject.js')

    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('message', { data: '{"type":"chat","msg":"hello"}' })

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvent = calls.find(c => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_WS' && msg.payload.event === 'message' && msg.payload.direction === 'incoming'
    })

    assert.ok(msgEvent, 'Expected incoming message event')
    assert.strictEqual(msgEvent.arguments[0].payload.data, '{"type":"chat","msg":"hello"}')
    assert.strictEqual(msgEvent.arguments[0].payload.size, 29)

    uninstallWebSocketCapture()
  })

  test('should intercept outgoing messages via send()', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } = await import('../extension/inject.js')

    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws.send('{"type":"ping"}')

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvent = calls.find(c => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_WS' && msg.payload.event === 'message' && msg.payload.direction === 'outgoing'
    })

    assert.ok(msgEvent, 'Expected outgoing message event')
    assert.strictEqual(msgEvent.arguments[0].payload.data, '{"type":"ping"}')

    uninstallWebSocketCapture()
  })

  test('should call original send() after interception', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../extension/inject.js')

    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    const originalSend = mock.fn()
    ws.send = originalSend
    // Re-install after replacing send (the wrapper wraps the current send)
    // Actually the wrapper should have already wrapped it

    uninstallWebSocketCapture()
  })

  test('should assign unique ID per connection', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture } = await import('../extension/inject.js')

    installWebSocketCapture()

    const ws1 = new globalThis.window.WebSocket('wss://a.com/ws')
    const ws2 = new globalThis.window.WebSocket('wss://b.com/ws')
    ws1._emit('open', {})
    ws2._emit('open', {})

    const calls = globalThis.window.postMessage.mock.calls
    const openEvents = calls
      .filter(c => c.arguments[0].type === 'GASOLINE_WS' && c.arguments[0].payload.event === 'open')
      .map(c => c.arguments[0].payload.id)

    assert.strictEqual(openEvents.length, 2)
    assert.notStrictEqual(openEvents[0], openEvents[1], 'Expected unique IDs per connection')

    uninstallWebSocketCapture()
  })

  test('should not emit message events in lifecycle mode', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } = await import('../extension/inject.js')

    setWebSocketCaptureMode('lifecycle')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})
    ws._emit('message', { data: '{"type":"chat","msg":"hello"}' })
    ws.send('{"type":"pong"}')

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvents = calls.filter(c => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_WS' && msg.payload.event === 'message'
    })

    assert.strictEqual(msgEvents.length, 0, 'Expected no message events in lifecycle mode')

    // But lifecycle events should still be emitted
    const openEvents = calls.filter(c => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_WS' && msg.payload.event === 'open'
    })
    assert.strictEqual(openEvents.length, 1, 'Expected lifecycle events to still be emitted')

    uninstallWebSocketCapture()
  })

  test('should truncate message data at 4KB', async () => {
    const { installWebSocketCapture, uninstallWebSocketCapture, setWebSocketCaptureMode } = await import('../extension/inject.js')

    setWebSocketCaptureMode('messages')
    installWebSocketCapture()

    const ws = new globalThis.window.WebSocket('wss://example.com/ws')
    ws._emit('open', {})

    const largeData = 'x'.repeat(5000)
    ws._emit('message', { data: largeData })

    const calls = globalThis.window.postMessage.mock.calls
    const msgEvent = calls.find(c => {
      const msg = c.arguments[0]
      return msg.type === 'GASOLINE_WS' && msg.payload.event === 'message'
    })

    assert.ok(msgEvent.arguments[0].payload.data.length <= 4096, 'Expected data truncated to 4KB')
    assert.strictEqual(msgEvent.arguments[0].payload.truncated, true)

    uninstallWebSocketCapture()
  })
})

describe('Adaptive Sampling', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow()
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should log every message when rate < 10 msg/s', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Simulate 5 messages in 1 second (5 msg/s)
    for (let i = 0; i < 5; i++) {
      tracker.recordMessage('incoming')
    }

    assert.strictEqual(tracker.shouldSample('incoming'), true)
  })

  test('should sample at ~10 msg/s when rate is 10-50 msg/s', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Simulate 30 messages in 1 second (30 msg/s)
    tracker.setMessageRate(30)

    // Should sample approximately 1 in 3 (targeting 10/s)
    let sampled = 0
    for (let i = 0; i < 30; i++) {
      if (tracker.shouldSample('incoming')) sampled++
    }

    // Should be approximately 10 (tolerance: 5-15)
    assert.ok(sampled >= 5 && sampled <= 15, `Expected ~10 sampled messages, got ${sampled}`)
  })

  test('should sample at ~5 msg/s when rate is 50-200 msg/s', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(100)

    let sampled = 0
    for (let i = 0; i < 100; i++) {
      if (tracker.shouldSample('incoming')) sampled++
    }

    // Should be approximately 5 (tolerance: 2-10)
    assert.ok(sampled >= 2 && sampled <= 10, `Expected ~5 sampled messages, got ${sampled}`)
  })

  test('should sample at ~2 msg/s when rate > 200 msg/s', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(500)

    let sampled = 0
    for (let i = 0; i < 500; i++) {
      if (tracker.shouldSample('incoming')) sampled++
    }

    // Should be approximately 2 (tolerance: 1-5)
    assert.ok(sampled >= 1 && sampled <= 5, `Expected ~2 sampled messages, got ${sampled}`)
  })

  test('should always log first 5 messages on a new connection', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(100) // High rate - would normally sample

    let sampled = 0
    for (let i = 0; i < 5; i++) {
      tracker.recordMessage('incoming')
      if (tracker.shouldSample('incoming')) sampled++
    }

    assert.strictEqual(sampled, 5, 'Expected all first 5 messages to be sampled')
  })

  test('should always log lifecycle events regardless of rate', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(500)

    // Lifecycle events should always be logged
    assert.strictEqual(tracker.shouldLogLifecycle(), true)
  })

  test('should include sampling info when sampling is active', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(50)

    const info = tracker.getSamplingInfo()

    assert.ok(info.rate, 'Expected rate in sampling info')
    assert.ok(info.logged, 'Expected logged ratio in sampling info')
    assert.ok(info.window, 'Expected window in sampling info')
  })

  test('should use rolling 5-second window for rate calculation', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Record messages over time
    for (let i = 0; i < 50; i++) {
      tracker.recordMessage('incoming')
    }

    const rate = tracker.getMessageRate()
    assert.ok(rate > 0, 'Expected positive rate')
  })
})

describe('Schema Detection', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow()
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should detect JSON schema from first 5 messages', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Feed 5 consistent JSON messages
    for (let i = 0; i < 5; i++) {
      tracker.recordMessage('incoming', JSON.stringify({ sym: 'AAPL', price: 185 + i, vol: 1000 }))
    }

    const schema = tracker.getSchema()
    assert.deepStrictEqual(schema.detectedKeys.sort(), ['price', 'sym', 'vol'])
    assert.strictEqual(schema.consistent, true)
  })

  test('should detect schema inconsistency', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Feed mixed JSON messages
    tracker.recordMessage('incoming', JSON.stringify({ type: 'message', text: 'hello' }))
    tracker.recordMessage('incoming', JSON.stringify({ type: 'message', text: 'world' }))
    tracker.recordMessage('incoming', JSON.stringify({ type: 'typing', user: 'alice' }))
    tracker.recordMessage('incoming', JSON.stringify({ type: 'presence', status: 'online' }))
    tracker.recordMessage('incoming', JSON.stringify({ type: 'message', text: 'again' }))

    const schema = tracker.getSchema()
    assert.strictEqual(schema.consistent, false)
  })

  test('should log schema-change messages even when sampling', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    tracker.setMessageRate(100) // High rate - sampling active

    // Establish schema
    for (let i = 0; i < 5; i++) {
      tracker.recordMessage('incoming', JSON.stringify({ sym: 'AAPL', price: 185 }))
    }

    // This message has different keys - should be logged
    const shouldLog = tracker.isSchemaChange(JSON.stringify({ error: 'rate_limit', code: 429 }))
    assert.strictEqual(shouldLog, true, 'Expected schema change to be flagged for logging')
  })

  test('should not detect schema from non-JSON messages', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    for (let i = 0; i < 5; i++) {
      tracker.recordMessage('incoming', 'plain text message ' + i)
    }

    const schema = tracker.getSchema()
    assert.strictEqual(schema.detectedKeys, null, 'Expected no keys for non-JSON')
  })

  test('should track schema variants', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    // Simulate many messages with known variants
    for (let i = 0; i < 89; i++) {
      tracker.recordMessage('incoming', JSON.stringify({ type: 'message', user: 'u', text: 't' }))
    }
    for (let i = 0; i < 8; i++) {
      tracker.recordMessage('incoming', JSON.stringify({ type: 'typing', user: 'u' }))
    }
    for (let i = 0; i < 3; i++) {
      tracker.recordMessage('incoming', JSON.stringify({ type: 'presence', status: 'on' }))
    }

    const schema = tracker.getSchema()
    assert.ok(schema.variants, 'Expected variants to be tracked')
    assert.ok(schema.variants.length >= 2, 'Expected at least 2 variants')
  })
})

describe('Binary Message Handling', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow()
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should format small binary as hex preview', async () => {
    const { formatPayload } = await import('../extension/inject.js')

    // Binary < 256 bytes: hex preview
    const buffer = new ArrayBuffer(128)
    const view = new Uint8Array(buffer)
    for (let i = 0; i < 128; i++) view[i] = i

    const formatted = formatPayload(buffer)

    assert.ok(formatted.startsWith('[Binary: 128B]'), `Expected binary prefix, got: ${formatted}`)
    assert.ok(formatted.includes('000102'), 'Expected hex content')
  })

  test('should format large binary with magic bytes only', async () => {
    const { formatPayload } = await import('../extension/inject.js')

    // Binary >= 256 bytes: size + magic bytes
    const buffer = new ArrayBuffer(4096)
    const view = new Uint8Array(buffer)
    view[0] = 0x0a; view[1] = 0x1b; view[2] = 0x2c; view[3] = 0x3d

    const formatted = formatPayload(buffer)

    assert.ok(formatted.startsWith('[Binary: 4096B, magic:'), `Expected large binary format, got: ${formatted}`)
    assert.ok(formatted.includes('0a1b2c3d'), 'Expected magic bytes')
  })

  test('should pass through JSON text as-is', async () => {
    const { formatPayload } = await import('../extension/inject.js')

    const json = '{"type":"chat","msg":"hello"}'
    const formatted = formatPayload(json)

    assert.strictEqual(formatted, json)
  })

  test('should pass through non-JSON text as-is', async () => {
    const { formatPayload } = await import('../extension/inject.js')

    const text = 'Hello, world!'
    const formatted = formatPayload(text)

    assert.strictEqual(formatted, text)
  })

  test('should handle Blob binary data', async () => {
    const { formatPayload } = await import('../extension/inject.js')

    // Simulate Blob (in test environment, use object with size)
    const blob = { size: 1024, type: 'application/octet-stream', arrayBuffer: () => Promise.resolve(new ArrayBuffer(1024)) }

    const formatted = formatPayload(blob)

    assert.ok(formatted.includes('Binary'), 'Expected binary indicator for Blob')
    assert.ok(formatted.includes('1024'), 'Expected size in output')
  })

  test('should compute correct size for text messages', async () => {
    const { getSize } = await import('../extension/inject.js')

    assert.strictEqual(getSize('hello'), 5)
    assert.strictEqual(getSize('{"type":"chat"}'), 15)
  })

  test('should compute correct size for ArrayBuffer', async () => {
    const { getSize } = await import('../extension/inject.js')

    const buffer = new ArrayBuffer(256)
    assert.strictEqual(getSize(buffer), 256)
  })

  test('should compute correct size for Blob', async () => {
    const { getSize } = await import('../extension/inject.js')

    const blob = { size: 4096 }
    assert.strictEqual(getSize(blob), 4096)
  })
})

describe('WebSocket Message Truncation', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow()
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should truncate text messages longer than 4KB', async () => {
    const { truncateWsMessage } = await import('../extension/inject.js')

    const longMessage = 'x'.repeat(5000)
    const result = truncateWsMessage(longMessage)

    assert.ok(result.data.length <= 4096)
    assert.strictEqual(result.truncated, true)
  })

  test('should not truncate messages within 4KB', async () => {
    const { truncateWsMessage } = await import('../extension/inject.js')

    const shortMessage = '{"type":"chat","msg":"hello"}'
    const result = truncateWsMessage(shortMessage)

    assert.strictEqual(result.data, shortMessage)
    assert.strictEqual(result.truncated, false)
  })
})

describe('Connection Stats', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalCrypto = globalThis.crypto
    globalThis.window = createMockWindow()
    Object.defineProperty(globalThis, 'crypto', { value: createMockCrypto(), writable: true, configurable: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
    Object.defineProperty(globalThis, 'crypto', { value: originalCrypto, writable: true, configurable: true })
  })

  test('should track incoming message count and bytes', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    tracker.recordMessage('incoming', '{"msg":"hello"}') // 15 bytes
    tracker.recordMessage('incoming', '{"msg":"world"}') // 15 bytes

    assert.strictEqual(tracker.stats.incoming.count, 2)
    assert.strictEqual(tracker.stats.incoming.bytes, 30)
  })

  test('should track outgoing message count and bytes', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    tracker.recordMessage('outgoing', '{"type":"ping"}') // 15 bytes

    assert.strictEqual(tracker.stats.outgoing.count, 1)
    assert.strictEqual(tracker.stats.outgoing.bytes, 15)
  })

  test('should track last message preview', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')

    tracker.recordMessage('incoming', '{"type":"first"}')
    tracker.recordMessage('incoming', '{"type":"second"}')

    assert.strictEqual(tracker.stats.incoming.lastPreview, '{"type":"second"}')
  })

  test('should truncate preview to 200 chars', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    const longMessage = 'x'.repeat(300)

    tracker.recordMessage('incoming', longMessage)

    assert.ok(tracker.stats.incoming.lastPreview.length <= 200)
  })

  test('should track last message timestamp', async () => {
    const { createConnectionTracker } = await import('../extension/inject.js')

    const tracker = createConnectionTracker('test-id', 'wss://example.com')
    const before = Date.now()

    tracker.recordMessage('incoming', '{"msg":"test"}')

    assert.ok(tracker.stats.incoming.lastAt >= before)
    assert.ok(tracker.stats.incoming.lastAt <= Date.now())
  })
})
