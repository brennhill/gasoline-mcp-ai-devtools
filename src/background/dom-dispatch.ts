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
function mergeListInteractive(results: chrome.scripting.InjectionResult[]): { success: boolean; elements: unknown[] } {
  const elements: unknown[] = []
  for (const r of results) {
    const res = r.result as { success?: boolean; elements?: unknown[] } | null
    if (res?.elements) elements.push(...res.elements)
    if (elements.length >= 100) break
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
        observe_mutations: params.observe_mutations
      }
    ]
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
      action === 'wait_for'
        ? await executeWaitFor(executionTarget, params)
        : await executeStandardAction(executionTarget, params)

    // wait_for quick-check can return a DOMResult directly
    if (!Array.isArray(rawResult)) {
      if (!readOnly) actionToast(tabId, toastLabel, toastDetail, 'success')
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', await enrichWithEffectiveContext(tabId, rawResult))
      return
    }

    // Ensure "trying" toast is visible for at least 500ms
    const MIN_TOAST_MS = 500
    const elapsed = Date.now() - tryingShownAt
    if (!readOnly && elapsed < MIN_TOAST_MS) await new Promise((r) => setTimeout(r, MIN_TOAST_MS - elapsed))

    // list_interactive: merge elements from all frames
    if (action === 'list_interactive') {
      const merged = mergeListInteractive(rawResult)
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', await enrichWithEffectiveContext(tabId, merged))
      return
    }

    const picked = pickFrameResult(rawResult)
    const firstResult = picked?.result
    if (firstResult && typeof firstResult === 'object') {
      const resultPayload =
        params.frame !== undefined && params.frame !== null && picked
          ? { ...(firstResult as Record<string, unknown>), frame_id: picked.frameId }
          : firstResult
      sendToastForResult(
        tabId,
        readOnly,
        resultPayload as { success?: boolean; error?: string },
        actionToast,
        toastLabel,
        toastDetail
      )
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', await enrichWithEffectiveContext(tabId, resultPayload))
    } else {
      if (!readOnly) actionToast(tabId, toastLabel, 'no result', 'error')
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, 'no_result')
    }
  } catch (err) {
    actionToast(tabId, action, (err as Error).message, 'error')
    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, (err as Error).message)
  }
}
