/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

// dom-dispatch.ts — DOM action dispatcher and utilities.
// Extracted from dom-primitives.ts to reduce file size.
// Script builders stay self-contained because chrome.scripting.executeScript
// serializes injected functions independently.

import type { PendingQuery } from '../types/queries'
import type { SyncClient } from './sync-client'
import type { DOMActionParams, DOMResult } from './dom-types'
import { domFrameProbe } from './dom-frame-probe'
import { domPrimitive } from './dom-primitives'
import { domPrimitiveListInteractive } from './dom-primitives-list-interactive'

type SendAsyncResult = (
  syncClient: SyncClient,
  queryId: string,
  correlationId: string,
  status: 'complete' | 'error' | 'timeout',
  result?: unknown,
  error?: string
) => void

type ActionToast = (
  tabId: number,
  text: string,
  detail?: string,
  state?: 'trying' | 'success' | 'warning' | 'error',
  durationMs?: number
) => void

function parseDOMParams(query: PendingQuery): DOMActionParams | null {
  try {
    return typeof query.params === 'string' ? JSON.parse(query.params) : (query.params as DOMActionParams)
  } catch {
    return null
  }
}

function isReadOnlyAction(action: string): boolean {
  return action === 'list_interactive' || action.startsWith('get_')
}

function isMutatingAction(action: string): boolean {
  return (
    action === 'click' ||
    action === 'type' ||
    action === 'select' ||
    action === 'check' ||
    action === 'set_attribute' ||
    action === 'paste' ||
    action === 'key_press' ||
    action === 'focus' ||
    action === 'scroll_to'
  )
}

function hasMatchedTargetEvidence(result: DOMResult): boolean {
  const matched = result.matched
  if (!matched || typeof matched !== 'object' || Array.isArray(matched)) return false
  return (
    typeof matched.selector === 'string' ||
    typeof matched.tag === 'string' ||
    typeof matched.aria_label === 'string' ||
    typeof matched.role === 'string' ||
    typeof matched.text_preview === 'string'
  )
}

type DOMExecutionTarget = { tabId: number; allFrames: true } | { tabId: number; frameIds: number[] }

function normalizeFrameTarget(frame: unknown): string | number | undefined | null {
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

async function resolveExecutionTarget(tabId: number, frame: unknown): Promise<DOMExecutionTarget> {
  const normalized = normalizeFrameTarget(frame)
  if (normalized === null) {
    throw new Error('invalid_frame')
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
    throw new Error('frame_not_found')
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
function mergeListInteractive(
  results: chrome.scripting.InjectionResult[]
): { success: boolean; elements: unknown[]; error?: string; message?: string } {
  const elements: unknown[] = []
  let firstError: { error?: string; message?: string } | null = null
  for (const r of results) {
    const res = r.result as { success?: boolean; elements?: unknown[]; error?: string; message?: string } | null
    if (res?.success === false) {
      if (!firstError) firstError = { error: res.error, message: res.message }
      continue
    }
    if (res?.elements) elements.push(...res.elements)
    if (elements.length >= 100) break
  }
  if (elements.length === 0 && firstError?.error) {
    return { success: false, elements: [], error: firstError.error, message: firstError.message }
  }
  return { success: true, elements: elements.slice(0, 100) }
}

const WAIT_FOR_POLL_INTERVAL_MS = 80

function toDOMResult(value: unknown): DOMResult | null {
  if (!value || typeof value !== 'object') return null
  const candidate = value as DOMResult
  if (typeof candidate.success !== 'boolean') return null
  if (typeof candidate.action !== 'string' || typeof candidate.selector !== 'string') return null
  return candidate
}

function withTimeoutResult(
  results: chrome.scripting.InjectionResult[],
  selector: string,
  timeoutMs: number
): chrome.scripting.InjectionResult[] {
  const timeoutResult: DOMResult = {
    success: false,
    action: 'wait_for',
    selector,
    error: 'timeout',
    message: `Element not found within ${timeoutMs}ms: ${selector}`
  }

  if (results.length === 0) {
    return [{ frameId: 0, result: timeoutResult } as chrome.scripting.InjectionResult]
  }
  return results.map((result) => ({ ...result, result: timeoutResult }))
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function executeWaitFor(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[] | DOMResult> {
  const selector = params.selector || ''
  const timeoutMs = Math.max(1, params.timeout_ms || 5000)
  const startedAt = Date.now()
  const quickCheck = await chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitive,
    args: [params.action!, selector, { timeout_ms: timeoutMs }]
  })
  const quickPicked = pickFrameResult(quickCheck)
  const quickResult = toDOMResult(quickPicked?.result)
  if (quickResult?.success) {
    return quickResult
  }

  let lastResults = quickCheck
  while (Date.now() - startedAt < timeoutMs) {
    await wait(Math.min(WAIT_FOR_POLL_INTERVAL_MS, timeoutMs))

    const probeResults = await chrome.scripting.executeScript({
      target,
      world: 'MAIN',
      func: domPrimitive,
      args: [params.action!, selector, { timeout_ms: timeoutMs }]
    })
    lastResults = probeResults

    const picked = pickFrameResult(probeResults)
    const result = toDOMResult(picked?.result)
    if (result?.success) {
      return probeResults
    }
  }

  return withTimeoutResult(lastResults, selector, timeoutMs)
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
        value: params.value,
        clear: params.clear,
        checked: params.checked,
        name: params.name,
        timeout_ms: params.timeout_ms,
        analyze: params.analyze,
        observe_mutations: params.observe_mutations,
        element_id: params.element_id,
        scope_selector: params.scope_selector
      }
    ]
  })
}

async function executeListInteractive(
  target: DOMExecutionTarget,
  params: DOMActionParams
): Promise<chrome.scripting.InjectionResult[]> {
  return chrome.scripting.executeScript({
    target,
    world: 'MAIN',
    func: domPrimitiveListInteractive,
    args: [params.selector || '']
  })
}

function sendToastForResult(
  tabId: number,
  readOnly: boolean,
  result: { success?: boolean; error?: string },
  actionToast: ActionToast,
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
      return { ...(result as Record<string, unknown>), effective_tab_id: tabId, effective_url: tab.url, effective_title: tab.title }
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
  sendAsyncResult: SendAsyncResult,
  actionToast: ActionToast
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
  if (action === 'wait_for' && !selector) {
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'missing_selector')
    return
  }

  const toastLabel = reason || action
  const toastDetail = reason ? undefined : selector || 'page'
  const readOnly = isReadOnlyAction(action)

  try {
    const executionTarget = await resolveExecutionTarget(tabId, params.frame)
    const tryingShownAt = Date.now()
    if (!readOnly) actionToast(tabId, toastLabel, toastDetail, 'trying', 10000)

    const rawResult =
      action === 'list_interactive'
        ? await executeListInteractive(executionTarget, params)
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

      const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(
        action,
        selector || '',
        rawResult
      )
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
      if (params.frame !== undefined && params.frame !== null && picked) {
        const base: Record<string, unknown> = { ...(firstResult as Record<string, unknown>), frame_id: picked.frameId }
        const matched = base["matched"]
        if (matched && typeof matched === 'object' && !Array.isArray(matched)) {
          base["matched"] = { ...(matched as Record<string, unknown>), frame_id: picked.frameId }
        }
        resultPayload = base
      } else {
        resultPayload = firstResult
      }
      const { result: reconciledResult, status, error } = deriveAsyncStatusFromDOMResult(
        action,
        selector || '',
        resultPayload
      )
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
