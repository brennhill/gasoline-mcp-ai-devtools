// @ts-nocheck
/**
 * @fileoverview Tests for Web Vitals capture (FCP, LCP, CLS, TTFB)
 * TDD: These tests are written BEFORE implementation
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Mock PerformanceObserver
class MockPerformanceObserver {
  constructor(callback) {
    MockPerformanceObserver._instances.push(this)
    this._callback = callback
    this._types = []
  }
  observe(options) {
    if (options.type) this._types.push(options.type)
  }
  disconnect() {
    this._disconnected = true
  }
  // Helper to simulate entries
  _emit(entries) {
    this._callback({ getEntries: () => entries })
  }
}
MockPerformanceObserver._instances = []
MockPerformanceObserver.supportedEntryTypes = ['paint', 'largest-contentful-paint', 'layout-shift']

let originalWindow, originalPerformanceObserver, originalPerformance

describe('Web Vitals Capture', () => {
  beforeEach(() => {
    MockPerformanceObserver._instances = []
    originalWindow = globalThis.window
    originalPerformanceObserver = globalThis.PerformanceObserver
    originalPerformance = globalThis.performance

    globalThis.PerformanceObserver = MockPerformanceObserver
    globalThis.performance = {
      getEntriesByType: mock.fn(() => []),
      now: () => 1000,
    }
    globalThis.window = {
      postMessage: mock.fn(),
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      location: { href: 'http://localhost:3000/test' },
      history: { pushState: mock.fn(), replaceState: mock.fn() },
      onerror: null,
      onunhandledrejection: null,
      WebSocket: class {
        addEventListener() {}
      },
    }
    globalThis.document = { readyState: 'complete', addEventListener: mock.fn() }
    // Preserve console but mock it to avoid inject.js wrapping issues
    if (!globalThis.console) {
      globalThis.console = { log: mock.fn(), warn: mock.fn(), error: mock.fn(), info: mock.fn(), debug: mock.fn() }
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.PerformanceObserver = originalPerformanceObserver
    globalThis.performance = originalPerformance
    delete globalThis.document
  })

  test('startWebVitalsCapture creates observers for FCP, LCP, CLS', async () => {
    const mod = await import('../extension/inject.js')
    mod.startWebVitalsCapture()

    // Should create observers for paint (FCP), largest-contentful-paint (LCP), layout-shift (CLS)
    const types = MockPerformanceObserver._instances.flatMap((obs) => obs._types)
    assert.ok(types.includes('paint'), 'Should observe paint entries (FCP)')
    assert.ok(types.includes('largest-contentful-paint'), 'Should observe LCP entries')
    assert.ok(types.includes('layout-shift'), 'Should observe CLS entries')
  })

  test('FCP is captured from paint entries', async () => {
    const mod = await import('../extension/inject.js')
    mod.resetWebVitals()
    mod.startWebVitalsCapture()

    // Find the paint observer
    const paintObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('paint'))
    assert.ok(paintObs, 'Paint observer should exist')

    // Emit a first-contentful-paint entry
    paintObs._emit([{ name: 'first-contentful-paint', startTime: 1200 }])

    const vitals = mod.getWebVitals()
    assert.ok(vitals.fcp, 'FCP should be captured')
    assert.strictEqual(vitals.fcp.value, 1200)
  })

  test('LCP updates to latest entry', async () => {
    const mod = await import('../extension/inject.js')
    mod.resetWebVitals()
    mod.startWebVitalsCapture()

    const lcpObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('largest-contentful-paint'))
    assert.ok(lcpObs, 'LCP observer should exist')

    // Emit multiple LCP entries - should keep the last one
    lcpObs._emit([
      { startTime: 1000, element: { tagName: 'DIV' }, size: 5000 },
      { startTime: 2400, element: { tagName: 'IMG' }, size: 150000 },
    ])

    const vitals = mod.getWebVitals()
    assert.ok(vitals.lcp, 'LCP should be captured')
    assert.strictEqual(vitals.lcp.value, 2400)
    assert.strictEqual(vitals.lcp.element, 'IMG')
    assert.strictEqual(vitals.lcp.size, 150000)
  })

  test('CLS accumulates layout shifts (ignores input-driven)', async () => {
    const mod = await import('../extension/inject.js')
    mod.resetWebVitals()
    mod.startWebVitalsCapture()

    const clsObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('layout-shift'))
    assert.ok(clsObs, 'CLS observer should exist')

    // Emit layout shifts - one input-driven (should be ignored)
    clsObs._emit([
      { value: 0.02, hadRecentInput: false },
      { value: 0.03, hadRecentInput: false },
      { value: 0.1, hadRecentInput: true }, // Should be ignored
    ])

    const vitals = mod.getWebVitals()
    assert.ok(vitals.cls, 'CLS should be captured')
    assert.strictEqual(vitals.cls.value, 0.05) // 0.02 + 0.03
    assert.strictEqual(vitals.cls.shifts, 2) // Only non-input shifts
  })

  test('TTFB captured from navigation timing', async () => {
    globalThis.performance.getEntriesByType = mock.fn(() => [
      {
        requestStart: 100,
        responseStart: 280,
      },
    ])

    const mod = await import('../extension/inject.js')
    mod.resetWebVitals()
    mod.startWebVitalsCapture()

    const vitals = mod.getWebVitals()
    assert.ok(vitals.ttfb, 'TTFB should be captured')
    assert.strictEqual(vitals.ttfb.value, 180) // 280 - 100
  })

  test('observer error does not crash', async () => {
    // Make PerformanceObserver throw for one type
    let callCount = 0
    globalThis.PerformanceObserver = class {
      constructor(cb) {
        this._cb = cb
      }
      observe() {
        callCount++
        if (callCount === 1) throw new Error('Not supported')
      }
      disconnect() {}
    }
    globalThis.PerformanceObserver.supportedEntryTypes = []

    const mod = await import('../extension/inject.js')
    // Should not throw
    mod.startWebVitalsCapture()
    const vitals = mod.getWebVitals()
    // FCP observer failed, but others should still work
    assert.strictEqual(vitals.fcp, null)
  })

  test('resetWebVitals clears all values', async () => {
    const mod = await import('../extension/inject.js')
    mod.resetWebVitals()
    mod.startWebVitalsCapture()

    // Set some values via observers
    const paintObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('paint'))
    if (paintObs) paintObs._emit([{ name: 'first-contentful-paint', startTime: 500 }])

    mod.resetWebVitals()
    const vitals = mod.getWebVitals()
    assert.strictEqual(vitals.fcp, null)
    assert.strictEqual(vitals.lcp, null)
    assert.strictEqual(vitals.cls, null)
    assert.strictEqual(vitals.ttfb, null)
  })

  test('getWebVitals returns a copy', async () => {
    const mod = await import('../extension/inject.js')
    mod.resetWebVitals()

    const vitals1 = mod.getWebVitals()
    const vitals2 = mod.getWebVitals()
    assert.notStrictEqual(vitals1, vitals2) // Different objects
  })

  test('stopWebVitalsCapture disconnects all observers', async () => {
    const mod = await import('../extension/inject.js')
    mod.startWebVitalsCapture()

    const observersBefore = MockPerformanceObserver._instances.filter((obs) => !obs._disconnected)
    assert.ok(observersBefore.length > 0, 'Should have active observers')

    mod.stopWebVitalsCapture()

    const observersAfter = MockPerformanceObserver._instances.filter((obs) => obs._disconnected)
    assert.ok(observersAfter.length > 0, 'All observers should be disconnected')
  })
})

describe('Web Vitals Message Flow', () => {
  beforeEach(() => {
    MockPerformanceObserver._instances = []
    originalWindow = globalThis.window
    originalPerformanceObserver = globalThis.PerformanceObserver
    originalPerformance = globalThis.performance

    globalThis.PerformanceObserver = MockPerformanceObserver
    globalThis.performance = {
      getEntriesByType: mock.fn(() => []),
      now: () => 1000,
    }
    globalThis.window = {
      postMessage: mock.fn(),
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
      location: { href: 'http://localhost:3000/test' },
      history: { pushState: mock.fn(), replaceState: mock.fn() },
      onerror: null,
      onunhandledrejection: null,
      WebSocket: class {
        addEventListener() {}
      },
    }
    globalThis.document = { readyState: 'complete', addEventListener: mock.fn() }
    // Preserve console but mock it to avoid inject.js wrapping issues
    if (!globalThis.console) {
      globalThis.console = { log: mock.fn(), warn: mock.fn(), error: mock.fn(), info: mock.fn(), debug: mock.fn() }
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.PerformanceObserver = originalPerformanceObserver
    globalThis.performance = originalPerformance
    delete globalThis.document
  })

  test('sendWebVitals posts GASOLINE_WEB_VITALS message', async () => {
    const mod = await import('../extension/inject.js')
    mod.resetWebVitals()
    mod.startWebVitalsCapture()

    // Simulate FCP capture
    const paintObs = MockPerformanceObserver._instances.find((obs) => obs._types.includes('paint'))
    if (paintObs) paintObs._emit([{ name: 'first-contentful-paint', startTime: 800 }])

    mod.sendWebVitals()

    const calls = globalThis.window.postMessage.mock.calls
    const vitalsMessage = calls.find((c) => c.arguments[0]?.type === 'GASOLINE_WEB_VITALS')
    assert.ok(vitalsMessage, 'Should post GASOLINE_WEB_VITALS message')
    assert.ok(vitalsMessage.arguments[0].payload.vitals.fcp, 'Message should include FCP data')
  })
})
