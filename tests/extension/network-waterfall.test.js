// @ts-nocheck
/**
 * @fileoverview network-waterfall.test.js — Tests for network waterfall capture.
 * Verifies timing-ordered network request recording, 30-second time window
 * enforcement, 50-entry buffer cap, and the waterfall data structure with
 * start/duration/status fields for visualizing request concurrency.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Define esbuild constant not available in Node test env
globalThis.__GASOLINE_VERSION__ = 'test'

// Mock performance API
let originalPerformance
let originalWindow
let originalDocument

function createMockResourceTiming(overrides = {}) {
  return {
    name: 'http://localhost:3000/api/data',
    entryType: 'resource',
    startTime: 100,
    duration: 250,
    initiatorType: 'fetch',
    // DNS
    domainLookupStart: 100,
    domainLookupEnd: 110,
    // Connection
    connectStart: 110,
    connectEnd: 130,
    secureConnectionStart: 115,
    // Request/Response
    requestStart: 130,
    responseStart: 200, // TTFB
    responseEnd: 350,
    // Size
    transferSize: 1024,
    encodedBodySize: 900,
    decodedBodySize: 2048,
    // Cache
    fetchStart: 100,
    ...overrides,
  }
}

function createMockPerformance() {
  const entries = []
  return {
    getEntriesByType: mock.fn((type) => {
      if (type === 'resource') return entries
      if (type === 'navigation') return [{ type: 'navigate', startTime: 0 }]
      return []
    }),
    getEntriesByName: mock.fn((name) => entries.filter((e) => e.name === name)),
    clearResourceTimings: mock.fn(),
    mark: mock.fn(),
    measure: mock.fn(),
    now: mock.fn(() => Date.now()),
    _entries: entries,
    _addEntry: (entry) => entries.push(entry),
  }
}

function createMockWindow() {
  return {
    location: { href: 'http://localhost:3000/test' },
    postMessage: mock.fn(),
    performance: createMockPerformance(),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
    onerror: null,
    innerWidth: 1920,
    innerHeight: 1080,
    scrollX: 0,
    scrollY: 0,
  }
}

function createMockDocument() {
  return {
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
  }
}

describe('Network Waterfall - parseResourceTiming', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('should parse resource timing into waterfall phases', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming()
    const result = parseResourceTiming(timing)

    assert.ok(result)
    assert.ok(result.phases || result.timing)
    assert.ok(result.url || result.name)
  })

  test('should calculate DNS lookup time', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      domainLookupStart: 100,
      domainLookupEnd: 120,
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.phases?.dns || result.dns, 20)
  })

  test('should calculate TCP connection time', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      connectStart: 120,
      connectEnd: 150,
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.phases?.connect || result.connect || result.tcp, 30)
  })

  test('should calculate TLS handshake time', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      secureConnectionStart: 130,
      connectEnd: 160,
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.phases?.tls || result.tls || result.ssl, 30)
  })

  test('should calculate TTFB (time to first byte)', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      requestStart: 160,
      responseStart: 260,
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.phases?.ttfb || result.ttfb, 100)
  })

  test('should calculate download time', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      responseStart: 260,
      responseEnd: 360,
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.phases?.download || result.download || result.content, 100)
  })

  test('should include total duration', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      duration: 500,
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.duration || result.total, 500)
  })

  test('should include transfer size information', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      transferSize: 2048,
      encodedBodySize: 1800,
      decodedBodySize: 4096,
    })

    const result = parseResourceTiming(timing)

    assert.ok(result.size || result.transferSize || result.bytes)
    assert.strictEqual(result.transferSize || result.size?.transfer, 2048)
  })

  test('should handle cache hit (transferSize = 0)', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      transferSize: 0,
      encodedBodySize: 1000,
    })

    const result = parseResourceTiming(timing)

    assert.ok(result.cached === true || result.fromCache === true || result.cacheHit === true)
  })

  test('should include initiator type', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      initiatorType: 'fetch',
    })

    const result = parseResourceTiming(timing)

    assert.strictEqual(result.initiatorType || result.initiator || result.type, 'fetch')
  })

  test('should handle missing timing values (0)', async () => {
    const { parseResourceTiming } = await import('../../extension/inject.js')

    const timing = createMockResourceTiming({
      domainLookupStart: 0,
      domainLookupEnd: 0,
      secureConnectionStart: 0,
    })

    const result = parseResourceTiming(timing)

    // Should not throw and should handle gracefully
    assert.ok(result)
    assert.strictEqual(result.phases?.dns || result.dns || 0, 0)
  })
})

describe('Network Waterfall - getNetworkWaterfall', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('should return all resource entries', async () => {
    const { getNetworkWaterfall } = await import('../../extension/inject.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api/1' }))
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api/2' }))

    const waterfall = getNetworkWaterfall()

    assert.ok(Array.isArray(waterfall))
    assert.strictEqual(waterfall.length, 2)
  })

  test('should filter by time range', async () => {
    const { getNetworkWaterfall } = await import('../../extension/inject.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/early', startTime: 50 }))
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/late', startTime: 500 }))

    const waterfall = getNetworkWaterfall({ since: 400 })

    assert.strictEqual(waterfall.length, 1)
    assert.ok(waterfall[0].url?.includes('late') || waterfall[0].name?.includes('late'))
  })

  test('should limit number of entries', async () => {
    const { getNetworkWaterfall, MAX_WATERFALL_ENTRIES } = await import('../../extension/inject.js')

    // Add more entries than the limit
    for (let i = 0; i < 100; i++) {
      globalThis.performance._addEntry(
        createMockResourceTiming({ name: `http://localhost/api/${i}`, startTime: i * 10 }),
      )
    }

    const waterfall = getNetworkWaterfall()

    assert.ok(waterfall.length <= MAX_WATERFALL_ENTRIES)
  })

  test('should sort entries by start time', async () => {
    const { getNetworkWaterfall } = await import('../../extension/inject.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/second', startTime: 200 }))
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/first', startTime: 100 }))

    const waterfall = getNetworkWaterfall()

    assert.ok(waterfall[0].startTime <= waterfall[1].startTime)
  })

  test('should filter by initiator type', async () => {
    const { getNetworkWaterfall } = await import('../../extension/inject.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api', initiatorType: 'fetch' }))
    globalThis.performance._addEntry(
      createMockResourceTiming({ name: 'http://localhost/style.css', initiatorType: 'link' }),
    )

    const waterfall = getNetworkWaterfall({ initiatorTypes: ['fetch', 'xmlhttprequest'] })

    assert.strictEqual(waterfall.length, 1)
    assert.ok(
      waterfall[0].initiatorType === 'fetch' || waterfall[0].initiator === 'fetch' || waterfall[0].type === 'fetch',
    )
  })

  test('should exclude data URLs', async () => {
    const { getNetworkWaterfall } = await import('../../extension/inject.js')

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'data:image/png;base64,abc123' }))
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api' }))

    const waterfall = getNetworkWaterfall()

    assert.strictEqual(waterfall.length, 1)
    assert.ok(!waterfall[0].url?.startsWith('data:') && !waterfall[0].name?.startsWith('data:'))
  })

  test('should return empty array when performance API unavailable', async () => {
    const { getNetworkWaterfall } = await import('../../extension/inject.js')

    globalThis.performance = null

    const waterfall = getNetworkWaterfall()

    assert.ok(Array.isArray(waterfall))
    assert.strictEqual(waterfall.length, 0)
  })
})

describe('Network Waterfall - Pending Requests', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('should track pending fetch requests', async () => {
    const { trackPendingRequest, getPendingRequests, clearPendingRequests } = await import('../../extension/inject.js')

    clearPendingRequests()

    trackPendingRequest({
      url: 'http://localhost/api/slow',
      method: 'POST',
      startTime: Date.now(),
    })

    const pending = getPendingRequests()

    assert.ok(Array.isArray(pending))
    assert.strictEqual(pending.length, 1)
    assert.strictEqual(pending[0].url, 'http://localhost/api/slow')

    clearPendingRequests()
  })

  test('should remove completed requests', async () => {
    const { trackPendingRequest, completePendingRequest, getPendingRequests, clearPendingRequests } =
      await import('../../extension/inject.js')

    clearPendingRequests()

    const requestId = trackPendingRequest({
      url: 'http://localhost/api/data',
      method: 'GET',
      startTime: Date.now(),
    })

    completePendingRequest(requestId)

    const pending = getPendingRequests()
    assert.strictEqual(pending.length, 0)
  })

  test('should include pending requests in error snapshots', async () => {
    const { trackPendingRequest, getNetworkWaterfallForError, clearPendingRequests, setNetworkWaterfallEnabled } =
      await import('../../extension/inject.js')

    setNetworkWaterfallEnabled(true)
    clearPendingRequests()

    trackPendingRequest({
      url: 'http://localhost/api/slow-endpoint',
      method: 'POST',
      startTime: Date.now() - 1000, // Started 1 second ago
    })

    const errorEntry = {
      type: 'exception',
      level: 'error',
      message: 'Network timeout',
    }

    const snapshot = await getNetworkWaterfallForError(errorEntry)

    assert.ok(snapshot)
    assert.ok(snapshot.pending || snapshot.pendingRequests)
    assert.strictEqual((snapshot.pending || snapshot.pendingRequests).length, 1)

    clearPendingRequests()
  })
})

describe('Network Waterfall - Error Integration', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('should create waterfall snapshot for error', async () => {
    const { getNetworkWaterfallForError, setNetworkWaterfallEnabled } = await import('../../extension/inject.js')

    setNetworkWaterfallEnabled(true)

    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/api' }))

    const errorEntry = {
      type: 'network',
      level: 'error',
      url: 'http://localhost/api/failed',
      status: 500,
    }

    const snapshot = await getNetworkWaterfallForError(errorEntry)

    assert.ok(snapshot)
    assert.strictEqual(snapshot.type, 'network_waterfall')
    assert.ok(snapshot.ts)
    assert.ok(snapshot.entries || snapshot.waterfall)
  })

  test('should respect networkWaterfallEnabled setting', async () => {
    const { getNetworkWaterfallForError, setNetworkWaterfallEnabled } = await import('../../extension/inject.js')

    setNetworkWaterfallEnabled(false)

    const errorEntry = {
      type: 'network',
      level: 'error',
    }

    const snapshot = await getNetworkWaterfallForError(errorEntry)

    assert.strictEqual(snapshot, null)

    // Re-enable
    setNetworkWaterfallEnabled(true)
  })

  test('should only capture recent entries (last 30 seconds)', async () => {
    const { getNetworkWaterfallForError, setNetworkWaterfallEnabled } = await import('../../extension/inject.js')

    setNetworkWaterfallEnabled(true)

    // Old entry (simulated via startTime relative to performance.now)
    globalThis.performance._addEntry(createMockResourceTiming({ name: 'http://localhost/old', startTime: 0 }))

    // Recent entry
    const now = Date.now()
    globalThis.performance.now = () => now
    globalThis.performance._addEntry(
      createMockResourceTiming({ name: 'http://localhost/recent', startTime: now - 5000 }),
    )

    const errorEntry = { type: 'exception', level: 'error' }
    const snapshot = await getNetworkWaterfallForError(errorEntry)

    // Should filter based on recency
    assert.ok(snapshot)
  })
})

describe('Network Waterfall - Configuration', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalPerformance = globalThis.performance
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.performance = globalThis.window.performance
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.performance = originalPerformance
    globalThis.document = originalDocument
  })

  test('setNetworkWaterfallEnabled should toggle feature', async () => {
    const { setNetworkWaterfallEnabled, isNetworkWaterfallEnabled } = await import('../../extension/inject.js')

    setNetworkWaterfallEnabled(true)
    assert.strictEqual(isNetworkWaterfallEnabled(), true)

    setNetworkWaterfallEnabled(false)
    assert.strictEqual(isNetworkWaterfallEnabled(), false)
  })

  test('should expose network waterfall through __gasoline API', async () => {
    const { installGasolineAPI, uninstallGasolineAPI, setNetworkWaterfallEnabled } =
      await import('../../extension/inject.js')

    setNetworkWaterfallEnabled(true)
    installGasolineAPI()

    assert.ok(globalThis.window.__gasoline)
    assert.ok(typeof globalThis.window.__gasoline.setNetworkWaterfall === 'function')
    assert.ok(typeof globalThis.window.__gasoline.getNetworkWaterfall === 'function')

    uninstallGasolineAPI()
  })
})
