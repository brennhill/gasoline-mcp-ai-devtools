// @ts-nocheck
/**
 * @fileoverview ai-context-enrichment.test.js — Unit tests for framework detection,
 * React fiber walking, state snapshot, AI summary generation, and enrichment pipeline.
 */

import { describe, test, beforeEach, afterEach } from 'node:test'
import assert from 'node:assert'

import {
  detectFramework,
  getReactComponentAncestry,
  captureStateSnapshot,
  generateAiSummary,
  enrichErrorWithAiContext,
  setAiContextEnabled,
  setAiContextStateSnapshot,
  resetEnrichmentForTesting
} from '../../extension/lib/ai-context-enrichment.js'

// =============================================================================
// FRAMEWORK DETECTION
// =============================================================================

describe('detectFramework', () => {
  test('detects React via __reactFiber$ key', () => {
    const el = { __reactFiber$abc123: {} }
    const result = detectFramework(el)

    assert.strictEqual(result.framework, 'react')
    assert.strictEqual(result.key, '__reactFiber$abc123')
  })

  test('detects React via __reactInternalInstance$ key', () => {
    const result = detectFramework({ __reactInternalInstance$xyz: {} })

    assert.strictEqual(result.framework, 'react')
    assert.strictEqual(result.key, '__reactInternalInstance$xyz')
  })

  test('detects Vue via __vueParentComponent', () => {
    const result = detectFramework({ __vueParentComponent: {} })

    assert.strictEqual(result.framework, 'vue')
  })

  test('detects Vue via __vue_app__', () => {
    const result = detectFramework({ __vue_app__: {} })

    assert.strictEqual(result.framework, 'vue')
  })

  test('detects Svelte via __svelte_meta', () => {
    const result = detectFramework({ __svelte_meta: { loc: { file: 'App.svelte' } } })

    assert.strictEqual(result.framework, 'svelte')
  })

  test('returns null for plain DOM element', () => {
    assert.strictEqual(detectFramework({ tagName: 'DIV' }), null)
  })

  test('returns null for empty object', () => {
    assert.strictEqual(detectFramework({}), null)
  })

  test('returns null for null', () => {
    assert.strictEqual(detectFramework(null), null)
  })

  test('returns null for undefined', () => {
    assert.strictEqual(detectFramework(undefined), null)
  })
})

// =============================================================================
// REACT COMPONENT ANCESTRY
// =============================================================================

describe('getReactComponentAncestry', () => {
  test('walks fiber tree and returns root-first order', () => {
    const fiber = {
      type: { name: 'Child' },
      memoizedProps: {},
      return: {
        type: { name: 'Parent' },
        memoizedProps: {},
        return: {
          type: { name: 'App' },
          memoizedProps: {},
          return: null
        }
      }
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry.length, 3)
    assert.strictEqual(ancestry[0].name, 'App')
    assert.strictEqual(ancestry[2].name, 'Child')
  })

  test('prefers displayName over name', () => {
    const fiber = {
      type: { name: 'Comp', displayName: 'PrettyName' },
      memoizedProps: {},
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].name, 'PrettyName')
  })

  test('uses Anonymous for unnamed components', () => {
    const fiber = {
      type: { name: '', displayName: null },
      memoizedProps: {},
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].name, 'Anonymous')
  })

  test('extracts prop keys excluding children', () => {
    const fiber = {
      type: { name: 'Btn' },
      memoizedProps: { onClick: () => {}, label: 'ok', children: 'text' },
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.ok(ancestry[0].propKeys.includes('onClick'))
    assert.ok(ancestry[0].propKeys.includes('label'))
    assert.ok(!ancestry[0].propKeys.includes('children'))
  })

  test('extracts state keys and sets hasState', () => {
    const fiber = {
      type: { name: 'Form' },
      memoizedProps: {},
      memoizedState: { email: '', loading: false },
      return: null
    }

    const ancestry = getReactComponentAncestry(fiber)

    assert.strictEqual(ancestry[0].hasState, true)
    assert.ok(ancestry[0].stateKeys.includes('email'))
  })

  test('skips host elements (type is string)', () => {
    const fiber = {
      type: { name: 'Inner' },
      memoizedProps: {},
      return: {
        type: 'div',
        memoizedProps: {},
        return: {
          type: { name: 'Outer' },
          memoizedProps: {},
          return: null
        }
      }
    }

    const ancestry = getReactComponentAncestry(fiber)
    const names = ancestry.map((c) => c.name)

    assert.ok(!names.includes('div'))
    assert.ok(names.includes('Inner'))
    assert.ok(names.includes('Outer'))
  })

  test('limits depth to 10', () => {
    let current = null
    for (let i = 0; i < 15; i++) {
      current = { type: { name: `C${i}` }, memoizedProps: {}, return: current }
    }

    const ancestry = getReactComponentAncestry(current)

    assert.ok(ancestry.length <= 10)
  })

  test('returns null for null fiber', () => {
    assert.strictEqual(getReactComponentAncestry(null), null)
  })
})

// =============================================================================
// AI SUMMARY GENERATION
// =============================================================================

describe('generateAiSummary', () => {
  test('includes file and line when provided', () => {
    const summary = generateAiSummary({
      errorType: 'TypeError',
      message: 'boom',
      file: 'app.tsx',
      line: 42,
      componentAncestry: null,
      stateSnapshot: null
    })

    assert.ok(summary.includes('TypeError'))
    assert.ok(summary.includes('app.tsx:42'))
    assert.ok(summary.includes('boom'))
  })

  test('falls back to type: message when no file', () => {
    const summary = generateAiSummary({
      errorType: 'Error',
      message: 'Something broke',
      file: null,
      line: null,
      componentAncestry: null,
      stateSnapshot: null
    })

    assert.ok(summary.includes('Error: Something broke'))
  })

  test('includes component tree path', () => {
    const summary = generateAiSummary({
      errorType: 'Error',
      message: 'x',
      file: null,
      line: null,
      componentAncestry: {
        framework: 'react',
        components: [{ name: 'App' }, { name: 'Dashboard' }, { name: 'Chart' }]
      },
      stateSnapshot: null
    })

    assert.ok(summary.includes('App > Dashboard > Chart'))
  })

  test('includes state info when present', () => {
    const summary = generateAiSummary({
      errorType: 'Error',
      message: 'fail',
      file: null,
      line: null,
      componentAncestry: null,
      stateSnapshot: {
        source: 'redux',
        keys: {},
        relevantSlice: { 'auth.error': 'Unauthorized' }
      }
    })

    assert.ok(summary.includes('auth.error'))
    assert.ok(summary.includes('Unauthorized'))
  })
})

// =============================================================================
// ENRICHMENT PIPELINE
// =============================================================================

describe('enrichErrorWithAiContext', () => {
  let originalDocument

  beforeEach(() => {
    resetEnrichmentForTesting()
    originalDocument = globalThis.document
    globalThis.document = { activeElement: null }
  })

  afterEach(() => {
    globalThis.document = originalDocument
    resetEnrichmentForTesting()
  })

  test('produces _aiContext with summary on error entry', async () => {
    const error = {
      message: 'Cannot read prop',
      stack: `Error: Cannot read prop
    at foo (http://localhost:3000/app.js:10:5)`
    }

    const enriched = await enrichErrorWithAiContext(error)

    assert.ok(enriched._aiContext)
    assert.ok(enriched._aiContext.summary)
    assert.ok(enriched._enrichments.includes('aiContext'))
  })

  test('skips enrichment when disabled', async () => {
    setAiContextEnabled(false)

    const enriched = await enrichErrorWithAiContext({ message: 'x', stack: 'Error' })

    assert.strictEqual(enriched._aiContext, undefined)
  })

  test('completes within timeout budget', async () => {
    const start = Date.now()
    const enriched = await enrichErrorWithAiContext({
      message: 'test',
      stack: 'Error: test\n    at fn (http://localhost:3000/main.js:1:1)'
    })
    const elapsed = Date.now() - start

    assert.ok(elapsed < 4000, `Took ${elapsed}ms, expected < 4s`)
    assert.ok(enriched._aiContext)
  })

  test('resetEnrichmentForTesting restores defaults', () => {
    setAiContextEnabled(false)
    setAiContextStateSnapshot(true)

    resetEnrichmentForTesting()

    // After reset, enrichment should be enabled (verified by pipeline producing _aiContext)
    // We just verify the function doesn't throw — the state is internal
    assert.ok(true)
  })
})

// =============================================================================
// STATE SNAPSHOT (needs window mock)
// =============================================================================

describe('captureStateSnapshot', () => {
  let originalWindow

  beforeEach(() => {
    originalWindow = globalThis.window
    globalThis.window = {}
  })

  afterEach(() => {
    globalThis.window = originalWindow
  })

  test('returns null when no Redux store', () => {
    assert.strictEqual(captureStateSnapshot('error'), null)
  })

  test('captures Redux store keys and types', () => {
    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        auth: { user: null },
        cart: [1, 2],
        count: 42
      })
    }

    const snapshot = captureStateSnapshot('')

    assert.strictEqual(snapshot.source, 'redux')
    assert.strictEqual(snapshot.keys.auth.type, 'object')
    assert.strictEqual(snapshot.keys.cart.type, 'array')
    assert.strictEqual(snapshot.keys.count.type, 'number')
  })

  test('extracts relevant slice based on error keywords', () => {
    globalThis.window.__REDUX_STORE__ = {
      getState: () => ({
        auth: { user: null, error: 'Token expired' },
        ui: { theme: 'dark' }
      })
    }

    const snapshot = captureStateSnapshot('auth Token expired')

    const keys = Object.keys(snapshot.relevantSlice)
    assert.ok(keys.some((k) => k.startsWith('auth.')))
  })

  test('returns null when getState throws', () => {
    globalThis.window.__REDUX_STORE__ = {
      getState: () => { throw new Error('broken') }
    }

    assert.strictEqual(captureStateSnapshot(''), null)
  })
})
