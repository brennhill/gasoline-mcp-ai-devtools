/**
 * Purpose: Shared infrastructure for command dispatch -- result helpers, target tab resolution, action toast, and type aliases.
 */

// helpers.ts — Shared infrastructure for command dispatch.
// Types, result helpers, target resolution, action toast, and constants.

import type { PendingQuery } from '../../types/index.js'
import type { SyncClient } from '../sync-client.js'
import { sendTabToast } from '../tab-state.js'
import { DebugCategory } from '../debug.js'
import { isAiWebPilotEnabled } from '../state.js'
import { KABOOM_LOG_PREFIX } from '../../lib/brand.js'
import { errorMessage } from '../../lib/error-utils.js'

// Re-export target resolution symbols so existing consumers keep working
export {
  type QueryParamsObject,
  type TargetResolution,
  withTargetContext,
  requiresTargetTab,
  isBrowserEscapeAction,
  persistTrackedTab,
  resolveTargetTab,
  isRestrictedUrl
} from './target-resolution.js'
import type { QueryParamsObject } from './target-resolution.js'

// =============================================================================
// EXPORTED TYPE ALIASES (used by browser-actions.ts, dom-dispatch.ts, etc.)
// =============================================================================

/** Callback signature for sending async command results back through /sync */
export type SendAsyncResultFn = (
  syncClient: SyncClient,
  queryId: string,
  correlationId: string,
  status: 'complete' | 'error' | 'timeout' | 'cancelled',
  result?: unknown,
  error?: string
) => void

/** Callback signature for showing visual action toasts */
export type ActionToastFn = (
  tabId: number,
  text: string,
  detail?: string,
  state?: 'trying' | 'success' | 'warning' | 'error',
  durationMs?: number
) => void

export function debugLog(category: string, message: string, data: unknown = null): void {
  const globalLogger = (globalThis as { __KABOOM_DEBUG_LOG__?: (c: string, m: string, d?: unknown) => void })
    .__KABOOM_DEBUG_LOG__
  if (typeof globalLogger === 'function') {
    globalLogger(category, message, data)
    return
  }

  // Keep helpers usable before the main debug logger is initialized.
  const debugEnabled = (globalThis as { __KABOOM_REGISTRY_DEBUG__?: boolean }).__KABOOM_REGISTRY_DEBUG__ === true
  if (!debugEnabled) return
  const prefix = `${KABOOM_LOG_PREFIX.slice(0, -1)}:${category}]`
  if (data === null) {
    console.debug(`${prefix} ${message}`)
    return
  }
  console.debug(`${prefix} ${message}`, data)
}

function diagnosticLog(message: string): void {
  debugLog(DebugCategory.CONNECTION, message)
}

// =============================================================================
// RESULT HELPERS
// =============================================================================

/** Send a query result back through /sync */
export function sendResult(syncClient: SyncClient, queryId: string, result: unknown): void {
  debugLog(DebugCategory.CONNECTION, 'sendResult via /sync', { queryId, hasResult: result != null })
  syncClient.queueCommandResult({ id: queryId, status: 'complete', result })
}

/** Send an async command result back through /sync */
export function sendAsyncResult(
  syncClient: SyncClient,
  queryId: string,
  correlationId: string,
  status: 'complete' | 'error' | 'timeout' | 'cancelled',
  result?: unknown,
  error?: string
): void {
  debugLog(DebugCategory.CONNECTION, 'sendAsyncResult via /sync', {
    queryId,
    correlationId,
    status,
    hasResult: result != null,
    error: error || null
  })
  syncClient.queueCommandResult({
    id: queryId,
    correlation_id: correlationId,
    status,
    result,
    error
  })
}

// =============================================================================
// ACTION TOAST
// =============================================================================

/** Map raw action names to human-readable toast labels */
const PRETTY_LABELS: Record<string, string> = {
  navigate: 'Navigate to',
  refresh: 'Refresh',
  execute_js: 'Execute',
  click: 'Click',
  type: 'Type',
  select: 'Select',
  check: 'Check',
  focus: 'Focus',
  scroll_to: 'Scroll to',
  wait_for: 'Wait for',
  wait_for_stable: 'Waiting for page to stabilize...',
  key_press: 'Key press',
  highlight: 'Highlight',
  subtitle: 'Subtitle',
  upload: 'Upload file'
}

const PRETTY_TRYING_LABELS: Record<string, string> = {
  scroll_to: 'Scrolling to',
  open_composer: 'Opening composer',
  submit_active_composer: 'Submitting active composer',
  confirm_top_dialog: 'Confirming top dialog',
  dismiss_top_overlay: 'Dismissing top overlay',
  auto_dismiss_overlays: 'Dismissing overlays'
}

function humanizeActionLabel(action: string): string {
  const explicit = PRETTY_LABELS[action]
  if (explicit) return explicit
  if (!/^[a-z0-9]+(?:_[a-z0-9]+)+$/.test(action)) return action
  const sentence = action.replaceAll('_', ' ')
  return sentence.charAt(0).toUpperCase() + sentence.slice(1)
}

function inferWaitTarget(detail?: string): string | undefined {
  if (!detail) return undefined
  const trimmed = detail.trim()
  if (!trimmed || trimmed.toLowerCase() === 'page') return undefined
  return trimmed
}

function resolveToastCopy(
  action: string,
  detail: string | undefined,
  state: 'trying' | 'success' | 'warning' | 'error'
): { text: string; detail?: string } {
  if (state !== 'trying') return { text: humanizeActionLabel(action), detail }

  if (action === 'wait_for') {
    const waitTarget = inferWaitTarget(detail)
    if (waitTarget) return { text: `Waiting for ${waitTarget}` }
    return { text: 'Waiting for condition...' }
  }

  const tryingText = PRETTY_TRYING_LABELS[action]
  if (tryingText) return { text: tryingText, detail }
  return { text: humanizeActionLabel(action), detail }
}

/** Show a visual action toast on the tracked tab */
export function actionToast(
  tabId: number,
  action: string,
  detail?: string,
  state: 'trying' | 'success' | 'warning' | 'error' = 'success',
  durationMs = 3000
): void {
  const toastCopy = resolveToastCopy(action, detail, state)
  sendTabToast(tabId, toastCopy.text, toastCopy.detail ?? '', state, durationMs)
}

// =============================================================================
// PARAMS PARSING
// =============================================================================

export function parseQueryParamsObject(params: PendingQuery['params']): QueryParamsObject {
  if (typeof params === 'string') {
    try {
      const parsed = JSON.parse(params)
      if (parsed && typeof parsed === 'object') {
        return parsed as QueryParamsObject
      }
    } catch {
      return {}
    }
    return {}
  }
  if (params && typeof params === 'object') {
    return params as QueryParamsObject
  }
  return {}
}

// Target resolution extracted to ./target-resolution.ts (re-exported above)

// =============================================================================
// CONTENT SCRIPT ERROR DETECTION
// =============================================================================

/** Check if an error indicates the content script is not loaded on the target page. */
export function isContentScriptUnreachableError(err: unknown): boolean {
  const message = errorMessage(err, '')
  return message.includes('Receiving end does not exist') || message.includes('Could not establish connection')
}

// =============================================================================
// AI WEB PILOT GUARD
// =============================================================================

/**
 * Minimal context shape needed by requireAiWebPilot.
 * Avoids circular import with registry.ts (which defines CommandContext).
 */
interface AiWebPilotGuardContext {
  sendResult: (result: unknown) => void
}

/**
 * Guard that checks AI Web Pilot is enabled.
 * Returns true if enabled and the caller should proceed.
 * Returns false if disabled — the error response has already been sent.
 */
export function requireAiWebPilot(ctx: AiWebPilotGuardContext): boolean {
  if (isAiWebPilotEnabled()) return true
  ctx.sendResult({ error: 'ai_web_pilot_disabled' })
  return false
}
