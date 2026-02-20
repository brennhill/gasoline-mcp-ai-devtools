// helpers.ts — Shared infrastructure for command dispatch.
// Types, result helpers, target resolution, action toast, and constants.

import type { PendingQuery } from '../../types'
import type { SyncClient } from '../sync-client'
import * as eventListeners from '../event-listeners'
import * as index from '../index'
import { DebugCategory } from '../debug'

const { debugLog, diagnosticLog } = index

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

export type QueryParamsObject = Record<string, unknown>
type TargetResolutionSource = 'explicit_tab' | 'tracked_tab' | 'active_tab'

export interface TargetResolution {
  tabId: number
  url: string
  source: TargetResolutionSource
  requestedTabId?: number
  trackedTabId?: number | null
  useActiveTab: boolean
}

interface TargetResolutionError {
  payload: Record<string, unknown>
  message: string
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
  key_press: 'Key press',
  highlight: 'Highlight',
  subtitle: 'Subtitle',
  upload: 'Upload file'
}

/** Show a visual action toast on the tracked tab */
export function actionToast(
  tabId: number,
  action: string,
  detail?: string,
  state: 'trying' | 'success' | 'warning' | 'error' = 'success',
  durationMs = 3000
): void {
  chrome.tabs
    .sendMessage(tabId, {
      type: 'GASOLINE_ACTION_TOAST',
      text: PRETTY_LABELS[action] || action,
      detail,
      state,
      duration_ms: durationMs
    })
    .catch(() => {})
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

// =============================================================================
// TARGET RESOLUTION
// =============================================================================

export function withTargetContext(result: unknown, target: TargetResolution): Record<string, unknown> {
  const targetContext = {
    resolved_tab_id: target.tabId,
    resolved_url: target.url,
    target_context: {
      source: target.source,
      requested_tab_id: target.requestedTabId ?? null,
      tracked_tab_id: target.trackedTabId ?? null,
      use_active_tab: target.useActiveTab
    }
  }

  if (result && typeof result === 'object' && !Array.isArray(result)) {
    return {
      ...(result as Record<string, unknown>),
      ...targetContext
    }
  }

  return {
    value: result ?? null,
    ...targetContext
  }
}

const TARGETED_QUERY_TYPES = new Set<string>([
  'subtitle',
  'screenshot',
  'browser_action',
  'highlight',
  'page_info',
  'waterfall',
  'dom',
  'a11y',
  'dom_action',
  'upload',
  'record_start',
  'execute',
  'link_health',
  'draw_mode'
])

export function requiresTargetTab(queryType: string): boolean {
  return TARGETED_QUERY_TYPES.has(queryType)
}

async function getTabWithRetry(tabId: number, retry = false): Promise<chrome.tabs.Tab | null> {
  try {
    return await chrome.tabs.get(tabId)
  } catch {
    if (!retry) {
      return null
    }
    await new Promise((r) => setTimeout(r, 300))
    try {
      return await chrome.tabs.get(tabId)
    } catch {
      return null
    }
  }
}

async function getActiveTab(): Promise<chrome.tabs.Tab | null> {
  const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true })
  const tab = activeTabs[0]
  if (!tab?.id) {
    return null
  }
  return tab
}

function buildMissingTargetError(queryType: string, useActiveTab: boolean, trackedTabId: number | null): TargetResolutionError {
  const message =
    "No target tab resolved. Provide 'tab_id', enable tab tracking, or set 'use_active_tab=true' explicitly."
  return {
    message,
    payload: {
      success: false,
      error: 'missing_target',
      message,
      query_type: queryType,
      use_active_tab: useActiveTab,
      tracked_tab_id: trackedTabId
    }
  }
}

export async function resolveTargetTab(query: PendingQuery, paramsObj: QueryParamsObject): Promise<{
  target?: TargetResolution
  error?: TargetResolutionError
}> {
  const explicitTabId = typeof query.tab_id === 'number' && query.tab_id > 0 ? query.tab_id : undefined
  const useActiveTab = paramsObj.use_active_tab === true

  if (explicitTabId) {
    const explicitTab = await getTabWithRetry(explicitTabId)
    if (!explicitTab?.id) {
      const message = `Requested tab_id ${explicitTabId} is not available`
      return {
        error: {
          message,
          payload: {
            success: false,
            error: 'target_tab_not_found',
            message,
            requested_tab_id: explicitTabId
          }
        }
      }
    }
    return {
      target: {
        tabId: explicitTab.id,
        url: explicitTab.url || '',
        source: 'explicit_tab',
        requestedTabId: explicitTabId,
        trackedTabId: null,
        useActiveTab
      }
    }
  }

  if (useActiveTab) {
    const activeTab = await getActiveTab()
    if (!activeTab?.id) {
      return {
        error: {
          message: 'No active tab available',
          payload: {
            success: false,
            error: 'no_active_tab',
            message: 'No active tab available',
            use_active_tab: true
          }
        }
      }
    }
    return {
      target: {
        tabId: activeTab.id,
        url: activeTab.url || '',
        source: 'active_tab',
        trackedTabId: null,
        useActiveTab
      }
    }
  }

  const storage = await eventListeners.getTrackedTabInfo()
  const trackedTabId = storage.trackedTabId ?? null
  if (trackedTabId) {
    diagnosticLog(`[Diagnostic] Using tracked tab ${trackedTabId} for query ${query.type}`)
    const trackedTab = await getTabWithRetry(trackedTabId, true)
    if (trackedTab?.id) {
      return {
        target: {
          tabId: trackedTab.id,
          url: trackedTab.url || storage.trackedTabUrl || '',
          source: 'tracked_tab',
          trackedTabId,
          useActiveTab
        }
      }
    }

    diagnosticLog(`[Diagnostic] Tracked tab ${trackedTabId} unavailable, clearing tracking state`)
    eventListeners.clearTrackedTab()

    try {
      const toastTab = await getActiveTab()
      if (toastTab?.id) {
        chrome.tabs
          .sendMessage(toastTab.id, {
            type: 'GASOLINE_ACTION_TOAST',
            text: 'Tracked tab unavailable',
            detail: "Provide tab_id or use 'use_active_tab=true'",
            state: 'warning',
            duration_ms: 5000
          })
          .catch(() => {})
      }
    } catch {
      /* best effort */
    }

    return { error: buildMissingTargetError(query.type, useActiveTab, trackedTabId) }
  }

  return { error: buildMissingTargetError(query.type, useActiveTab, trackedTabId) }
}
