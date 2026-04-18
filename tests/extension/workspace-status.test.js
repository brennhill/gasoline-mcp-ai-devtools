// @ts-nocheck
/**
 * @fileoverview workspace-status.test.js — Regression coverage for workspace status snapshots and heuristics.
 */

import { describe, mock, test } from 'node:test'
import assert from 'node:assert'

describe('workspace status snapshot', () => {
  test('builds a live snapshot from deterministic heuristic signals', async () => {
    const { buildWorkspaceStatusSnapshot } = await import(`../../extension/background/workspace-status.js?live=1`)

    const snapshot = await buildWorkspaceStatusSnapshot({
      mode: 'live',
      tab: { id: 7, title: 'Checkout', url: 'https://tracked.example/checkout' },
      recordingState: { active: true },
      queryContentStatus: async () => ({
        seo: { score: 64, state: 'needs_attention', source: 'heuristic', label: 'SEO' },
        accessibility: { score: 71, state: 'needs_attention', source: 'heuristic', label: 'Accessibility' },
        performance: { verdict: 'mixed', source: 'heuristic' },
        page: {
          title: 'Checkout',
          url: 'https://tracked.example/checkout',
          summary: 'Checkout flow with a single-step form.'
        },
        recommendation: 'Run an audit to confirm field labels and metadata.'
      })
    })

    assert.strictEqual(snapshot.mode, 'live')
    assert.strictEqual(snapshot.seo.source, 'heuristic')
    assert.strictEqual(snapshot.accessibility.source, 'heuristic')
    assert.strictEqual(snapshot.performance.verdict, 'mixed')
    assert.strictEqual(snapshot.session.recording_active, true)
    assert.strictEqual(snapshot.page.title, 'Checkout')
  })

  test('builds an explicit audit snapshot with audit freshness metadata', async () => {
    const { buildWorkspaceStatusSnapshot } = await import(`../../extension/background/workspace-status.js?audit=1`)

    const snapshot = await buildWorkspaceStatusSnapshot({
      mode: 'audit',
      tab: { id: 9, title: 'Landing', url: 'https://tracked.example/' },
      recordingState: { active: false },
      audit: { updated_at: '2026-04-18T10:15:00.000Z' },
      queryContentStatus: async () => ({
        seo: { score: 88, state: 'healthy', source: 'audit', label: 'SEO' },
        accessibility: { score: 91, state: 'healthy', source: 'audit', label: 'Accessibility' },
        performance: { verdict: 'good', source: 'audit' },
        page: {
          title: 'Landing',
          url: 'https://tracked.example/',
          summary: 'Marketing landing page with hero, proof, and CTA.'
        },
        recommendation: 'Tighten the hero copy before shipping.'
      })
    })

    assert.strictEqual(snapshot.mode, 'audit')
    assert.ok(snapshot.audit.updated_at)
    assert.notStrictEqual(snapshot.seo.source, 'heuristic_only')
    assert.strictEqual(snapshot.performance.verdict, 'good')
  })

  test('does not synthesize audit freshness when no explicit audit result exists', async () => {
    globalThis.chrome = {
      tabs: {
        get: mock.fn((tabId) => Promise.resolve({
          id: tabId,
          title: 'Landing',
          url: 'https://tracked.example/'
        })),
        query: mock.fn(() => Promise.resolve([{
          id: 9,
          title: 'Landing',
          url: 'https://tracked.example/'
        }])),
        sendMessage: mock.fn(() => Promise.resolve({
          seo: { score: 88, state: 'healthy', source: 'audit', label: 'SEO' },
          accessibility: { score: 91, state: 'healthy', source: 'audit', label: 'Accessibility' },
          performance: { verdict: 'good', source: 'audit' },
          page: {
            title: 'Landing',
            url: 'https://tracked.example/',
            summary: 'Marketing landing page with hero, proof, and CTA.'
          },
          recommendation: 'Tighten the hero copy before shipping.'
        }))
      },
      storage: {
        local: {
          get: mock.fn((_keys, callback) => {
            callback({ kaboom_recording: { active: false } })
          })
        }
      }
    }

    const { getWorkspaceStatusSnapshot } = await import(`../../extension/background/workspace-status.js?freshness=1`)
    const snapshot = await getWorkspaceStatusSnapshot({ mode: 'audit', tabId: 9 })

    assert.strictEqual(snapshot.audit.updated_at, null)
    assert.strictEqual(snapshot.audit.state, 'unavailable')
  })

  test('falls back to unavailable states when the content bridge cannot provide signals', async () => {
    const { buildWorkspaceStatusSnapshot } = await import(`../../extension/background/workspace-status.js?fallback=1`)

    const snapshot = await buildWorkspaceStatusSnapshot({
      mode: 'live',
      tab: { id: 5, title: 'Broken', url: 'https://tracked.example/broken' },
      recordingState: { active: false },
      queryContentStatus: async () => {
        throw new Error('content unavailable')
      }
    })

    assert.strictEqual(snapshot.performance.verdict, 'not_measured')
    assert.strictEqual(snapshot.accessibility.state, 'unavailable')
    assert.strictEqual(snapshot.seo.state, 'unavailable')
  })
})

describe('workspace status heuristics', () => {
  test('derives seo, accessibility, and performance signals from page inputs', async () => {
    const { collectWorkspaceStatusHeuristics } = await import(`../../extension/content/workspace-status.js?heuristics=1`)

    const snapshot = collectWorkspaceStatusHeuristics({
      title: 'Pricing',
      url: 'https://tracked.example/pricing',
      metaDescription: 'Compare plans and pricing for tracked.example.',
      canonicalUrl: 'https://tracked.example/pricing',
      headings: ['Pricing'],
      images: [{ alt: 'Pricing table comparison' }],
      interactiveLabels: ['Start trial', 'Contact sales'],
      navigationTiming: { domContentLoadedMs: 1800, loadMs: 3200 }
    })

    assert.strictEqual(snapshot.seo.source, 'heuristic')
    assert.strictEqual(snapshot.accessibility.source, 'heuristic')
    assert.strictEqual(snapshot.performance.verdict, 'mixed')
    assert.match(snapshot.recommendation, /Run an audit/i)
  })
})
