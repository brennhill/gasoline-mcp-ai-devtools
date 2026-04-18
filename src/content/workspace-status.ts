/**
 * Purpose: Collects deterministic workspace status heuristics from page state in the content script.
 * Why: Provides lightweight QA signals without requiring a full explicit audit on every workspace open.
 * Docs: docs/features/feature/terminal/index.md
 */

import { errorMessage } from '../lib/error-utils.js'
import type {
  WorkspaceContentStatusPayload,
  WorkspaceMetric,
  WorkspacePerformanceStatus,
  WorkspaceStatusHeuristicInput
} from '../types/workspace-status.js'

function clampScore(score: number): number {
  return Math.max(0, Math.min(100, Math.round(score)))
}

function metricStateFromScore(score: number): WorkspaceMetric['state'] {
  return score >= 80 ? 'healthy' : 'needs_attention'
}

function buildMetric(label: string, score: number, source: WorkspaceMetric['source'] = 'heuristic'): WorkspaceMetric {
  const normalized = clampScore(score)
  return {
    label,
    score: normalized,
    state: metricStateFromScore(normalized),
    source
  }
}

function buildUnavailableMetric(label: string): WorkspaceMetric {
  return {
    label,
    score: null,
    state: 'unavailable',
    source: 'unavailable'
  }
}

function buildUnavailablePerformance(): WorkspacePerformanceStatus {
  return {
    verdict: 'not_measured',
    source: 'unavailable'
  }
}

function collectSeoMetric(input: WorkspaceStatusHeuristicInput): WorkspaceMetric {
  let score = 0
  if (input.title.trim()) score += 30
  if ((input.metaDescription || '').trim()) score += 30
  if ((input.canonicalUrl || '').trim()) score += 20
  if (input.headings.some((heading) => heading.trim().length > 0)) score += 20
  return buildMetric('SEO', score)
}

function collectAccessibilityMetric(input: WorkspaceStatusHeuristicInput): WorkspaceMetric {
  let score = 0
  if (input.headings.some((heading) => heading.trim().length > 0)) score += 25

  const imageCount = input.images.length
  const labeledImages = input.images.filter((image) => (image.alt || '').trim().length > 0).length
  score += imageCount === 0 ? 35 : (35 * labeledImages) / imageCount

  const interactiveCount = input.interactiveLabels.length
  const labeledInteractive = input.interactiveLabels.filter((label) => label.trim().length > 0).length
  score += interactiveCount === 0 ? 40 : (40 * labeledInteractive) / interactiveCount

  return buildMetric('Accessibility', score)
}

function collectPerformanceStatus(input: WorkspaceStatusHeuristicInput): WorkspacePerformanceStatus {
  const domContentLoadedMs = input.navigationTiming?.domContentLoadedMs
  const loadMs = input.navigationTiming?.loadMs
  if (domContentLoadedMs === undefined && loadMs === undefined) {
    return { verdict: 'not_measured', source: 'unavailable' }
  }

  const domMs = domContentLoadedMs ?? loadMs ?? 0
  const fullLoadMs = loadMs ?? domMs
  if (domMs <= 1500 && fullLoadMs <= 3000) {
    return { verdict: 'good', source: 'heuristic' }
  }
  if (domMs <= 3000 && fullLoadMs <= 5000) {
    return { verdict: 'mixed', source: 'heuristic' }
  }
  return { verdict: 'poor', source: 'heuristic' }
}

function summarizeRecommendation(
  seo: WorkspaceMetric,
  accessibility: WorkspaceMetric,
  performance: WorkspacePerformanceStatus
): string {
  if (seo.state === 'healthy' && accessibility.state === 'healthy' && performance.verdict === 'good') {
    return 'Run an audit to capture authoritative QA results before shipping.'
  }
  if (performance.verdict === 'not_measured') {
    return 'Run an audit to capture performance evidence and confirm page health.'
  }
  return 'Run an audit to confirm metadata, accessibility, and performance findings.'
}

function readMetaContent(name: string): string {
  const selector = `meta[name="${name}"]`
  return (document.querySelector(selector)?.getAttribute('content') || '').trim()
}

function readCanonicalUrl(): string {
  return (document.querySelector('link[rel="canonical"]')?.getAttribute('href') || '').trim()
}

function readHeadingTexts(): string[] {
  return Array.from(document.querySelectorAll('h1, h2, h3'))
    .map((node) => node.textContent || '')
    .map((text) => text.trim())
    .filter(Boolean)
}

function readImageAlts(): Array<{ alt?: string }> {
  return Array.from(document.querySelectorAll('img')).map((image) => ({
    alt: image.getAttribute('alt') || undefined
  }))
}

function readInteractiveLabels(): string[] {
  return Array.from(document.querySelectorAll('button, a, input[type="button"], input[type="submit"]'))
    .map((node) => {
      const explicitLabel = node.getAttribute('aria-label') || node.getAttribute('title') || ''
      const text = node.textContent || ''
      return explicitLabel.trim() || text.trim()
    })
    .filter(Boolean)
}

function readNavigationTiming(): WorkspaceStatusHeuristicInput['navigationTiming'] {
  const navigationEntry = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined
  if (!navigationEntry) return undefined
  return {
    domContentLoadedMs: Math.round(navigationEntry.domContentLoadedEventEnd),
    loadMs: Math.round(navigationEntry.loadEventEnd)
  }
}

function buildInputFromDocument(): WorkspaceStatusHeuristicInput {
  return {
    title: document.title || '',
    url: location.href,
    metaDescription: readMetaContent('description'),
    canonicalUrl: readCanonicalUrl(),
    headings: readHeadingTexts(),
    images: readImageAlts(),
    interactiveLabels: readInteractiveLabels(),
    navigationTiming: readNavigationTiming()
  }
}

export function collectWorkspaceStatusHeuristics(input: WorkspaceStatusHeuristicInput): WorkspaceContentStatusPayload {
  const seo = input.url ? collectSeoMetric(input) : buildUnavailableMetric('SEO')
  const accessibility = input.url ? collectAccessibilityMetric(input) : buildUnavailableMetric('Accessibility')
  const performance = input.url ? collectPerformanceStatus(input) : buildUnavailablePerformance()

  return {
    seo,
    accessibility,
    performance,
    page: {
      title: input.title,
      url: input.url,
      summary: input.headings[0] || input.title || input.url
    },
    recommendation: summarizeRecommendation(seo, accessibility, performance)
  }
}

export function handleWorkspaceStatusQuery(
  sendResponse: (result: WorkspaceContentStatusPayload | { error: string; message: string }) => void
): boolean {
  try {
    sendResponse(collectWorkspaceStatusHeuristics(buildInputFromDocument()))
  } catch (err) {
    sendResponse({
      error: 'workspace_status_failed',
      message: errorMessage(err, 'Workspace status collection failed')
    })
  }
  return false
}
