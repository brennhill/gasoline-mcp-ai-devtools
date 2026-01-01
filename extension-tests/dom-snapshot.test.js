/**
 * @fileoverview Tests for DOM snapshot capture feature
 * TDD: These tests are written BEFORE implementation
 */

import { test, describe, mock, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

// Mock window and document
let originalWindow
let originalDocument

function createMockElement(tagName, attrs = {}, children = []) {
  const element = {
    tagName: tagName.toUpperCase(),
    id: attrs.id || '',
    className: attrs.className || '',
    textContent: attrs.textContent || '',
    innerHTML: attrs.innerHTML || '',
    attributes: Object.entries(attrs).map(([name, value]) => ({ name, value })),
    getAttribute: (name) => attrs[name] || null,
    children: children,
    childNodes: children,
    parentElement: null,
    outerHTML: `<${tagName}></${tagName}>`,
    nodeType: 1, // ELEMENT_NODE
    // Add direct properties for isSensitiveInput check
    type: attrs.type || '',
    name: attrs.name || '',
    autocomplete: attrs.autocomplete || '',
    value: attrs.value || '',
  }

  // Set parent references
  children.forEach((child) => {
    child.parentElement = element
  })

  return element
}

function createMockWindow() {
  return {
    location: { href: 'http://localhost:3000/test' },
    postMessage: mock.fn(),
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
    body: createMockElement('body'),
    documentElement: createMockElement('html'),
    querySelector: mock.fn(() => null),
    querySelectorAll: mock.fn(() => []),
    addEventListener: mock.fn(),
    removeEventListener: mock.fn(),
  }
}

describe('DOM Snapshot - captureDOMSnapshot', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.document = originalDocument
  })

  test('should capture DOM subtree around error element', async () => {
    const { captureDOMSnapshot } = await import('../extension/inject.js')

    const errorElement = createMockElement('div', { id: 'error-container', className: 'alert' }, [
      createMockElement('span', { textContent: 'Error message' }),
    ])

    const snapshot = captureDOMSnapshot(errorElement)

    assert.ok(snapshot)
    assert.strictEqual(snapshot.tagName, 'div')
    assert.strictEqual(snapshot.id, 'error-container')
    assert.ok(snapshot.children)
  })

  test('should capture document body when no element provided', async () => {
    const { captureDOMSnapshot } = await import('../extension/inject.js')

    globalThis.document.body = createMockElement('body', {}, [
      createMockElement('div', { id: 'content' }),
    ])

    const snapshot = captureDOMSnapshot(null)

    assert.ok(snapshot)
    assert.strictEqual(snapshot.tagName, 'body')
  })

  test('should limit depth to MAX_SNAPSHOT_DEPTH (5)', async () => {
    const { captureDOMSnapshot, MAX_SNAPSHOT_DEPTH } = await import('../extension/inject.js')

    // Create deeply nested structure (10 levels)
    let deepElement = createMockElement('div', { id: 'level-10' })
    for (let i = 9; i >= 1; i--) {
      deepElement = createMockElement('div', { id: `level-${i}` }, [deepElement])
    }

    const snapshot = captureDOMSnapshot(deepElement)

    // Count actual depth
    let depth = 0
    let current = snapshot
    while (current && current.children && current.children.length > 0) {
      depth++
      current = current.children[0]
    }

    assert.ok(depth <= MAX_SNAPSHOT_DEPTH)
  })

  test('should limit total nodes to MAX_SNAPSHOT_NODES (100)', async () => {
    const { captureDOMSnapshot, MAX_SNAPSHOT_NODES } = await import('../extension/inject.js')

    // Create wide structure with 200 children
    const children = []
    for (let i = 0; i < 200; i++) {
      children.push(createMockElement('span', { id: `child-${i}` }))
    }
    const wideElement = createMockElement('div', { id: 'wide' }, children)

    const snapshot = captureDOMSnapshot(wideElement)

    // Count total nodes
    function countNodes(node) {
      if (!node) return 0
      let count = 1
      if (node.children) {
        for (const child of node.children) {
          count += countNodes(child)
        }
      }
      return count
    }

    const totalNodes = countNodes(snapshot)
    assert.ok(totalNodes <= MAX_SNAPSHOT_NODES)
  })

  test('should include element attributes', async () => {
    const { captureDOMSnapshot } = await import('../extension/inject.js')

    const element = createMockElement('input', {
      id: 'email',
      type: 'email',
      placeholder: 'Enter email',
      'data-testid': 'email-input',
    })

    const snapshot = captureDOMSnapshot(element)

    assert.ok(snapshot.attributes)
    assert.ok(snapshot.attributes.id === 'email' || snapshot.id === 'email')
  })

  test('should redact sensitive input values', async () => {
    const { captureDOMSnapshot } = await import('../extension/inject.js')

    const passwordInput = createMockElement('input', {
      id: 'password',
      type: 'password',
      value: 'super-secret-123',
    })

    const snapshot = captureDOMSnapshot(passwordInput)

    // Value should be redacted
    assert.ok(!JSON.stringify(snapshot).includes('super-secret-123'))
    assert.ok(
      JSON.stringify(snapshot).includes('[redacted]') ||
        !snapshot.attributes?.value ||
        snapshot.attributes?.value === '[redacted]'
    )
  })

  test('should exclude script and style tags', async () => {
    const { captureDOMSnapshot } = await import('../extension/inject.js')

    const element = createMockElement('div', {}, [
      createMockElement('script', { textContent: 'alert("xss")' }),
      createMockElement('style', { textContent: 'body { color: red }' }),
      createMockElement('span', { textContent: 'Visible content' }),
    ])

    const snapshot = captureDOMSnapshot(element)

    // Should not contain script or style
    const json = JSON.stringify(snapshot)
    assert.ok(!json.includes('"tagName":"script"'))
    assert.ok(!json.includes('"tagName":"style"'))
    assert.ok(json.includes('span') || snapshot.children?.some((c) => c.tagName === 'span'))
  })

  test('should truncate long text content', async () => {
    const { captureDOMSnapshot, MAX_TEXT_LENGTH } = await import('../extension/inject.js')

    const longText = 'x'.repeat(10000)
    const element = createMockElement('p', { textContent: longText })

    const snapshot = captureDOMSnapshot(element)

    assert.ok(snapshot.textContent.length <= MAX_TEXT_LENGTH + 20) // Allow for truncation marker
    assert.ok(snapshot.textContent.includes('[truncated]') || snapshot.textContent.length < 10000)
  })

  test('should capture parent context (ancestors)', async () => {
    const { captureDOMSnapshot } = await import('../extension/inject.js')

    const child = createMockElement('span', { id: 'error-text', textContent: 'Error!' })
    const parent = createMockElement('div', { id: 'error-container', className: 'alert' }, [child])
    const grandparent = createMockElement('section', { id: 'main' }, [parent])

    child.parentElement = parent
    parent.parentElement = grandparent

    const snapshot = captureDOMSnapshot(child, { includeAncestors: true })

    // Should include ancestor chain
    assert.ok(snapshot.ancestors || snapshot._path)
  })

  test('should return null for null/undefined element and no body', async () => {
    const { captureDOMSnapshot } = await import('../extension/inject.js')

    globalThis.document.body = null

    const snapshot = captureDOMSnapshot(null)

    assert.strictEqual(snapshot, null)
  })
})

describe('DOM Snapshot - serializeElement', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.document = originalDocument
  })

  test('should serialize basic element', async () => {
    const { serializeElement } = await import('../extension/inject.js')

    const element = createMockElement('div', { id: 'test', className: 'container' })

    const result = serializeElement(element)

    assert.strictEqual(result.tagName, 'div')
    assert.strictEqual(result.id, 'test')
    assert.ok(result.className === 'container' || result.attributes?.class === 'container')
  })

  test('should handle text nodes', async () => {
    const { serializeElement } = await import('../extension/inject.js')

    const textNode = {
      nodeType: 3, // TEXT_NODE
      textContent: 'Hello world',
    }

    const result = serializeElement(textNode)

    assert.ok(result.type === 'text' || result.text || result.textContent)
  })

  test('should skip comment nodes', async () => {
    const { serializeElement } = await import('../extension/inject.js')

    const commentNode = {
      nodeType: 8, // COMMENT_NODE
      textContent: 'This is a comment',
    }

    const result = serializeElement(commentNode)

    assert.strictEqual(result, null)
  })

  test('should handle elements with data attributes', async () => {
    const { serializeElement } = await import('../extension/inject.js')

    const element = createMockElement('button', {
      'data-action': 'submit',
      'data-testid': 'submit-btn',
    })

    const result = serializeElement(element)

    assert.ok(result)
    // Should preserve data attributes
    const json = JSON.stringify(result)
    assert.ok(json.includes('submit') || json.includes('data-'))
  })
})

describe('DOM Snapshot - getDOMSnapshotForError', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.document = originalDocument
  })

  test('should create snapshot entry for error', async () => {
    const { getDOMSnapshotForError, setDOMSnapshotEnabled, resetDOMSnapshotRateLimit } =
      await import('../extension/inject.js')

    setDOMSnapshotEnabled(true)
    resetDOMSnapshotRateLimit()

    globalThis.document.body = createMockElement('body', {}, [
      createMockElement('div', { id: 'app' }),
    ])

    const errorEntry = {
      type: 'exception',
      level: 'error',
      message: 'Test error',
      ts: new Date().toISOString(),
    }

    const snapshot = await getDOMSnapshotForError(errorEntry)

    assert.ok(snapshot)
    assert.strictEqual(snapshot.type, 'dom_snapshot')
    assert.ok(snapshot.ts)
    assert.ok(snapshot.snapshot)
    assert.ok(snapshot.relatedErrorId || snapshot._errorTs)
  })

  test('should respect domSnapshotEnabled setting', async () => {
    const { getDOMSnapshotForError, setDOMSnapshotEnabled } = await import('../extension/inject.js')

    setDOMSnapshotEnabled(false)

    const errorEntry = {
      type: 'exception',
      level: 'error',
      message: 'Test error',
    }

    const snapshot = await getDOMSnapshotForError(errorEntry)

    assert.strictEqual(snapshot, null)

    // Re-enable for other tests
    setDOMSnapshotEnabled(true)
  })

  test('should rate limit snapshots', async () => {
    const {
      getDOMSnapshotForError,
      setDOMSnapshotEnabled,
      resetDOMSnapshotRateLimit,
    } = await import('../extension/inject.js')

    setDOMSnapshotEnabled(true)
    resetDOMSnapshotRateLimit()

    globalThis.document.body = createMockElement('body')

    const errorEntry = {
      type: 'exception',
      level: 'error',
      message: 'Test error',
      ts: new Date().toISOString(),
    }

    // First snapshot should succeed
    const first = await getDOMSnapshotForError(errorEntry)
    assert.ok(first)

    // Immediate second should be rate limited
    const second = await getDOMSnapshotForError(errorEntry)
    assert.strictEqual(second, null)
  })

  test('should include viewport dimensions', async () => {
    const { getDOMSnapshotForError, setDOMSnapshotEnabled, resetDOMSnapshotRateLimit } = await import('../extension/inject.js')

    setDOMSnapshotEnabled(true)
    resetDOMSnapshotRateLimit()

    globalThis.window.innerWidth = 1920
    globalThis.window.innerHeight = 1080
    globalThis.document.body = createMockElement('body')

    const errorEntry = { type: 'exception', level: 'error', message: 'Test' }

    const snapshot = await getDOMSnapshotForError(errorEntry)

    assert.ok(snapshot.viewport || snapshot.width || snapshot.meta?.viewport)
  })
})

describe('DOM Snapshot - Configuration', () => {
  beforeEach(() => {
    originalWindow = globalThis.window
    originalDocument = globalThis.document
    globalThis.window = createMockWindow()
    globalThis.document = createMockDocument()
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.document = originalDocument
  })

  test('setDOMSnapshotEnabled should toggle feature', async () => {
    const { setDOMSnapshotEnabled, isDOMSnapshotEnabled } = await import('../extension/inject.js')

    setDOMSnapshotEnabled(true)
    assert.strictEqual(isDOMSnapshotEnabled(), true)

    setDOMSnapshotEnabled(false)
    assert.strictEqual(isDOMSnapshotEnabled(), false)
  })

  test('should expose DOM snapshot through __gasoline API', async () => {
    const { installGasolineAPI, uninstallGasolineAPI, setDOMSnapshotEnabled } = await import(
      '../extension/inject.js'
    )

    setDOMSnapshotEnabled(true)
    installGasolineAPI()

    assert.ok(globalThis.window.__gasoline)
    assert.ok(typeof globalThis.window.__gasoline.setDOMSnapshot === 'function')

    globalThis.window.__gasoline.setDOMSnapshot(false)

    const { isDOMSnapshotEnabled } = await import('../extension/inject.js')
    assert.strictEqual(isDOMSnapshotEnabled(), false)

    uninstallGasolineAPI()
  })
})
