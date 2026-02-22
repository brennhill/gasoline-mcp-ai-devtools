// @ts-nocheck
import { beforeEach, describe, test } from 'node:test'
import assert from 'node:assert'

class MockHTMLElement {
  constructor(tag, props = {}) {
    this.tagName = tag.toUpperCase()
    this.id = props.id || ''
    this.textContent = props.textContent || ''
    this.offsetParent = {}
    this._attrs = props.attrs || {}
  }
  getAttribute(name) {
    return this._attrs[name] || null
  }
  getRootNode() {
    return globalThis.document
  }
  querySelectorAll() {
    return []
  }
  getBoundingClientRect() {
    return { x: 0, y: 0, left: 0, top: 0, width: 100, height: 30, right: 100, bottom: 30 }
  }
}

globalThis.HTMLElement = MockHTMLElement
globalThis.HTMLInputElement = class extends MockHTMLElement {}
globalThis.ShadowRoot = class ShadowRoot {}

const { domPrimitiveListInteractive } = await import('./dom-primitives-list-interactive.js')

describe('domPrimitiveListInteractive', () => {
  beforeEach(() => {
    const btn1 = new MockHTMLElement('button', { id: 'save-btn', textContent: 'Save' })
    btn1.getBoundingClientRect = () => ({
      x: 24,
      y: 48,
      left: 24,
      top: 48,
      width: 120,
      height: 36,
      right: 144,
      bottom: 84
    })
    const btn2 = new MockHTMLElement('button', { id: 'cancel-btn', textContent: 'Cancel' })
    btn2.getBoundingClientRect = () => ({
      x: 180,
      y: 48,
      left: 180,
      top: 48,
      width: 120,
      height: 36,
      right: 300,
      bottom: 84
    })

    globalThis.document = {
      querySelectorAll: (sel) => (sel === 'button' ? [btn1, btn2] : []),
      body: {
        querySelectorAll: (sel) => (sel === 'button' ? [btn1, btn2] : []),
        children: { length: 0 }
      },
      documentElement: { children: { length: 0 } }
    }
  })

  test('returns bbox coordinates for interactive elements', () => {
    const result = domPrimitiveListInteractive('')
    assert.strictEqual(result.success, true)
    assert.ok(Array.isArray(result.elements))
    assert.strictEqual(result.elements.length >= 2, true)

    const first = result.elements[0]
    assert.ok(first.bbox, 'bbox should be present on list_interactive elements')
    assert.deepStrictEqual(first.bbox, { x: 24, y: 48, width: 120, height: 36 })
  })
})
