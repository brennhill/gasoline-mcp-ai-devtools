/**
 * Purpose: Command handlers for the analyze MCP tool (DOM inspection, accessibility audits, link health, draw mode) with frame routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 */

// analyze.ts — Command handlers for the analyze MCP tool.
// Handles: dom, a11y, link_health, draw_mode.
// Includes frame routing helpers used by dom and a11y.

import { registerCommand } from './registry.js'
import { isContentScriptUnreachableError, requireAiWebPilot } from './helpers.js'
import { normalizeFrameTarget } from '../../lib/frame-utils.js'

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
  const normalized = normalizeFrameTarget(frame)
  if (normalized === null) {
    throw new Error(
      'invalid_frame: frame parameter must be a CSS selector, 0-based index, or "all". Got unsupported type or value'
    )
  }

  // No frame targeting requested — skip the probe entirely and target the main frame.
  if (normalized === undefined) {
    return { frameIds: [0], mode: 'main' }
  }

  // Pass null instead of undefined to satisfy chrome.scripting.executeScript serialization.
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
    throw new Error(
      'frame_not_found: no iframe matched the given selector or index. Verify the iframe exists and is loaded on the page'
    )
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

/**
 * Fallback DOM query implementation executed via chrome.scripting in ISOLATED world.
 * This keeps analyze(dom) working when content-script messaging is temporarily unavailable.
 */
function executeDOMQueryInIsolatedWorld(params: Record<string, unknown>): Record<string, unknown> {
  const selector = typeof params.selector === 'string' && params.selector.trim().length > 0 ? params.selector : '*'
  const includeStyles = params.include_styles === true
  const includeChildren = params.include_children === true
  const styleProps = (Array.isArray(params.properties) ? params.properties : []).filter(
    (prop): prop is string => typeof prop === 'string' && prop.length > 0
  )
  const rawDepth = typeof params.max_depth === 'number' ? params.max_depth : 3
  const maxDepth = Math.max(0, Math.min(5, Math.floor(rawDepth)))

  const MAX_ELEMENTS = 50
  const MAX_TEXT = 500

  const collectAttributes = (el: Element): Record<string, string> | undefined => {
    if (!el.attributes || el.attributes.length === 0) return undefined
    const attrs: Record<string, string> = {}
    for (const attr of Array.from(el.attributes)) {
      attrs[attr.name] = attr.value
    }
    return attrs
  }

  const serializeElement = (el: Element, depth: number): Record<string, unknown> => {
    const rect = el.getBoundingClientRect?.()
    const out: Record<string, unknown> = {
      tag: el.tagName?.toLowerCase() || '',
      text: (el.textContent || '').slice(0, MAX_TEXT),
      visible:
        (el as HTMLElement).offsetParent !== null ||
        (typeof rect?.width === 'number' && rect.width > 0) ||
        (typeof rect?.height === 'number' && rect.height > 0)
    }

    const attrs = collectAttributes(el)
    if (attrs) out.attributes = attrs

    if (rect) {
      out.boundingBox = {
        x: rect.x,
        y: rect.y,
        width: rect.width,
        height: rect.height
      }
    }

    if (includeStyles && typeof window.getComputedStyle === 'function') {
      const computed = window.getComputedStyle(el)
      if (styleProps.length > 0) {
        const styles: Record<string, string> = {}
        for (const prop of styleProps) {
          styles[prop] = computed.getPropertyValue(prop)
        }
        out.styles = styles
      } else {
        out.styles = {
          display: computed.display,
          color: computed.color,
          position: computed.position
        }
      }
    }

    if (includeChildren && depth < maxDepth && el.children.length > 0) {
      const children: Record<string, unknown>[] = []
      const childLimit = Math.min(el.children.length, MAX_ELEMENTS)
      for (let i = 0; i < childLimit; i++) {
        const child = el.children[i]
        if (child) children.push(serializeElement(child, depth + 1))
      }
      out.children = children
    }

    return out
  }

  try {
    const allMatches = Array.from(document.querySelectorAll(selector))
    const matches = allMatches.slice(0, MAX_ELEMENTS).map((el) => serializeElement(el, 0))
    return {
      url: window.location.href,
      title: document.title,
      matchCount: allMatches.length,
      returnedCount: matches.length,
      matches
    }
  } catch (err) {
    return {
      error: 'dom_query_failed',
      message: (err as Error)?.message || 'Failed to execute DOM query'
    }
  }
}

async function executeDOMQueryFallbackViaScripting(
  tabId: number,
  params: Record<string, unknown>,
  fallbackReason: string
): Promise<Record<string, unknown>> {
  const execution = await chrome.scripting.executeScript({
    target: { tabId },
    world: 'ISOLATED',
    func: executeDOMQueryInIsolatedWorld,
    args: [params]
  })

  const first = execution?.[0]?.result
  const payload = first && typeof first === 'object' ? (first as Record<string, unknown>) : {}
  return {
    ...payload,
    execution_world: 'ISOLATED',
    fallback_reason: fallbackReason
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
      let result: Record<string, unknown>
      try {
        result = (await chrome.tabs.sendMessage(ctx.tabId, {
          type: 'DOM_QUERY',
          params: ctx.query.params
        })) as Record<string, unknown>
      } catch (err) {
        const fallbackReason = isContentScriptUnreachableError(err)
          ? 'content_script_unreachable'
          : 'content_script_send_failed'
        try {
          result = await executeDOMQueryFallbackViaScripting(ctx.tabId, stripFrameParam(ctx.params), fallbackReason)
        } catch {
          throw err
        }
      }
      ctx.sendResult(result)
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

    ctx.sendResult(result)
  } catch (err) {
    const message = (err as Error).message || 'Failed to execute DOM query'
    console.error('[Gasoline][DOM] Command failed:', message, (err as Error).stack || err)
    const isFrameNotFound = message.startsWith('frame_not_found')
    const isInvalidFrame = message.startsWith('invalid_frame')
    ctx.sendResult({
      error: isInvalidFrame || isFrameNotFound ? message : 'dom_query_failed',
      message: isInvalidFrame || isFrameNotFound ? message : `Failed to execute DOM query: ${message}`
    })
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
      ctx.sendResult(result)
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

    ctx.sendResult(result)
  } catch (err) {
    const message = (err as Error).message || 'Failed to execute accessibility audit'
    console.error('[Gasoline][A11Y] Command failed:', message, (err as Error).stack || err)
    const isFrameNotFound = message.startsWith('frame_not_found')
    const isInvalidFrame = message.startsWith('invalid_frame')
    ctx.sendResult({
      error: isInvalidFrame || isFrameNotFound ? message : 'a11y_audit_failed',
      message: isInvalidFrame || isFrameNotFound ? message : `Failed to execute accessibility audit: ${message}`
    })
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

registerCommand('form_state', async (ctx) => {
  try {
    const result = await chrome.tabs.sendMessage(ctx.tabId, {
      type: 'FORM_STATE_QUERY',
      params: ctx.query.params
    })
    ctx.sendResult(result)
  } catch (err) {
    ctx.sendResult({
      error: 'form_state_failed',
      message: (err as Error).message || 'Form state extraction failed'
    })
  }
})

registerCommand('data_table', async (ctx) => {
  try {
    const result = await chrome.tabs.sendMessage(ctx.tabId, {
      type: 'DATA_TABLE_QUERY',
      params: ctx.query.params
    })
    ctx.sendResult(result)
  } catch (err) {
    ctx.sendResult({
      error: 'data_table_failed',
      message: (err as Error).message || 'Data table extraction failed'
    })
  }
})

// =============================================================================
// DRAW MODE
// =============================================================================

registerCommand('draw_mode', async (ctx) => {
  if (!requireAiWebPilot(ctx)) return
  const params = ctx.params as { action?: string; annot_session?: string }
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

// Navigation command handler extracted to analyze-navigation.ts (#335)
