/**
 * Purpose: Dispatches DOM actions (click, type, wait_for, list_interactive, query) to injected page scripts with frame targeting and CDP escalation.
 * Docs: docs/features/feature/interact-explore/index.md
 */

// dom-dispatch.ts — DOM action dispatcher and utilities.
// Extracted from dom-primitives.ts to reduce file size.
// Script builders stay self-contained because chrome.scripting.executeScript
// serializes injected functions independently.

import type { PendingQuery } from '../types/queries.js'
import type { SyncClient } from './sync-client.js'
import type { DOMActionParams, DOMResult } from './dom-types.js'
import type { SendAsyncResultFn, ActionToastFn } from './commands/helpers.js'
import { domFrameProbe } from './dom-frame-probe.js'
import { domPrimitive } from './dom-primitives.js'
import { domPrimitiveListInteractive } from './dom-primitives-list-interactive.js'
import { domPrimitiveQuery } from './dom-primitives-query.js'
import { isCDPEscalatable, tryCDPEscalation } from './cdp-dispatch.js'
import { normalizeFrameTarget } from '../lib/frame-utils.js'
import { isReadOnlyAction, isMutatingAction } from './action-metadata.js'

function parseDOMParams(query: PendingQuery): DOMActionParams | null {
  try {
    return typeof query.params === 'string' ? JSON.parse(query.params) : (query.params as DOMActionParams)
  } catch {
    return null
  }
}

function hasMatchedTargetEvidence(result: DOMResult): boolean {
  const matched = result.matched
  if (!matched || typeof matched !== 'object' || Array.isArray(matched)) return false
  return (
    typeof matched.selector === 'string' ||
    typeof matched.tag === 'string' ||
    typeof matched.element_id === 'string' ||
    typeof matched.aria_label === 'string' ||
    typeof matched.role === 'string' ||
    typeof matched.text_preview === 'string'
  )
}

type DOMExecutionTarget = { tabId: number; allFrames: true } | { tabId: number; frameIds: number[] }

async function resolveExecutionTarget(tabId: number, frame: unknown): Promise<DOMExecutionTarget> {
  const normalized = normalizeFrameTarget(frame)
  if (normalized === null) {
    throw new Error(
      'invalid_frame: frame parameter must be a CSS selector, 0-based index, or "all". Got unsupported type or value'
    )
  }

  if (normalized === undefined || normalized === 'all') {
    return { tabId, allFrames: true }
  }

  const probeResults = await chrome.scripting.executeScript({
    target: { tabId, allFrames: true },
    world: 'MAIN',
    func: domFrameProbe,
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

  return { tabId, frameIds }
}

/** Pick the best result from multi-frame executeScript. Prefers main frame, falls back to first success. */
function pickFrameResult(results: chrome.scripting.InjectionResult[]): { result: unknown; frameId: number } | null {
  const mainFrame = results.find((r) => r.frameId === 0)
  if (mainFrame?.result && (mainFrame.result as DOMResult).success) {
    return { result: mainFrame.result, frameId: 0 }
  }
  for (const r of results) {
    if (r.result && (r.result as DOMResult).success) {
      return { result: r.result, frameId: r.frameId }
    }
  }
  if (mainFrame?.result) return { result: mainFrame.result, frameId: 0 }
  return results[0] ? { result: results[0].result, frameId: results[0].frameId } : null
}

/** Merge list_interactive results from all frames (up to 100 elements). */
function mergeListInteractive(results: chrome.scripting.InjectionResult[]): {
  success: boolean
  elements: unknown[]
  candidate_count?: number
  scope_rect_used?: unknown
  error?: string
  message?: string
} {
  const elements: unknown[] = []
  let firstError: { error?: string; message?: string } | null = null
  let firstScopeRectUsed: unknown
  for (const r of results) {
    const res = r.result as {
      success?: boolean
      elements?: unknown[]
      scope_rect_used?: unknown
      error?: string
      message?: string
    } | null
    if (res?.success === false) {
      if (!firstError) firstError = { error: res.error, message: res.message }
      continue
    }
    if (firstScopeRectUsed === undefined && res?.scope_rect_used !== undefined) {
      firstScopeRectUsed = res.scope_rect_used
    }
    if (res?.elements) elements.push(...res.elements)
    if (elements.length >= 100) break
  }
  if (elements.length === 0 && firstError?.error) {
    return { success: false, elements: [], error: firstError.error, message: firstError.message }
  }
  const cappedElements = elements.slice(0, 100)
  const merged: {
    success: boolean
    elements: unknown[]
    candidate_count?: number
    scope_rect_used?: unknown
  } = {
    success: true,
    elements: cappedElements,
    candidate_count: cappedElements.length
  }
  if (firstScopeRectUsed !== undefined) {
    merged.scope_rect_used = firstScopeRectUsed
  }
  return merged
}

const WAIT_FOR_POLL_INTERVAL_MS = 80

function toDOMResult(value: unknown): DOMResult | null {
  if (!value || typeof value !== 'object') return null
  const candidate = value as DOMResult
  if (typeof candidate.success !== 'boolean') return null
  if (typeof candidate.action !== 'string' || typeof candidate.selector !== 'string') return null
  return candidate
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

/** Resolve which DOM action name to dispatch for wait_for based on params.
 *  Callers must validate mutual exclusivity before calling this. */
function resolveWaitForAction(params: DOMActionParams): string {
  if (params.absent) return 'wait_for_absent'
  if (params.text) return 'wait_for_text'
  return 'wait_for'
}

async function executeWaitForURL(tabId: number, params: DOMActionParams): Promise<DOMResult> {
  const urlSubstring = params.url_contains!
  const timeoutMs = Math.max(1, params.timeout_ms ?? 5000)
  const startedAt = Date.now()

  while (true) {
    const tab = await chrome.tabs.get(tabId)
    if (tab.url && tab.url.includes(urlSubstring)) {
      return {
        success: true,
        action: 'wait_for',
        selector: '',
        value: tab.url
      }
    }
    if (Date.now() - startedAt >= timeoutMs) {
      return {
        success: false,
        action: 'wait_for',
        selector: '',
        error: 'timeout',
        message: `URL did not contain "${urlSubstring}" within ${timeoutMs}ms`
      }
    }
    const remaining = timeoutMs - (Date.now() - startedAt)
    await wait(Math.min(WAIT_FOR_POLL_INTERVAL_MS, Math.max(1, remaining)))
  }
}

async function executeWaitFor(target: DOMExecutionTarget, params: DOMActionParams): Promise<DOMResult> {
  const selector = params.selector || ''
  const timeoutMs = Math.max(1, params.timeout_ms ?? 5000)
  const domAction = resolveWaitForAction(params)
  const domOpts = { timeout_ms: timeoutMs, text: params.text }
  const startedAt = Date.now()
  const quickCheck = await chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitive,
    args: [domAction, selector, domOpts]
  })
  const quickPicked = pickFrameResult(quickCheck)
  const quickResult = toDOMResult(quickPicked?.result)
  if (quickResult?.success) {
    return quickResult
  }

  let lastResult: DOMResult | null = toDOMResult(quickPicked?.result) ?? null
  while (Date.now() - startedAt < timeoutMs) {
    const remaining = timeoutMs - (Date.now() - startedAt)
    await wait(Math.min(WAIT_FOR_POLL_INTERVAL_MS, Math.max(1, remaining)))

    const probeResults = await chrome.scripting.executeScript({
      target,
      world: 'MAIN',
      func: domPrimitive,
      args: [domAction, selector, domOpts]
    })

    const picked = pickFrameResult(probeResults)
    const result = toDOMResult(picked?.result)
    if (result) lastResult = result
    if (result?.success) {
      return result
    }
  }

  const label =
    domAction === 'wait_for_text'
      ? `Text "${params.text}" not found within ${timeoutMs}ms`
      : domAction === 'wait_for_absent'
        ? `Element still present within ${timeoutMs}ms: ${selector}`
        : undefined
  if (lastResult?.error === 'timeout') {
    return lastResult
  }
  return {
    success: false,
    action: 'wait_for',
    selector,
    error: 'timeout',
    message: label || `Element not found within ${timeoutMs}ms: ${selector}`
  }
}

async function executeStandardAction(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitive,
    args: [
      params.action!,
      params.selector || '',
      {
        text: params.text,
        key: params.key,
        value: params.value,
        direction: params.direction,
        clear: params.clear,
        checked: params.checked,
        name: params.name,
        timeout_ms: params.timeout_ms,
        stability_ms: params.stability_ms,
        analyze: params.analyze,
        observe_mutations: params.observe_mutations,
        element_id: params.element_id,
        scope_selector: params.scope_selector,
        scope_rect: params.scope_rect,
        nth: params.nth,
        new_tab: params.new_tab,
        structured: params.structured
      }
    ]
  })
}

async function executeListInteractive(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  // Build options object with scope_rect and filter params (#369)
  const opts: Record<string, unknown> = {}
  if (params.scope_rect) opts.scope_rect = params.scope_rect
  if (params.text_contains) opts.text_contains = params.text_contains
  if (params.role) opts.role = params.role
  if (params.visible_only) opts.visible_only = params.visible_only
  if (params.exclude_nav) opts.exclude_nav = params.exclude_nav

  const hasOpts = Object.keys(opts).length > 0
  const args: [string] | [string, Record<string, unknown>] = hasOpts
    ? [params.selector || '', opts]
    : [params.selector || '']
  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveListInteractive,
    args
  })
}

// #370: Execute DOM query (exists, count, text, text_all, attributes)
async function executeQuery(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  const opts: Record<string, unknown> = {}
  if (params.query_type) opts.query_type = params.query_type
  if (params.attribute_names) opts.attribute_names = params.attribute_names
  if (params.scope_selector) opts.scope_selector = params.scope_selector

  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveQuery,
    args: [params.selector || '', Object.keys(opts).length > 0 ? opts : undefined]
  })
}

function sendToastForResult(
  tabId: number,
  readOnly: boolean,
  result: { success?: boolean; error?: string },
  actionToast: ActionToastFn,
  toastLabel: string,
  toastDetail: string | undefined
): void {
  if (readOnly) return
  if (result.success) {
    actionToast(tabId, toastLabel, toastDetail, 'success')
  } else {
    actionToast(tabId, toastLabel, result.error || 'failed', 'error')
  }
}

function reconcileDOMLifecycle(
  action: string,
  selector: string,
  result: unknown
): { result: unknown; status: 'complete' | 'error'; error?: string } {
  const domResult = toDOMResult(result)
  if (!domResult) {
    if (!isMutatingAction(action)) return { result, status: 'complete' }
    const coerced: DOMResult = {
      success: false,
      action,
      selector,
      error: 'status_mismatch',
      message: `Mutating action returned non-DOM payload: ${action}`
    }
    return { result: coerced, status: 'error', error: 'status_mismatch' }
  }

  if (!domResult.success) {
    return {
      result: domResult,
      status: 'error',
      error: domResult.error || domResult.message || 'dom_action_failed'
    }
  }

  if (domResult.error) {
    const coerced: DOMResult = {
      ...domResult,
      success: false,
      error: 'status_mismatch',
      message: `Payload marked success but includes error: ${domResult.error}`
    }
    return { result: coerced, status: 'error', error: 'status_mismatch' }
  }

  if (isMutatingAction(action) && !hasMatchedTargetEvidence(domResult)) {
    const coerced: DOMResult = {
      ...domResult,
      success: false,
      error: 'missing_match_evidence',
      message: `Mutating action completed without matched target evidence: ${action}`
    }
    return { result: coerced, status: 'error', error: 'missing_match_evidence' }
  }

  return { result: domResult, status: 'complete' }
}

function deriveAsyncStatusFromDOMResult(
  action: string,
  selector: string,
  result: unknown
): { result: unknown; status: 'complete' | 'error'; error?: string } {
  const reconciled = reconcileDOMLifecycle(action, selector, result)
  if (reconciled.status === 'complete') {
    return reconciled
  }
  return {
    status: 'error',
    error: reconciled.error || 'dom_action_failed',
    result: reconciled.result
  }
}

// Enrich results with effective tab context (post-execution URL).
// Agents compare resolved_url (dispatch time) vs effective_url (execution time) to detect drift.
async function enrichWithEffectiveContext(tabId: number, result: unknown): Promise<unknown> {
  try {
    const tab = await chrome.tabs.get(tabId)
    if (result && typeof result === 'object' && !Array.isArray(result)) {
      return {
        ...(result as Record<string, unknown>),
        effective_tab_id: tabId,
        effective_url: tab.url,
        effective_title: tab.title
      }
    }
    return result
  } catch {
    return result
  }
}

// #lizard forgives
export async function executeDOMAction(
  query: PendingQuery,
  tabId: number,
  syncClient: SyncClient,
  sendAsyncResult: SendAsyncResultFn,
  actionToast: ActionToastFn
): Promise<void> {
  const params = parseDOMParams(query)
  if (!params) {
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'invalid_params')
    return
  }

  const { action, selector, reason } = params
  if (!action) {
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'missing_action')
    return
  }
  if (action === 'wait_for') {
    const hasSelector = !!(selector || params.element_id)
    const hasText = !!params.text
    const hasURL = !!params.url_contains
    const condCount = (hasSelector || params.absent ? 1 : 0) + (hasText ? 1 : 0) + (hasURL ? 1 : 0)
    if (condCount === 0) {
      sendAsyncResult(
        syncClient,
        query.id,
        query.correlation_id!,
        'error',
        null,
        'wait_for requires selector, text, or url_contains'
      )
      return
    }
    if (condCount > 1) {
      sendAsyncResult(
        syncClient,
        query.id,
        query.correlation_id!,
        'error',
        null,
        'wait_for conditions are mutually exclusive'
      )
      return
    }
    if (params.absent && !hasSelector) {
      sendAsyncResult(
        syncClient,
        query.id,
        query.correlation_id!,
        'error',
        null,
        'wait_for with absent requires a selector'
      )
      return
    }
  }

  const toastLabel = reason || action
  const toastDetail = reason ? undefined : selector || 'page'
  const readOnly = isReadOnlyAction(action)

  // URL-based wait_for: polls chrome.tabs.get from background — no page injection needed.
  if (action === 'wait_for' && params.url_contains) {
    try {
      const urlResult = await executeWaitForURL(tabId, params)
      const status = urlResult.success ? 'complete' : 'error'
      sendAsyncResult(
        syncClient,
        query.id,
        query.correlation_id!,
        status,
        await enrichWithEffectiveContext(tabId, urlResult),
        urlResult.success ? undefined : urlResult.error
      )
    } catch (err) {
      actionToast(tabId, action, (err as Error).message, 'error')
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, (err as Error).message)
    }
    return
  }

  try {
    const executionTarget = await resolveExecutionTarget(tabId, params.frame)
    const tryingShownAt = Date.now()
    if (!readOnly) actionToast(tabId, toastLabel, toastDetail, 'trying', 10000)

    // CDP auto-escalation: try hardware events first for click/type/key_press (main frame only).
    // Falls back to DOM primitives silently if CDP is unavailable or fails.
    if (isCDPEscalatable(action) && !params.frame && params.nth === undefined) {
      try {
        const cdpResult = await tryCDPEscalation(tabId, action, params)
        if (cdpResult) {
          const {
            result: reconciledResult,
            status,
            error
          } = deriveAsyncStatusFromDOMResult(action, selector || '', cdpResult)
          const domResult = toDOMResult(reconciledResult)
          if (domResult) {
            sendToastForResult(tabId, false, domResult, actionToast, toastLabel, toastDetail)
          } else {
            actionToast(tabId, toastLabel, toastDetail, 'success')
          }
          sendAsyncResult(
            syncClient,
            query.id,
            query.correlation_id!,
            status,
            await enrichWithEffectiveContext(tabId, reconciledResult),
            error
          )
          return
        }
      } catch {
        // CDP failed — fall through to DOM primitives
      }
    }

    const rawResult =
      action === 'list_interactive'
        ? await executeListInteractive(executionTarget, params)
        : action === 'query'
          ? await executeQuery(executionTarget, params)
          : action === 'wait_for'
            ? await executeWaitFor(executionTarget, params)
            : await executeStandardAction(executionTarget, params)

    // wait_for quick-check can return a DOMResult directly
    if (!Array.isArray(rawResult)) {
      if (rawResult === null || rawResult === undefined) {
        if (!readOnly) actionToast(tabId, toastLabel, 'no result', 'error')
        sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'no_result')
        return
      }

      const {
        result: reconciledResult,
        status,
        error
      } = deriveAsyncStatusFromDOMResult(action, selector || '', rawResult)
      const domResult = toDOMResult(reconciledResult)
      if (domResult) {
        sendToastForResult(tabId, readOnly, domResult, actionToast, toastLabel, toastDetail)
      } else if (!readOnly && status === 'complete') {
        actionToast(tabId, toastLabel, toastDetail, 'success')
      } else if (!readOnly && status === 'error') {
        actionToast(tabId, toastLabel, error || 'failed', 'error')
      }

      sendAsyncResult(
        syncClient,
        query.id,
        query.correlation_id!,
        status,
        await enrichWithEffectiveContext(tabId, reconciledResult),
        error
      )
      return
    }

    // Ensure "trying" toast is visible for at least 500ms
    const MIN_TOAST_MS = 500
    const elapsed = Date.now() - tryingShownAt
    if (!readOnly && elapsed < MIN_TOAST_MS) await new Promise((r) => setTimeout(r, MIN_TOAST_MS - elapsed))

    // list_interactive: merge elements from all frames
    if (action === 'list_interactive') {
      const merged = mergeListInteractive(rawResult)
      const status = merged.success ? 'complete' : 'error'
      sendAsyncResult(
        syncClient,
        query.id,
        query.correlation_id!,
        status,
        await enrichWithEffectiveContext(tabId, merged),
        merged.success ? undefined : merged.error || 'list_interactive_failed'
      )
      return
    }

    const picked = pickFrameResult(rawResult)
    const firstResult = picked?.result
    if (firstResult && typeof firstResult === 'object') {
      let resultPayload: unknown
      if (picked) {
        const base: Record<string, unknown> = { ...(firstResult as Record<string, unknown>), frame_id: picked.frameId }
        const matched = base['matched']
        if (matched && typeof matched === 'object' && !Array.isArray(matched)) {
          base['matched'] = { ...(matched as Record<string, unknown>), frame_id: picked.frameId }
        }
        resultPayload = base
      } else {
        resultPayload = firstResult
      }
      const {
        result: reconciledResult,
        status,
        error
      } = deriveAsyncStatusFromDOMResult(action, selector || '', resultPayload)
      const domResult = toDOMResult(reconciledResult)
      if (domResult) {
        sendToastForResult(tabId, readOnly, domResult, actionToast, toastLabel, toastDetail)
      } else if (!readOnly && status === 'error') {
        actionToast(tabId, toastLabel, error || 'failed', 'error')
      }
      sendAsyncResult(
        syncClient,
        query.id,
        query.correlation_id!,
        status,
        await enrichWithEffectiveContext(tabId, reconciledResult),
        error
      )
    } else {
      if (!readOnly) actionToast(tabId, toastLabel, 'no result', 'error')
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'no_result')
    }
  } catch (err) {
    actionToast(tabId, action, (err as Error).message, 'error')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, (err as Error).message)
  }
}
