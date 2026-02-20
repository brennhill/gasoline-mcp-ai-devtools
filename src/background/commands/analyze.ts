// analyze.ts — Command handlers for the analyze MCP tool.
// Handles: dom, a11y, link_health, draw_mode.
// Includes frame routing helpers used by dom and a11y.

import * as index from '../index'
import { registerCommand } from './registry'

// =============================================================================
// FRAME ROUTING TYPES
// =============================================================================

type AnalyzeFrameTarget = string | number | undefined

interface AnalyzeFrameSelection {
  frameIds: number[]
  mode: 'main' | 'all' | 'targeted'
}

interface FrameQueryResult<T = unknown> {
  frame_id: number
  result?: T
  error?: string
}

// =============================================================================
// FRAME ROUTING HELPERS
// =============================================================================

function normalizeAnalyzeFrameTarget(frame: unknown): AnalyzeFrameTarget | null {
  if (frame === undefined || frame === null) return undefined

  if (typeof frame === 'number') {
    if (!Number.isInteger(frame) || frame < 0) return null
    return frame
  }

  if (typeof frame === 'string') {
    const trimmed = frame.trim()
    if (trimmed.length === 0) return null
    return trimmed
  }

  return null
}

/**
 * Frame selection probe executed in page context.
 * Must be self-contained for chrome.scripting.executeScript({ func }).
 */
function analyzeFrameProbe(frameTarget: AnalyzeFrameTarget): { matches: boolean } {
  const isTop = window === window.top

  const getParentFrameIndex = (): number => {
    if (isTop) return -1
    try {
      const parentFrames = window.parent?.frames
      if (!parentFrames) return -1
      for (let i = 0; i < parentFrames.length; i++) {
        if (parentFrames[i] === window) return i
      }
    } catch {
      return -1
    }
    return -1
  }

  if (frameTarget === undefined) {
    return { matches: isTop }
  }

  if (frameTarget === 'all') {
    return { matches: true }
  }

  if (typeof frameTarget === 'number') {
    return { matches: getParentFrameIndex() === frameTarget }
  }

  if (isTop) {
    return { matches: false }
  }

  try {
    const frameEl = window.frameElement
    if (!frameEl || typeof frameEl.matches !== 'function') {
      return { matches: false }
    }
    return { matches: frameEl.matches(frameTarget) }
  } catch {
    return { matches: false }
  }
}

async function resolveAnalyzeFrameSelection(tabId: number, frame: unknown): Promise<AnalyzeFrameSelection> {
  const normalized = normalizeAnalyzeFrameTarget(frame)
  if (normalized === null) {
    throw new Error('invalid_frame')
  }

  const probeResults = await chrome.scripting.executeScript({
    target: { tabId, allFrames: true },
    world: 'MAIN',
    func: analyzeFrameProbe,
    args: [normalized]
  })

  const frameIds = Array.from(
    new Set(
      probeResults
        .filter((r) => !!(r.result as { matches?: boolean } | undefined)?.matches)
        .map((r) => r.frameId)
        .filter((id): id is number => typeof id === 'number')
    )
  )

  if (frameIds.length === 0) {
    throw new Error('frame_not_found')
  }

  if (normalized === undefined) {
    return { frameIds, mode: 'main' }
  }
  if (normalized === 'all') {
    return { frameIds, mode: 'all' }
  }
  return { frameIds, mode: 'targeted' }
}

function stripFrameParam(params: Record<string, unknown>): Record<string, unknown> {
  const copy = { ...params }
  delete copy.frame
  return copy
}

async function sendFrameQueries<T>(
  tabId: number,
  frameIds: number[],
  message: Record<string, unknown>
): Promise<FrameQueryResult<T>[]> {
  return Promise.all(
    frameIds.map(async (frameId) => {
      try {
        const result = (await chrome.tabs.sendMessage(tabId, message, { frameId })) as T
        return { frame_id: frameId, result }
      } catch (err) {
        return {
          frame_id: frameId,
          error: (err as Error).message || 'frame_query_failed'
        }
      }
    })
  )
}

function toNonNegativeInt(value: unknown): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) return 0
  const n = Math.floor(value)
  return n > 0 ? n : 0
}

function aggregateDOMFrameResults(results: FrameQueryResult<Record<string, unknown>>[]): Record<string, unknown> {
  const MAX_MATCHES = 200
  const matches: unknown[] = []
  const frames: Record<string, unknown>[] = []
  let totalMatchCount = 0
  let totalReturnedCount = 0
  let url = ''
  let title = ''

  for (const entry of results) {
    if (entry.error) {
      frames.push({ frame_id: entry.frame_id, error: entry.error })
      continue
    }

    const payload = entry.result || {}
    const frameMatchCount = toNonNegativeInt(payload.matchCount)
    const frameReturnedCount = toNonNegativeInt(payload.returnedCount)
    const frameMatches = Array.isArray(payload.matches) ? payload.matches : []

    if (!url && typeof payload.url === 'string') {
      url = payload.url
    }
    if (!title && typeof payload.title === 'string') {
      title = payload.title
    }

    totalMatchCount += frameMatchCount
    totalReturnedCount += frameReturnedCount
    if (matches.length < MAX_MATCHES) {
      matches.push(...frameMatches.slice(0, MAX_MATCHES - matches.length))
    }

    frames.push({
      frame_id: entry.frame_id,
      match_count: frameMatchCount,
      returned_count: frameReturnedCount,
      ...(payload.error ? { error: payload.error } : {})
    })
  }

  return {
    url,
    title,
    matchCount: totalMatchCount,
    returnedCount: totalReturnedCount,
    matches,
    frames
  }
}

function aggregateA11yFrameResults(results: FrameQueryResult<Record<string, unknown>>[]): Record<string, unknown> {
  const violations: unknown[] = []
  const passes: unknown[] = []
  const incomplete: unknown[] = []
  const inapplicable: unknown[] = []
  const frames: Record<string, unknown>[] = []
  const errors: string[] = []

  for (const entry of results) {
    if (entry.error) {
      frames.push({ frame_id: entry.frame_id, error: entry.error })
      errors.push(entry.error)
      continue
    }

    const payload = entry.result || {}
    const frameViolations = Array.isArray(payload.violations) ? payload.violations : []
    const framePasses = Array.isArray(payload.passes) ? payload.passes : []
    const frameIncomplete = Array.isArray(payload.incomplete) ? payload.incomplete : []
    const frameInapplicable = Array.isArray(payload.inapplicable) ? payload.inapplicable : []

    violations.push(...frameViolations)
    passes.push(...framePasses)
    incomplete.push(...frameIncomplete)
    inapplicable.push(...frameInapplicable)

    const frameSummary = payload.summary as Record<string, unknown> | undefined
    frames.push({
      frame_id: entry.frame_id,
      summary: {
        violations: toNonNegativeInt(frameSummary?.violations ?? frameViolations.length),
        passes: toNonNegativeInt(frameSummary?.passes ?? framePasses.length),
        incomplete: toNonNegativeInt(frameSummary?.incomplete ?? frameIncomplete.length),
        inapplicable: toNonNegativeInt(frameSummary?.inapplicable ?? frameInapplicable.length)
      },
      ...(payload.error ? { error: payload.error } : {})
    })

    if (typeof payload.error === 'string' && payload.error.length > 0) {
      errors.push(payload.error)
    }
  }

  return {
    violations,
    passes,
    incomplete,
    inapplicable,
    summary: {
      violations: violations.length,
      passes: passes.length,
      incomplete: incomplete.length,
      inapplicable: inapplicable.length
    },
    frames,
    ...(errors.length > 0 ? { error: errors.join('; ') } : {})
  }
}

// =============================================================================
// DOM
// =============================================================================

registerCommand('dom', async (ctx) => {
  try {
    const frameSelection = await resolveAnalyzeFrameSelection(ctx.tabId, ctx.params.frame)

    // Fast path: preserve legacy behavior when no frame is specified.
    if (frameSelection.mode === 'main') {
      const result = await chrome.tabs.sendMessage(ctx.tabId, {
        type: 'DOM_QUERY',
        params: ctx.query.params
      })
      if (ctx.query.correlation_id) {
        ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', result)
      } else {
        ctx.sendResult(result)
      }
      return
    }

    const frameParams = stripFrameParam(ctx.params)
    const perFrame = await sendFrameQueries<Record<string, unknown>>(ctx.tabId, frameSelection.frameIds, {
      type: 'DOM_QUERY',
      params: frameParams
    })

    let result: Record<string, unknown>
    if (perFrame.length === 1) {
      const first = perFrame[0]
      if (!first) {
        result = { error: 'dom_query_failed', message: 'No frame response received' }
      } else if (first.error) {
        result = { error: 'dom_query_failed', message: first.error, frame_id: first.frame_id }
      } else {
        result = { ...(first.result || {}), frame_id: first.frame_id }
      }
    } else {
      result = aggregateDOMFrameResults(perFrame)
    }

    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', result)
    } else {
      ctx.sendResult(result)
    }
  } catch (err) {
    const message = (err as Error).message || 'Failed to execute DOM query'
    const isFrameNotFound = message === 'frame_not_found'
    const isInvalidFrame = message === 'invalid_frame'
    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(
        ctx.syncClient,
        ctx.query.id,
        ctx.query.correlation_id,
        'error',
        null,
        isInvalidFrame || isFrameNotFound ? message : 'Failed to execute DOM query'
      )
    } else {
      ctx.sendResult({
        error: isInvalidFrame || isFrameNotFound ? message : 'dom_query_failed',
        message: isInvalidFrame || isFrameNotFound ? message : 'Failed to execute DOM query'
      })
    }
  }
})

// =============================================================================
// A11Y
// =============================================================================

registerCommand('a11y', async (ctx) => {
  try {
    const frameSelection = await resolveAnalyzeFrameSelection(ctx.tabId, ctx.params.frame)

    // Fast path: preserve legacy behavior when no frame is specified.
    if (frameSelection.mode === 'main') {
      const result = await chrome.tabs.sendMessage(ctx.tabId, {
        type: 'A11Y_QUERY',
        params: ctx.query.params
      })
      if (ctx.query.correlation_id) {
        ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', result)
      } else {
        ctx.sendResult(result)
      }
      return
    }

    const frameParams = stripFrameParam(ctx.params)
    const perFrame = await sendFrameQueries<Record<string, unknown>>(ctx.tabId, frameSelection.frameIds, {
      type: 'A11Y_QUERY',
      params: frameParams
    })

    let result: Record<string, unknown>
    if (perFrame.length === 1) {
      const first = perFrame[0]
      if (!first) {
        result = { error: 'a11y_audit_failed', message: 'No frame response received' }
      } else if (first.error) {
        result = { error: 'a11y_audit_failed', message: first.error, frame_id: first.frame_id }
      } else {
        result = { ...(first.result || {}), frame_id: first.frame_id }
      }
    } else {
      result = aggregateA11yFrameResults(perFrame)
    }

    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', result)
    } else {
      ctx.sendResult(result)
    }
  } catch (err) {
    const message = (err as Error).message || 'Failed to execute accessibility audit'
    const isFrameNotFound = message === 'frame_not_found'
    const isInvalidFrame = message === 'invalid_frame'
    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(
        ctx.syncClient,
        ctx.query.id,
        ctx.query.correlation_id,
        'error',
        null,
        isInvalidFrame || isFrameNotFound ? message : 'Failed to execute accessibility audit'
      )
    } else {
      ctx.sendResult({
        error: isInvalidFrame || isFrameNotFound ? message : 'a11y_audit_failed',
        message: isInvalidFrame || isFrameNotFound ? message : 'Failed to execute accessibility audit'
      })
    }
  }
})

// =============================================================================
// LINK HEALTH
// =============================================================================

registerCommand('link_health', async (ctx) => {
  try {
    const result = await chrome.tabs.sendMessage(ctx.tabId, {
      type: 'LINK_HEALTH_QUERY',
      params: ctx.query.params
    })
    ctx.sendResult(result)
  } catch (err) {
    ctx.sendResult({
      error: 'link_health_failed',
      message: (err as Error).message || 'Link health check failed'
    })
  }
})

// =============================================================================
// COMPUTED STYLES
// =============================================================================

registerCommand('computed_styles', async (ctx) => {
  try {
    const result = await chrome.tabs.sendMessage(ctx.tabId, {
      type: 'COMPUTED_STYLES_QUERY',
      params: ctx.query.params
    })
    ctx.sendResult(result)
  } catch (err) {
    ctx.sendResult({
      error: 'computed_styles_failed',
      message: (err as Error).message || 'Computed styles query failed'
    })
  }
})

// =============================================================================
// FORM DISCOVERY
// =============================================================================

registerCommand('form_discovery', async (ctx) => {
  try {
    const result = await chrome.tabs.sendMessage(ctx.tabId, {
      type: 'FORM_DISCOVERY_QUERY',
      params: ctx.query.params
    })
    ctx.sendResult(result)
  } catch (err) {
    ctx.sendResult({
      error: 'form_discovery_failed',
      message: (err as Error).message || 'Form discovery failed'
    })
  }
})

// =============================================================================
// DRAW MODE
// =============================================================================

registerCommand('draw_mode', async (ctx) => {
  if (!index.__aiWebPilotEnabledCache) {
    ctx.sendResult({
      error: 'ai_web_pilot_disabled',
      message: 'AI Web Pilot is not enabled in the extension popup'
    })
    return
  }
  let params: { action?: string; annot_session?: string }
  try {
    params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params
  } catch {
    params = {}
  }
  if (params.action === 'start') {
    try {
      const result = await chrome.tabs.sendMessage(ctx.tabId, {
        type: 'GASOLINE_DRAW_MODE_START',
        started_by: 'llm',
        annot_session_name: params.annot_session || '',
        correlation_id: ctx.query.correlation_id || ctx.query.id || ''
      })
      ctx.sendResult({
        status: result?.status || 'active',
        message:
          'Draw mode activated. User can now draw annotations on the page. Results will be delivered when user finishes (presses ESC).',
        annotation_count: result?.annotation_count || 0
      })
    } catch (err) {
      ctx.sendResult({
        error: 'draw_mode_failed',
        message:
          (err as Error).message ||
          'Failed to activate draw mode. Ensure content script is loaded (try refreshing the page).'
      })
    }
  } else {
    ctx.sendResult({
      error: 'unknown_draw_mode_action',
      message: `Unknown draw mode action: ${params.action}. Use 'start'.`
    })
  }
})
