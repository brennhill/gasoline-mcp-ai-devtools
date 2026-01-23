/**
 * @fileoverview Tests for inject script (page capture logic)
 * TDD: These tests are written BEFORE implementation
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Mock window and document for browser environment
const createMockWindow = () => ({
  postMessage: mock.fn(),
  addEventListener: mock.fn(),
  removeEventListener: mock.fn(),
  location: { href: 'http://localhost:3000/test' },
  onerror: null,
  onunhandledrejection: null,
})

const createMockConsole = () => ({
  log: mock.fn(),
  warn: mock.fn(),
  error: mock.fn(),
  info: mock.fn(),
  debug: mock.fn(),
})

// Store original
let originalWindow
let originalConsole

describe('Console Capture', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    globalThis.window = createMockWindow()
    globalThis.console = createMockConsole()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
  })

  test('should intercept console.log', async () => {
    // Import the module to install interceptors
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../extension/inject.js')

    const originalLog = globalThis.console.log
    installConsoleCapture()

    // Call console.log
    globalThis.console.log('test message', { data: 123 })

    // Should have posted message
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'DEV_CONSOLE_LOG')
    assert.strictEqual(message.payload.level, 'log')
    assert.deepStrictEqual(message.payload.args, ['test message', { data: 123 }])

    // Should have called original
    assert.strictEqual(originalLog.mock.calls.length, 1)

    uninstallConsoleCapture()
  })

  test('should intercept console.error', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../extension/inject.js')

    installConsoleCapture()
    globalThis.console.error('error message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'error')

    uninstallConsoleCapture()
  })

  test('should intercept console.warn', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../extension/inject.js')

    installConsoleCapture()
    globalThis.console.warn('warning message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'warn')

    uninstallConsoleCapture()
  })

  test('should intercept console.info', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../extension/inject.js')

    installConsoleCapture()
    globalThis.console.info('info message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'info')

    uninstallConsoleCapture()
  })

  test('should intercept console.debug', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../extension/inject.js')

    installConsoleCapture()
    globalThis.console.debug('debug message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'debug')

    uninstallConsoleCapture()
  })

  test('should include page URL', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../extension/inject.js')

    installConsoleCapture()
    globalThis.console.log('test')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.url, 'http://localhost:3000/test')

    uninstallConsoleCapture()
  })

  test('should restore original console on uninstall', async () => {
    const { installConsoleCapture, uninstallConsoleCapture } = await import('../extension/inject.js')

    const originalLog = globalThis.console.log
    installConsoleCapture()

    // Console should be wrapped
    assert.notStrictEqual(globalThis.console.log, originalLog)

    uninstallConsoleCapture()

    // Console should be restored
    assert.strictEqual(globalThis.console.log, originalLog)
  })
})

describe('Network Capture', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should capture fetch errors (status >= 400)', async () => {
    const { wrapFetch } = await import('../extension/inject.js')

    const mockResponse = {
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      clone: () => ({
        text: () => Promise.resolve(JSON.stringify({ error: 'Invalid credentials' })),
      }),
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    const startTime = Date.now()
    await wrappedFetch('http://localhost:8789/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email: 'test@test.com' }),
    })

    // Should have posted network error
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 1)

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'DEV_CONSOLE_LOG')
    assert.strictEqual(message.payload.type, 'network')
    assert.strictEqual(message.payload.level, 'error')
    assert.strictEqual(message.payload.status, 401)
    assert.strictEqual(message.payload.method, 'POST')
    assert.strictEqual(message.payload.url, 'http://localhost:8789/auth/login')
    assert.ok(message.payload.duration >= 0)
  })

  test('should not capture successful fetch requests', async () => {
    const { wrapFetch } = await import('../extension/inject.js')

    const mockResponse = {
      ok: true,
      status: 200,
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    await wrappedFetch('http://localhost:8789/api/data')

    // Should NOT have posted message
    assert.strictEqual(globalThis.window.postMessage.mock.calls.length, 0)
  })

  test('should capture 5xx server errors', async () => {
    const { wrapFetch } = await import('../extension/inject.js')

    const mockResponse = {
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      clone: () => ({
        text: () => Promise.resolve('Server error'),
      }),
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    await wrappedFetch('http://localhost:8789/api/data')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.status, 500)
  })

  test('should capture network failures', async () => {
    const { wrapFetch } = await import('../extension/inject.js')

    const originalFetch = mock.fn(() => Promise.reject(new Error('Failed to fetch')))
    const wrappedFetch = wrapFetch(originalFetch)

    try {
      await wrappedFetch('http://localhost:8789/api/data')
    } catch {
      // Expected
    }

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.type, 'network')
    assert.strictEqual(message.payload.level, 'error')
    assert.ok(message.payload.error.includes('Failed to fetch'))
  })

  test('should exclude Authorization header from logs', async () => {
    const { wrapFetch } = await import('../extension/inject.js')

    const mockResponse = {
      ok: false,
      status: 401,
      statusText: 'Unauthorized',
      clone: () => ({
        text: () => Promise.resolve('{}'),
      }),
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    await wrappedFetch('http://localhost:8789/api/data', {
      headers: {
        Authorization: 'Bearer secret-token',
        'Content-Type': 'application/json',
      },
    })

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments

    // Should not include Authorization
    assert.ok(!JSON.stringify(message).includes('secret-token'))
    assert.ok(!JSON.stringify(message).includes('Authorization'))
  })

  test('should truncate large response bodies', async () => {
    const { wrapFetch } = await import('../extension/inject.js')

    const largeBody = 'x'.repeat(10000) // 10KB
    const mockResponse = {
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      clone: () => ({
        text: () => Promise.resolve(largeBody),
      }),
    }

    const originalFetch = mock.fn(() => Promise.resolve(mockResponse))
    const wrappedFetch = wrapFetch(originalFetch)

    await wrappedFetch('http://localhost:8789/api/data')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.ok(message.payload.response.length < 6000) // 5KB limit + some buffer
  })
})

describe('Exception Capture', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should capture window.onerror events', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../extension/inject.js')

    installExceptionCapture()

    // Simulate error event
    globalThis.window.onerror("Cannot read property 'x' of undefined", 'app.js', 42, 15, new Error())

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.type, 'DEV_CONSOLE_LOG')
    assert.strictEqual(message.payload.type, 'exception')
    assert.strictEqual(message.payload.level, 'error')
    assert.strictEqual(message.payload.message, "Cannot read property 'x' of undefined")
    assert.strictEqual(message.payload.filename, 'app.js')
    assert.strictEqual(message.payload.lineno, 42)
    assert.strictEqual(message.payload.colno, 15)

    uninstallExceptionCapture()
  })

  test('should capture unhandled promise rejections', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../extension/inject.js')

    installExceptionCapture()

    // Get the handler that was registered
    const addListenerCalls = globalThis.window.addEventListener.mock.calls
    const rejectionHandler = addListenerCalls.find(
      (call) => call.arguments[0] === 'unhandledrejection'
    )

    assert.ok(rejectionHandler, 'Should have registered unhandledrejection handler')

    // Simulate rejection event
    const handler = rejectionHandler.arguments[1]
    handler({
      reason: new Error('Promise rejection'),
    })

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.type, 'exception')
    assert.strictEqual(message.payload.level, 'error')
    assert.ok(message.payload.message.includes('Promise rejection'))

    uninstallExceptionCapture()
  })

  test('should include stack trace', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../extension/inject.js')

    installExceptionCapture()

    const error = new Error('Test error')
    error.stack = 'Error: Test error\n    at foo (app.js:42)\n    at bar (app.js:100)'

    globalThis.window.onerror('Test error', 'app.js', 42, 1, error)

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.ok(message.payload.stack.includes('app.js:42'))
    assert.ok(message.payload.stack.includes('app.js:100'))

    uninstallExceptionCapture()
  })

  test('should handle error event without error object', async () => {
    const { installExceptionCapture, uninstallExceptionCapture } = await import('../extension/inject.js')

    installExceptionCapture()

    // Some browsers don't provide error object
    globalThis.window.onerror('Script error', '', 0, 0, null)

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.message, 'Script error')
    assert.ok(message.payload.stack === '' || message.payload.stack === undefined)

    uninstallExceptionCapture()
  })
})

describe('Safe Serialization', () => {
  test('should handle circular references in objects', async () => {
    const { safeSerialize } = await import('../extension/inject.js')

    const obj = { name: 'test' }
    obj.self = obj

    const result = safeSerialize(obj)

    // Should not throw
    assert.ok(result)
    JSON.stringify(result) // Should be serializable
  })

  test('should handle undefined and null', async () => {
    const { safeSerialize } = await import('../extension/inject.js')

    assert.strictEqual(safeSerialize(undefined), undefined)
    assert.strictEqual(safeSerialize(null), null)
  })

  test('should preserve primitive types', async () => {
    const { safeSerialize } = await import('../extension/inject.js')

    assert.strictEqual(safeSerialize('string'), 'string')
    assert.strictEqual(safeSerialize(123), 123)
    assert.strictEqual(safeSerialize(true), true)
  })

  test('should handle Error objects', async () => {
    const { safeSerialize } = await import('../extension/inject.js')

    const error = new Error('test error')
    const result = safeSerialize(error)

    assert.ok(result.message === 'test error' || result.includes('test error'))
  })

  test('should truncate strings over 10KB', async () => {
    const { safeSerialize } = await import('../extension/inject.js')

    const longString = 'x'.repeat(15000)
    const result = safeSerialize(longString)

    assert.ok(result.length < 12000) // 10KB + truncation message
    assert.ok(result.includes('[truncated]'))
  })

  test('should handle functions by converting to string', async () => {
    const { safeSerialize } = await import('../extension/inject.js')

    const fn = function testFunc() {}
    const result = safeSerialize(fn)

    assert.ok(typeof result === 'string')
    assert.ok(result.includes('function') || result.includes('[Function'))
  })

  test('should handle DOM elements', async () => {
    const { safeSerialize } = await import('../extension/inject.js')

    // Mock a DOM element
    const element = {
      nodeType: 1,
      tagName: 'DIV',
      id: 'test',
      className: 'foo bar',
    }

    const result = safeSerialize(element)

    // Should have some representation
    assert.ok(result)
  })

  test('should handle deeply nested objects', async () => {
    const { safeSerialize } = await import('../extension/inject.js')

    let obj = { value: 'deep' }
    for (let i = 0; i < 50; i++) {
      obj = { nested: obj }
    }

    const result = safeSerialize(obj)

    // Should not throw, should truncate depth
    assert.ok(result)
  })
})

describe('Context Annotations', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    globalThis.window = createMockWindow()
    globalThis.console = createMockConsole()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
  })

  test('should set and get context annotation', async () => {
    const {
      setContextAnnotation,
      getContextAnnotations,
      clearContextAnnotations,
    } = await import('../extension/inject.js')

    clearContextAnnotations()

    setContextAnnotation('checkout-flow', { step: 'payment', items: 3 })

    const context = getContextAnnotations()
    assert.ok(context)
    assert.strictEqual(context['checkout-flow'].step, 'payment')
    assert.strictEqual(context['checkout-flow'].items, 3)

    clearContextAnnotations()
  })

  test('should remove context annotation', async () => {
    const {
      setContextAnnotation,
      removeContextAnnotation,
      getContextAnnotations,
      clearContextAnnotations,
    } = await import('../extension/inject.js')

    clearContextAnnotations()

    setContextAnnotation('user', { id: 'usr_123' })
    assert.ok(getContextAnnotations()['user'])

    removeContextAnnotation('user')
    const context = getContextAnnotations()
    assert.ok(!context || !context['user'])

    clearContextAnnotations()
  })

  test('should clear all annotations', async () => {
    const {
      setContextAnnotation,
      clearContextAnnotations,
      getContextAnnotations,
    } = await import('../extension/inject.js')

    setContextAnnotation('a', 1)
    setContextAnnotation('b', 2)

    clearContextAnnotations()

    const context = getContextAnnotations()
    assert.ok(context === null)
  })

  test('should reject empty key', async () => {
    const { setContextAnnotation, clearContextAnnotations } = await import('../extension/inject.js')

    clearContextAnnotations()

    const result = setContextAnnotation('', 'value')
    assert.strictEqual(result, false)
  })

  test('should reject non-string key', async () => {
    const { setContextAnnotation, clearContextAnnotations } = await import('../extension/inject.js')

    clearContextAnnotations()

    const result = setContextAnnotation(123, 'value')
    assert.strictEqual(result, false)
  })

  test('should reject key longer than 100 chars', async () => {
    const { setContextAnnotation, clearContextAnnotations } = await import('../extension/inject.js')

    clearContextAnnotations()

    const longKey = 'x'.repeat(101)
    const result = setContextAnnotation(longKey, 'value')
    assert.strictEqual(result, false)
  })

  test('should truncate large values', async () => {
    const { setContextAnnotation, getContextAnnotations, clearContextAnnotations } = await import(
      '../extension/inject.js'
    )

    clearContextAnnotations()

    const largeValue = { data: 'x'.repeat(5000) }
    const result = setContextAnnotation('large', largeValue)

    // Should return false or store truncated
    assert.ok(result === false || getContextAnnotations()['large'] === '[Value too large]')

    clearContextAnnotations()
  })

  test('should include context in error logs', async () => {
    const {
      installConsoleCapture,
      uninstallConsoleCapture,
      setContextAnnotation,
      clearContextAnnotations,
    } = await import('../extension/inject.js')

    clearContextAnnotations()
    setContextAnnotation('checkout', { step: 'payment' })

    installConsoleCapture()
    globalThis.console.error('Payment failed')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'error')
    assert.ok(message.payload._context)
    assert.strictEqual(message.payload._context.checkout.step, 'payment')

    uninstallConsoleCapture()
    clearContextAnnotations()
  })

  test('should not include context in non-error logs', async () => {
    const {
      installConsoleCapture,
      uninstallConsoleCapture,
      setContextAnnotation,
      clearContextAnnotations,
    } = await import('../extension/inject.js')

    clearContextAnnotations()
    setContextAnnotation('checkout', { step: 'payment' })

    installConsoleCapture()
    globalThis.console.log('Info message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'log')
    assert.ok(!message.payload._context)

    uninstallConsoleCapture()
    clearContextAnnotations()
  })
})

describe('Gasoline API', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = createMockWindow()
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should install window.__gasoline API', async () => {
    const { installGasolineAPI, uninstallGasolineAPI } = await import('../extension/inject.js')

    installGasolineAPI()

    assert.ok(globalThis.window.__gasoline)
    assert.ok(typeof globalThis.window.__gasoline.annotate === 'function')
    assert.ok(typeof globalThis.window.__gasoline.removeAnnotation === 'function')
    assert.ok(typeof globalThis.window.__gasoline.clearAnnotations === 'function')
    assert.ok(typeof globalThis.window.__gasoline.getContext === 'function')
    assert.ok(globalThis.window.__gasoline.version)

    uninstallGasolineAPI()
  })

  test('should uninstall window.__gasoline API', async () => {
    const { installGasolineAPI, uninstallGasolineAPI } = await import('../extension/inject.js')

    installGasolineAPI()
    assert.ok(globalThis.window.__gasoline)

    uninstallGasolineAPI()
    assert.ok(!globalThis.window.__gasoline)
  })

  test('__gasoline.annotate should work', async () => {
    const { installGasolineAPI, uninstallGasolineAPI, clearContextAnnotations } = await import(
      '../extension/inject.js'
    )

    clearContextAnnotations()
    installGasolineAPI()

    const result = globalThis.window.__gasoline.annotate('test', { value: 123 })
    assert.strictEqual(result, true)

    const context = globalThis.window.__gasoline.getContext()
    assert.strictEqual(context.test.value, 123)

    uninstallGasolineAPI()
    clearContextAnnotations()
  })

  test('__gasoline.getActions should work', async () => {
    const { installGasolineAPI, uninstallGasolineAPI, recordAction, clearActionBuffer } = await import(
      '../extension/inject.js'
    )

    clearActionBuffer()
    installGasolineAPI()

    recordAction({ type: 'click', target: 'button#test' })

    const actions = globalThis.window.__gasoline.getActions()
    assert.strictEqual(actions.length, 1)
    assert.strictEqual(actions[0].type, 'click')

    uninstallGasolineAPI()
    clearActionBuffer()
  })

  test('__gasoline.clearActions should work', async () => {
    const { installGasolineAPI, uninstallGasolineAPI, recordAction, getActionBuffer } = await import(
      '../extension/inject.js'
    )

    installGasolineAPI()

    recordAction({ type: 'click', target: 'button' })
    assert.ok(getActionBuffer().length > 0)

    globalThis.window.__gasoline.clearActions()
    assert.strictEqual(getActionBuffer().length, 0)

    uninstallGasolineAPI()
  })

  test('__gasoline.setActionCapture should work', async () => {
    const {
      installGasolineAPI,
      uninstallGasolineAPI,
      recordAction,
      getActionBuffer,
      clearActionBuffer,
      setActionCaptureEnabled,
    } = await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)
    installGasolineAPI()

    globalThis.window.__gasoline.setActionCapture(false)
    recordAction({ type: 'click', target: 'button' })

    assert.strictEqual(getActionBuffer().length, 0)

    globalThis.window.__gasoline.setActionCapture(true)

    uninstallGasolineAPI()
  })
})

describe('User Action Replay', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    globalThis.window = createMockWindow()
    globalThis.console = createMockConsole()
    globalThis.document = {
      addEventListener: mock.fn(),
      removeEventListener: mock.fn(),
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
    delete globalThis.document
  })

  test('should record actions to buffer', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    recordAction({ type: 'click', target: 'button#submit' })

    const buffer = getActionBuffer()
    assert.strictEqual(buffer.length, 1)
    assert.strictEqual(buffer[0].type, 'click')
    assert.strictEqual(buffer[0].target, 'button#submit')
    assert.ok(buffer[0].ts) // Should have timestamp

    clearActionBuffer()
  })

  test('should limit buffer to MAX_ACTION_BUFFER_SIZE (20)', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    // Add 25 actions
    for (let i = 0; i < 25; i++) {
      recordAction({ type: 'click', index: i })
    }

    const buffer = getActionBuffer()
    assert.strictEqual(buffer.length, 20)
    // First action should be index 5 (oldest 5 removed)
    assert.strictEqual(buffer[0].index, 5)
    // Last action should be index 24
    assert.strictEqual(buffer[19].index, 24)

    clearActionBuffer()
  })

  test('should clear action buffer', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    setActionCaptureEnabled(true)
    recordAction({ type: 'click' })

    assert.ok(getActionBuffer().length > 0)

    clearActionBuffer()

    assert.strictEqual(getActionBuffer().length, 0)
  })

  test('should not record actions when capture disabled', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(false)

    recordAction({ type: 'click', target: 'button' })

    assert.strictEqual(getActionBuffer().length, 0)

    setActionCaptureEnabled(true)
  })

  test('should get element selector with id', async () => {
    const { getElementSelector } = await import('../extension/inject.js')

    const element = {
      tagName: 'BUTTON',
      id: 'submit-btn',
      className: '',
      getAttribute: () => null,
    }

    const selector = getElementSelector(element)
    assert.ok(selector.includes('button'))
    assert.ok(selector.includes('#submit-btn'))
  })

  test('should get element selector with classes', async () => {
    const { getElementSelector } = await import('../extension/inject.js')

    const element = {
      tagName: 'DIV',
      id: '',
      className: 'card primary large',
      getAttribute: () => null,
    }

    const selector = getElementSelector(element)
    assert.ok(selector.includes('div'))
    assert.ok(selector.includes('.card'))
    assert.ok(selector.includes('.primary'))
  })

  test('should get element selector with data-testid', async () => {
    const { getElementSelector } = await import('../extension/inject.js')

    const element = {
      tagName: 'INPUT',
      id: '',
      className: '',
      getAttribute: (attr) => (attr === 'data-testid' ? 'email-input' : null),
    }

    const selector = getElementSelector(element)
    assert.ok(selector.includes('input'))
    assert.ok(selector.includes('[data-testid="email-input"]'))
  })

  test('should truncate element selector to 100 chars', async () => {
    const { getElementSelector } = await import('../extension/inject.js')

    const element = {
      tagName: 'DIV',
      id: 'a'.repeat(50),
      className: 'b'.repeat(50),
      getAttribute: () => null,
    }

    const selector = getElementSelector(element)
    assert.ok(selector.length <= 100)
  })

  test('should identify password inputs as sensitive', async () => {
    const { isSensitiveInput } = await import('../extension/inject.js')

    assert.strictEqual(isSensitiveInput({ type: 'password' }), true)
  })

  test('should identify credit card inputs as sensitive', async () => {
    const { isSensitiveInput } = await import('../extension/inject.js')

    assert.strictEqual(isSensitiveInput({ type: 'text', autocomplete: 'cc-number' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', autocomplete: 'cc-exp' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', autocomplete: 'cc-csc' }), true)
  })

  test('should identify inputs by name as sensitive', async () => {
    const { isSensitiveInput } = await import('../extension/inject.js')

    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'password' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'user_password' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'secret_key' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'api_token' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'credit_card' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'cvv' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'ssn_number' }), true)
  })

  test('should not identify regular inputs as sensitive', async () => {
    const { isSensitiveInput } = await import('../extension/inject.js')

    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'email' }), false)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'username' }), false)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'address' }), false)
  })

  test('should handle click event', async () => {
    const { handleClick, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    const mockEvent = {
      target: {
        tagName: 'BUTTON',
        id: 'submit',
        className: 'btn primary',
        textContent: 'Submit Form',
        getAttribute: () => null,
      },
      clientX: 100,
      clientY: 200,
    }

    handleClick(mockEvent)

    const buffer = getActionBuffer()
    assert.strictEqual(buffer.length, 1)
    assert.strictEqual(buffer[0].type, 'click')
    assert.ok(buffer[0].target.includes('button'))
    assert.strictEqual(buffer[0].x, 100)
    assert.strictEqual(buffer[0].y, 200)
    assert.ok(buffer[0].text.includes('Submit'))

    clearActionBuffer()
  })

  test('should handle input event for non-sensitive field', async () => {
    const { handleInput, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    const mockEvent = {
      target: {
        tagName: 'INPUT',
        id: 'email',
        className: '',
        type: 'email',
        value: 'test@example.com',
        name: 'email',
        autocomplete: 'email',
        getAttribute: () => null,
      },
    }

    handleInput(mockEvent)

    const buffer = getActionBuffer()
    assert.strictEqual(buffer.length, 1)
    assert.strictEqual(buffer[0].type, 'input')
    assert.strictEqual(buffer[0].value, 'test@example.com')
    assert.strictEqual(buffer[0].length, 16)

    clearActionBuffer()
  })

  test('should redact sensitive input values', async () => {
    const { handleInput, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    const mockEvent = {
      target: {
        tagName: 'INPUT',
        id: 'password',
        className: '',
        type: 'password',
        value: 'super-secret-password',
        name: 'password',
        autocomplete: '',
        getAttribute: () => null,
      },
    }

    handleInput(mockEvent)

    const buffer = getActionBuffer()
    assert.strictEqual(buffer.length, 1)
    assert.strictEqual(buffer[0].value, '[redacted]')
    assert.strictEqual(buffer[0].length, 21) // Original length preserved

    clearActionBuffer()
  })

  test('should include actions in error logs', async () => {
    const {
      installConsoleCapture,
      uninstallConsoleCapture,
      recordAction,
      clearActionBuffer,
      setActionCaptureEnabled,
    } = await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    recordAction({ type: 'click', target: 'button#submit' })
    recordAction({ type: 'input', target: 'input#email', value: 'test@test.com' })

    installConsoleCapture()
    globalThis.console.error('Payment failed')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'error')
    assert.ok(message.payload._actions)
    assert.strictEqual(message.payload._actions.length, 2)
    assert.strictEqual(message.payload._actions[0].type, 'click')
    assert.strictEqual(message.payload._actions[1].type, 'input')

    uninstallConsoleCapture()
    clearActionBuffer()
  })

  test('should not include actions in non-error logs', async () => {
    const {
      installConsoleCapture,
      uninstallConsoleCapture,
      recordAction,
      clearActionBuffer,
      setActionCaptureEnabled,
    } = await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    recordAction({ type: 'click', target: 'button' })

    installConsoleCapture()
    globalThis.console.log('Info message')

    const [message] = globalThis.window.postMessage.mock.calls[0].arguments
    assert.strictEqual(message.payload.level, 'log')
    assert.ok(!message.payload._actions)

    uninstallConsoleCapture()
    clearActionBuffer()
  })

  test('should handle null target in events', async () => {
    const { handleClick, handleInput, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    handleClick({ target: null, clientX: 0, clientY: 0 })
    handleInput({ target: null })

    // Should not throw and buffer should be empty
    assert.strictEqual(getActionBuffer().length, 0)
  })

  test('should handle element without tagName in selector', async () => {
    const { getElementSelector } = await import('../extension/inject.js')

    assert.strictEqual(getElementSelector(null), '')
    assert.strictEqual(getElementSelector({}), '')
    assert.strictEqual(getElementSelector({ id: 'test' }), '')
  })

  test('should handle scroll event', async () => {
    const { handleScroll, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    // Mock window scroll position
    globalThis.window.scrollX = 100
    globalThis.window.scrollY = 500

    const mockEvent = {
      target: globalThis.document,
    }

    handleScroll(mockEvent)

    const buffer = getActionBuffer()
    assert.strictEqual(buffer.length, 1)
    assert.strictEqual(buffer[0].type, 'scroll')
    assert.strictEqual(buffer[0].scrollX, 100)
    assert.strictEqual(buffer[0].scrollY, 500)
    assert.strictEqual(buffer[0].target, 'document')

    clearActionBuffer()
  })
})
