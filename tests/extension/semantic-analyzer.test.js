// @ts-nocheck
import { test, describe, beforeEach, mock } from 'node:test'
import assert from 'node:assert'
import { analyzePage } from '../../extension/lib/semantic-analyzer.js'

describe('Semantic Analyzer Logic (True TDD)', () => {
  
  beforeEach(() => {
    globalThis.window = {
      innerHeight: 1080,
      innerWidth: 1920,
      location: { href: 'https://example.com/article/123' }
    }
    
    globalThis.document = {
      title: 'Mock Page Title',
      querySelector: mock.fn(() => null),
      querySelectorAll: mock.fn(() => []),
      body: { innerText: 'Some default body text.' }
    }
  })

  test('compact mode: identifies login pages correctly', () => {
    // Mock a login page
    const passwordInput = { tagName: 'INPUT', type: 'password' }
    globalThis.document.querySelector = (s) => {
      if (s === 'input[type="password"]') return passwordInput
      return null
    }

    const result = analyzePage('compact')

    assert.strictEqual(result.type, 'login')
  })

  test('compact mode: extracts primary CTAs including [onclick]', () => {
    const customButton = {
      tagName: 'DIV',
      textContent: 'Custom Action',
      hasAttribute: (a) => a === 'onclick',
      getAttribute: (a) => a === 'onclick' ? 'run()' : null,
      getBoundingClientRect: () => ({ top: 100, left: 100, width: 100, height: 40 }),
      offsetParent: {}
    }

    globalThis.document.querySelectorAll = (s) => {
      if (s.includes('[onclick]')) return [customButton]
      return []
    }

    const result = analyzePage('compact')

    assert.ok(result.primary_actions.length > 0)
    assert.strictEqual(result.primary_actions[0].label, 'Custom Action')
  })
})
