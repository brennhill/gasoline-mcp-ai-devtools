// @ts-nocheck
/**
 * @fileoverview inject-context-api-actions.test.js — Tests for context annotations,
 * the window.__gasoline API, and user action replay in inject.js.
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'
import { createMockWindow, createMockConsole, createMockDocument } from './helpers.js'

// Define esbuild constant not available in Node test env
globalThis.__GASOLINE_VERSION__ = 'test'

// Store original
let originalWindow
let originalConsole

describe('Context Annotations', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalConsole = globalThis.console
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.console = createMockConsole()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
  })

  test('should set and get context annotation', async () => {
    const { setContextAnnotation, getContextAnnotations, clearContextAnnotations } =
      await import('../../extension/inject.js')

    clearContextAnnotations()

    setContextAnnotation('checkout-flow', { step: 'payment', items: 3 })

    const context = getContextAnnotations()
    assert.ok(context)
    assert.strictEqual(context['checkout-flow'].step, 'payment')
    assert.strictEqual(context['checkout-flow'].items, 3)

    clearContextAnnotations()
  })

  test('should remove context annotation', async () => {
    const { setContextAnnotation, removeContextAnnotation, getContextAnnotations, clearContextAnnotations } =
      await import('../../extension/inject.js')

    clearContextAnnotations()

    setContextAnnotation('user', { id: 'usr_123' })
    assert.ok(getContextAnnotations()['user'])

    removeContextAnnotation('user')
    const context = getContextAnnotations()
    assert.ok(!context || !context['user'])

    clearContextAnnotations()
  })

  test('should clear all annotations', async () => {
    const { setContextAnnotation, clearContextAnnotations, getContextAnnotations } =
      await import('../../extension/inject.js')

    setContextAnnotation('a', 1)
    setContextAnnotation('b', 2)

    clearContextAnnotations()

    const context = getContextAnnotations()
    assert.ok(context === null)
  })

  test('should reject empty key', async () => {
    const { setContextAnnotation, clearContextAnnotations } = await import('../../extension/inject.js')

    clearContextAnnotations()

    const result = setContextAnnotation('', 'value')
    assert.strictEqual(result, false)
  })

  test('should reject non-string key', async () => {
    const { setContextAnnotation, clearContextAnnotations } = await import('../../extension/inject.js')

    clearContextAnnotations()

    const result = setContextAnnotation(123, 'value')
    assert.strictEqual(result, false)
  })

  test('should reject key longer than 100 chars', async () => {
    const { setContextAnnotation, clearContextAnnotations } = await import('../../extension/inject.js')

    clearContextAnnotations()

    const longKey = 'x'.repeat(101)
    const result = setContextAnnotation(longKey, 'value')
    assert.strictEqual(result, false)
  })

  test('should truncate large values', async () => {
    const { setContextAnnotation, getContextAnnotations, clearContextAnnotations } =
      await import('../../extension/inject.js')

    clearContextAnnotations()

    const largeValue = { data: 'x'.repeat(5000) }
    const result = setContextAnnotation('large', largeValue)

    // Should return false or store truncated
    assert.ok(result === false || getContextAnnotations()['large'] === '[Value too large]')

    clearContextAnnotations()
  })

  test('should include context in error logs', async () => {
    const { installConsoleCapture, uninstallConsoleCapture, setContextAnnotation, clearContextAnnotations } =
      await import('../../extension/inject.js')

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
    const { installConsoleCapture, uninstallConsoleCapture, setContextAnnotation, clearContextAnnotations } =
      await import('../../extension/inject.js')

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
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('should install window.__gasoline API', async () => {
    const { installGasolineAPI, uninstallGasolineAPI } = await import('../../extension/inject.js')

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
    const { installGasolineAPI, uninstallGasolineAPI } = await import('../../extension/inject.js')

    installGasolineAPI()
    assert.ok(globalThis.window.__gasoline)

    uninstallGasolineAPI()
    assert.ok(!globalThis.window.__gasoline)
  })

  test('__gasoline.annotate should work', async () => {
    const { installGasolineAPI, uninstallGasolineAPI, clearContextAnnotations } =
      await import('../../extension/inject.js')

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
    const { installGasolineAPI, uninstallGasolineAPI, recordAction, clearActionBuffer } =
      await import('../../extension/inject.js')

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
    const { installGasolineAPI, uninstallGasolineAPI, recordAction, getActionBuffer } =
      await import('../../extension/inject.js')

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
      setActionCaptureEnabled
    } = await import('../../extension/inject.js')

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
    globalThis.window = createMockWindow({ href: 'http://localhost:3000/test', withOnerror: true })
    globalThis.console = createMockConsole()
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.console = originalConsole
    delete globalThis.document
  })

  test('should record actions to buffer', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../extension/inject.js')

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
      await import('../../extension/inject.js')

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
      await import('../../extension/inject.js')

    setActionCaptureEnabled(true)
    recordAction({ type: 'click' })

    assert.ok(getActionBuffer().length > 0)

    clearActionBuffer()

    assert.strictEqual(getActionBuffer().length, 0)
  })

  test('should not record actions when capture disabled', async () => {
    const { recordAction, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(false)

    recordAction({ type: 'click', target: 'button' })

    assert.strictEqual(getActionBuffer().length, 0)

    setActionCaptureEnabled(true)
  })

  test('should get element selector with id', async () => {
    const { getElementSelector } = await import('../../extension/inject.js')

    const element = {
      tagName: 'BUTTON',
      id: 'submit-btn',
      className: '',
      getAttribute: () => null
    }

    const selector = getElementSelector(element)
    assert.ok(selector.includes('button'))
    assert.ok(selector.includes('#submit-btn'))
  })

  test('should get element selector with classes', async () => {
    const { getElementSelector } = await import('../../extension/inject.js')

    const element = {
      tagName: 'DIV',
      id: '',
      className: 'card primary large',
      getAttribute: () => null
    }

    const selector = getElementSelector(element)
    assert.ok(selector.includes('div'))
    assert.ok(selector.includes('.card'))
    assert.ok(selector.includes('.primary'))
  })

  test('should get element selector with data-testid', async () => {
    const { getElementSelector } = await import('../../extension/inject.js')

    const element = {
      tagName: 'INPUT',
      id: '',
      className: '',
      getAttribute: (attr) => (attr === 'data-testid' ? 'email-input' : null)
    }

    const selector = getElementSelector(element)
    assert.ok(selector.includes('input'))
    assert.ok(selector.includes('[data-testid="email-input"]'))
  })

  test('should truncate element selector to 100 chars', async () => {
    const { getElementSelector } = await import('../../extension/inject.js')

    const element = {
      tagName: 'DIV',
      id: 'a'.repeat(50),
      className: 'b'.repeat(50),
      getAttribute: () => null
    }

    const selector = getElementSelector(element)
    assert.ok(selector.length <= 100)
  })

  test('should identify password inputs as sensitive', async () => {
    const { isSensitiveInput } = await import('../../extension/inject.js')

    assert.strictEqual(isSensitiveInput({ type: 'password' }), true)
  })

  test('should identify credit card inputs as sensitive', async () => {
    const { isSensitiveInput } = await import('../../extension/inject.js')

    assert.strictEqual(isSensitiveInput({ type: 'text', autocomplete: 'cc-number' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', autocomplete: 'cc-exp' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', autocomplete: 'cc-csc' }), true)
  })

  test('should identify inputs by name as sensitive', async () => {
    const { isSensitiveInput } = await import('../../extension/inject.js')

    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'password' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'user_password' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'secret_key' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'api_token' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'credit_card' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'cvv' }), true)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'ssn_number' }), true)
  })

  test('should not identify regular inputs as sensitive', async () => {
    const { isSensitiveInput } = await import('../../extension/inject.js')

    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'email' }), false)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'username' }), false)
    assert.strictEqual(isSensitiveInput({ type: 'text', name: 'address' }), false)
  })

  test('should handle click event', async () => {
    const { handleClick, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    const mockEvent = {
      target: {
        tagName: 'BUTTON',
        id: 'submit',
        className: 'btn primary',
        textContent: 'Submit Form',
        getAttribute: () => null
      },
      clientX: 100,
      clientY: 200
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
      await import('../../extension/inject.js')

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
        getAttribute: () => null
      }
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
      await import('../../extension/inject.js')

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
        getAttribute: () => null
      }
    }

    handleInput(mockEvent)

    const buffer = getActionBuffer()
    assert.strictEqual(buffer.length, 1)
    assert.strictEqual(buffer[0].value, '[redacted]')
    assert.strictEqual(buffer[0].length, 21) // Original length preserved

    clearActionBuffer()
  })

  test('should include actions in error logs', async () => {
    const { installConsoleCapture, uninstallConsoleCapture, recordAction, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../extension/inject.js')

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
    const { installConsoleCapture, uninstallConsoleCapture, recordAction, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../extension/inject.js')

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
      await import('../../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    handleClick({ target: null, clientX: 0, clientY: 0 })
    handleInput({ target: null })

    // Should not throw and buffer should be empty
    assert.strictEqual(getActionBuffer().length, 0)
  })

  test('should handle element without tagName in selector', async () => {
    const { getElementSelector } = await import('../../extension/inject.js')

    assert.strictEqual(getElementSelector(null), '')
    assert.strictEqual(getElementSelector({}), '')
    assert.strictEqual(getElementSelector({ id: 'test' }), '')
  })

  test('should handle scroll event', async () => {
    const { handleScroll, getActionBuffer, clearActionBuffer, setActionCaptureEnabled } =
      await import('../../extension/inject.js')

    clearActionBuffer()
    setActionCaptureEnabled(true)

    // Mock window scroll position
    globalThis.window.scrollX = 100
    globalThis.window.scrollY = 500

    const mockEvent = {
      target: globalThis.document
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
