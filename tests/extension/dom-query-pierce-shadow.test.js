// @ts-nocheck
import { test, describe, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

import { executeDOMQuery } from '../../extension/lib/dom-queries.js'

function makeElement(tag, options = {}) {
  const id = options.id || ''
  const text = options.text || ''
  const attrs = []
  if (id) attrs.push({ name: 'id', value: id })

  return {
    tagName: tag.toUpperCase(),
    id,
    textContent: text,
    attributes: attrs,
    children: [],
    shadowRoot: null,
    offsetParent: {},
    getBoundingClientRect: () => ({ x: 0, y: 0, width: 100, height: 20 })
  }
}

function appendChild(parent, child) {
  parent.children.push(child)
  return child
}

function selectorMatches(el, selector) {
  if (selector === '*') return true
  if (selector.startsWith('#')) return el.id === selector.slice(1)
  return el.tagName?.toLowerCase() === selector.toLowerCase()
}

function collectMatches(root, selector) {
  const out = []
  const stack = Array.isArray(root.children) ? [...root.children] : []
  while (stack.length > 0) {
    const el = stack.shift()
    if (!el) continue
    if (selectorMatches(el, selector)) out.push(el)
    if (Array.isArray(el.children) && el.children.length > 0) {
      stack.push(...el.children)
    }
  }
  return out
}

function makeShadowRoot() {
  const root = {
    children: [],
    querySelectorAll(selector) {
      return collectMatches(root, selector)
    }
  }
  return root
}

function makeDocument(children) {
  const doc = {
    title: 'Test Page',
    children,
    querySelectorAll(selector) {
      return collectMatches(doc, selector)
    }
  }
  return doc
}

function getResultIds(result) {
  return result.matches
    .map((m) => m.attributes?.id || '')
    .filter((id) => id)
    .sort()
}

describe('executeDOMQuery pierce_shadow', () => {
  let originalWindow
  let originalDocument

  beforeEach(() => {
    originalWindow = globalThis.window
    originalDocument = globalThis.document

    globalThis.window = {
      location: { href: 'https://example.test/app' },
      __GASOLINE_CLOSED_SHADOWS__: undefined
    }
  })

  afterEach(() => {
    globalThis.window = originalWindow
    globalThis.document = originalDocument
  })

  test('pierce_shadow=false only searches light DOM', async () => {
    const lightButton = makeElement('button', { id: 'light-btn', text: 'Light' })
    const host = makeElement('widget-host', { id: 'host-1' })
    const openRoot = makeShadowRoot()
    const shadowButton = makeElement('button', { id: 'shadow-btn', text: 'Shadow' })
    appendChild(openRoot, shadowButton)
    host.shadowRoot = openRoot

    globalThis.document = makeDocument([lightButton, host])

    const result = await executeDOMQuery({ selector: 'button', pierce_shadow: false })

    assert.strictEqual(result.matchCount, 1)
    assert.strictEqual(result.returnedCount, 1)
    assert.deepStrictEqual(getResultIds(result), ['light-btn'])
  })

  test('pierce_shadow=true traverses open shadow roots', async () => {
    const lightButton = makeElement('button', { id: 'light-btn', text: 'Light' })
    const host = makeElement('widget-host', { id: 'host-1' })
    const openRoot = makeShadowRoot()
    const shadowButton = makeElement('button', { id: 'shadow-btn', text: 'Shadow' })
    appendChild(openRoot, shadowButton)
    host.shadowRoot = openRoot

    globalThis.document = makeDocument([lightButton, host])

    const result = await executeDOMQuery({ selector: 'button', pierce_shadow: true })

    assert.strictEqual(result.matchCount, 2)
    assert.strictEqual(result.returnedCount, 2)
    assert.deepStrictEqual(getResultIds(result), ['light-btn', 'shadow-btn'])
  })

  test('pierce_shadow=true traverses captured closed shadow roots', async () => {
    const lightButton = makeElement('button', { id: 'light-btn', text: 'Light' })
    const closedHost = makeElement('secure-host', { id: 'closed-host' })
    const closedRoot = makeShadowRoot()
    const closedShadowButton = makeElement('button', { id: 'closed-shadow-btn', text: 'Secret' })
    appendChild(closedRoot, closedShadowButton)

    const closedMap = new WeakMap()
    closedMap.set(closedHost, closedRoot)
    globalThis.window.__GASOLINE_CLOSED_SHADOWS__ = closedMap

    globalThis.document = makeDocument([lightButton, closedHost])

    const result = await executeDOMQuery({ selector: 'button', pierce_shadow: true })

    assert.strictEqual(result.matchCount, 2)
    assert.strictEqual(result.returnedCount, 2)
    assert.deepStrictEqual(getResultIds(result), ['closed-shadow-btn', 'light-btn'])
  })

  test('pierce_shadow=auto defaults to light DOM unless resolved upstream', async () => {
    const lightButton = makeElement('button', { id: 'light-btn', text: 'Light' })
    const host = makeElement('widget-host', { id: 'host-1' })
    const openRoot = makeShadowRoot()
    const shadowButton = makeElement('button', { id: 'shadow-btn', text: 'Shadow' })
    appendChild(openRoot, shadowButton)
    host.shadowRoot = openRoot

    globalThis.document = makeDocument([lightButton, host])

    const result = await executeDOMQuery({ selector: 'button', pierce_shadow: 'auto' })

    assert.strictEqual(result.matchCount, 1)
    assert.strictEqual(result.returnedCount, 1)
    assert.deepStrictEqual(getResultIds(result), ['light-btn'])
  })

  test('global max element cap applies across light + shadow matches', async () => {
    const lightButtons = []
    for (let i = 0; i < 40; i++) {
      lightButtons.push(makeElement('button', { id: `light-${i}` }))
    }

    const host = makeElement('shadow-host', { id: 'host' })
    const openRoot = makeShadowRoot()
    const shadowButtons = []
    for (let i = 0; i < 40; i++) {
      shadowButtons.push(makeElement('button', { id: `shadow-${i}` }))
    }
    for (const el of shadowButtons) appendChild(openRoot, el)
    host.shadowRoot = openRoot

    globalThis.document = makeDocument([...lightButtons, host])

    const result = await executeDOMQuery({ selector: 'button', pierce_shadow: true })

    assert.strictEqual(result.matchCount, 80)
    assert.strictEqual(result.returnedCount, 50)
    assert.strictEqual(result.matches.length, 50)
  })
})
