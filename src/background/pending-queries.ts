/**
 * @fileoverview Pending Query Handlers
 * Handles all query types from the server: DOM, accessibility, browser actions,
 * execute commands, and state management.
 *
 * All results are returned via syncClient.queueCommandResult() which routes them
 * through the unified /sync endpoint. No direct HTTP POSTs to legacy endpoints.
 *
 * Split into modules:
 * - query-execution.ts: JS execution with world-aware routing and CSP fallback
 * - browser-actions.ts: Browser navigation/action handlers with async timeout support
 */

import type { PendingQuery } from '../types'
import type { SyncClient } from './sync-client'
import * as eventListeners from './event-listeners'
import * as index from './index'
import { DebugCategory } from './debug'
import {
  saveStateSnapshot,
  loadStateSnapshot,
  listStateSnapshots,
  deleteStateSnapshot
} from './message-handlers'
import { executeDOMAction } from './dom-primitives'
import { canTakeScreenshot, recordScreenshot } from './state-manager'
import { startRecording, stopRecording } from './recording'
import { executeWithWorldRouting } from './query-execution'
import {
  handleBrowserAction,
  handleAsyncBrowserAction,
  handleAsyncExecuteCommand
} from './browser-actions'

// Extract values from index for easier reference (but NOT DebugCategory - imported directly above)
const { debugLog, diagnosticLog } = index

// =============================================================================
// EXPORTED TYPE ALIASES (used by browser-actions.ts and dom-primitives.ts)
// =============================================================================

/** Callback signature for sending async command results back through /sync */
export type SendAsyncResultFn = (
  syncClient: SyncClient,
  queryId: string,
  correlationId: string,
  status: 'complete' | 'error' | 'timeout',
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

// =============================================================================
// RESULT HELPERS
// =============================================================================

/** Send a query result back through /sync */
function sendResult(syncClient: SyncClient, queryId: string, result: unknown): void {
  debugLog(DebugCategory.CONNECTION, 'sendResult via /sync', { queryId, hasResult: result != null })
  syncClient.queueCommandResult({ id: queryId, status: 'complete', result })
}

/** Send an async command result back through /sync */
function sendAsyncResult(
  syncClient: SyncClient,
  queryId: string,
  correlationId: string,
  status: 'complete' | 'error' | 'timeout',
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
  subtitle: 'Subtitle'
}

/** Show a visual action toast on the tracked tab */
function actionToast(
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
// PENDING QUERY HANDLING
// =============================================================================

export async function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void> {
  // Wait for initialization to complete (max 2s) so pilot cache is populated
  await Promise.race([index.initReady, new Promise((r) => setTimeout(r, 2000))])

  debugLog(DebugCategory.CONNECTION, 'handlePendingQuery ENTER', {
    id: query.id,
    type: query.type,
    correlation_id: query.correlation_id || null,
    hasSyncClient: !!syncClient
  })
  try {
    if (query.type.startsWith('state_')) {
      await handleStateQuery(query, syncClient)
      return
    }

    const storage = await eventListeners.getTrackedTabInfo()
    let tabId: number | undefined

    if (storage.trackedTabId) {
      diagnosticLog(`[Diagnostic] Using tracked tab ${storage.trackedTabId} for query ${query.type}`)
      try {
        await chrome.tabs.get(storage.trackedTabId)
        tabId = storage.trackedTabId
      } catch {
        // Retry once after delay — tabs.get can fail transiently during SW wakeup or navigation
        await new Promise((r) => setTimeout(r, 300))
        try {
          await chrome.tabs.get(storage.trackedTabId)
          tabId = storage.trackedTabId
          diagnosticLog(`[Diagnostic] Tracked tab ${storage.trackedTabId} recovered on retry`)
        } catch {
          diagnosticLog(`[Diagnostic] Tracked tab ${storage.trackedTabId} confirmed gone, clearing tracking`)
          eventListeners.clearTrackedTab()
          // Show toast on the active tab so user knows tracking was lost
          try {
            const toastTabs = await chrome.tabs.query({ active: true, currentWindow: true })
            if (toastTabs[0]?.id) {
              chrome.tabs
                .sendMessage(toastTabs[0].id, {
                  type: 'GASOLINE_ACTION_TOAST',
                  text: 'Tracked tab closed',
                  detail: 'Re-enable tracking in Gasoline popup',
                  state: 'warning',
                  duration_ms: 5000
                })
                .catch(() => {})
            }
          } catch {
            /* best effort */
          }
          const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true })
          const firstActiveTab = activeTabs[0]
          if (!firstActiveTab?.id) return
          tabId = firstActiveTab.id
        }
      }
    } else {
      const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true })
      const firstActiveTab = activeTabs[0]
      if (!firstActiveTab?.id) return
      tabId = firstActiveTab.id
    }

    if (!tabId) return

    if (query.type === 'subtitle') {
      let params: { text?: string }
      try {
        params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
      } catch {
        params = {}
      }
      chrome.tabs
        .sendMessage(tabId, {
          type: 'GASOLINE_SUBTITLE',
          text: params.text ?? ''
        })
        .catch(() => {})
      sendResult(syncClient, query.id, { success: true, subtitle: params.text || 'cleared' })
      return
    }

    if (query.type === 'screenshot') {
      try {
        const rateCheck = canTakeScreenshot(tabId)
        if (!rateCheck.allowed) {
          sendResult(syncClient, query.id, {
            error: `Rate limited: ${rateCheck.reason}`,
            ...(rateCheck.nextAllowedIn != null ? { next_allowed_in: rateCheck.nextAllowedIn } : {})
          })
          return
        }

        const tab = await chrome.tabs.get(tabId)
        const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
          format: 'jpeg',
          quality: 80
        })
        recordScreenshot(tabId)

        // POST to /screenshots with query_id — server saves file and resolves query directly
        const response = await fetch(`${index.serverUrl}/screenshots`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            data_url: dataUrl,
            url: tab.url,
            query_id: query.id
          })
        })
        if (!response.ok) {
          sendResult(syncClient, query.id, { error: `Server returned ${response.status}` })
        }
        // No sendResult needed — server resolves the query via query_id
      } catch (err) {
        sendResult(syncClient, query.id, {
          error: 'screenshot_failed',
          message: (err as Error).message || 'Failed to capture screenshot'
        })
      }
      return
    }

    if (query.type === 'browser_action') {
      let params: { action?: string; url?: string; reason?: string }
      try {
        params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
      } catch {
        sendResult(syncClient, query.id, {
          success: false,
          error: 'invalid_params',
          message: 'Failed to parse browser_action params as JSON'
        })
        return
      }
      if (query.correlation_id) {
        await handleAsyncBrowserAction(query, tabId, params, syncClient, sendAsyncResult, actionToast)
      } else {
        const result = await handleBrowserAction(tabId, params, actionToast)
        sendResult(syncClient, query.id, result)
      }
      return
    }

    if (query.type === 'highlight') {
      let params: unknown
      try {
        params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
      } catch {
        sendResult(syncClient, query.id, {
          error: 'invalid_params',
          message: 'Failed to parse highlight params as JSON'
        })
        return
      }
      const result = await handlePilotCommand('GASOLINE_HIGHLIGHT', params)
      sendResult(syncClient, query.id, result)
      return
    }

    if (query.type === 'page_info') {
      const tab = await chrome.tabs.get(tabId)
      const result = {
        url: tab.url,
        title: tab.title,
        favicon: tab.favIconUrl,
        status: tab.status,
        viewport: {
          width: tab.width,
          height: tab.height
        }
      }
      sendResult(syncClient, query.id, result)
      return
    }

    if (query.type === 'tabs') {
      const allTabs = await chrome.tabs.query({})
      const tabsList = allTabs.map((tab) => ({
        id: tab.id,
        url: tab.url,
        title: tab.title,
        active: tab.active,
        windowId: tab.windowId,
        index: tab.index
      }))
      sendResult(syncClient, query.id, { tabs: tabsList })
      return
    }

    // Waterfall query - fetch network waterfall data on demand
    if (query.type === 'waterfall') {
      debugLog(DebugCategory.CAPTURE, 'Handling waterfall query', { queryId: query.id, tabId })
      try {
        const tab = await chrome.tabs.get(tabId)
        debugLog(DebugCategory.CAPTURE, 'Got tab for waterfall', { tabId, url: tab.url })
        const result = (await chrome.tabs.sendMessage(tabId, {
          type: 'GET_NETWORK_WATERFALL'
        })) as { entries?: unknown[] }
        debugLog(DebugCategory.CAPTURE, 'Waterfall result from content script', {
          entries: result?.entries?.length || 0
        })

        sendResult(syncClient, query.id, {
          entries: result?.entries || [],
          pageURL: tab.url || '',
          count: result?.entries?.length || 0
        })
        debugLog(DebugCategory.CAPTURE, 'Posted waterfall result', { queryId: query.id })
      } catch (err) {
        debugLog(DebugCategory.CAPTURE, 'Waterfall query error', {
          queryId: query.id,
          error: (err as Error).message
        })
        sendResult(syncClient, query.id, {
          error: 'waterfall_query_failed',
          message: (err as Error).message || 'Failed to fetch network waterfall',
          entries: []
        })
      }
      return
    }

    if (query.type === 'dom') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'DOM_QUERY',
          params: query.params
        })
        sendResult(syncClient, query.id, result)
      } catch (err) {
        sendResult(syncClient, query.id, {
          error: 'dom_query_failed',
          message: (err as Error).message || 'Failed to execute DOM query'
        })
      }
      return
    }

    if (query.type === 'a11y') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'A11Y_QUERY',
          params: query.params
        })
        sendResult(syncClient, query.id, result)
      } catch (err) {
        sendResult(syncClient, query.id, {
          error: 'a11y_audit_failed',
          message: (err as Error).message || 'Failed to execute accessibility audit'
        })
      }
      return
    }

    if (query.type === 'dom_action') {
      if (!index.__aiWebPilotEnabledCache) {
        sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', null, 'ai_web_pilot_disabled')
        return
      }
      await executeDOMAction(query, tabId, syncClient, sendAsyncResult, actionToast)
      return
    }

    if (query.type === 'record_start') {
      if (!index.__aiWebPilotEnabledCache) {
        sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', undefined, 'ai_web_pilot_disabled')
        return
      }
      let params: { name?: string; fps?: number; audio?: string }
      try {
        params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
      } catch {
        params = {}
      }
      const result = await startRecording(params.name ?? 'recording', params.fps ?? 15, query.id, params.audio ?? '')
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', result, result.error || undefined)
      return
    }

    if (query.type === 'record_stop') {
      if (!index.__aiWebPilotEnabledCache) {
        sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', undefined, 'ai_web_pilot_disabled')
        return
      }
      const result = await stopRecording()
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', result, result.error || undefined)
      return
    }

    if (query.type === 'execute') {
      if (!index.__aiWebPilotEnabledCache) {
        if (query.correlation_id) {
          sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', null, 'ai_web_pilot_disabled')
        } else {
          sendResult(syncClient, query.id, {
            success: false,
            error: 'ai_web_pilot_disabled',
            message: 'AI Web Pilot is not enabled in the extension popup'
          })
        }
        return
      }

      // Parse world param for routing
      let execParams: { script?: string; timeout_ms?: number; world?: string }
      try {
        execParams = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
      } catch {
        execParams = {}
      }
      const world = execParams.world || 'auto'

      if (query.correlation_id) {
        await handleAsyncExecuteCommand(query, tabId, world, syncClient, sendAsyncResult, actionToast)
      } else {
        try {
          const result = await executeWithWorldRouting(tabId, query.params, world)
          sendResult(syncClient, query.id, result)
        } catch (err) {
          sendResult(syncClient, query.id, {
            success: false,
            error: 'execution_failed',
            message: (err as Error).message || 'Execution failed'
          })
        }
      }
      return
    }

    if (query.type === 'link_health') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'LINK_HEALTH_QUERY',
          params: query.params
        })
        sendResult(syncClient, query.id, result)
      } catch (err) {
        sendResult(syncClient, query.id, {
          error: 'link_health_failed',
          message: (err as Error).message || 'Link health check failed'
        })
      }
      return
    }
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Error handling pending query', {
      type: query.type,
      id: query.id,
      error: (err as Error).message
    })
  }
}

// =============================================================================
// STATE QUERY HANDLING
// =============================================================================

async function handleStateQuery(query: PendingQuery, syncClient: SyncClient): Promise<void> {
  if (!index.__aiWebPilotEnabledCache) {
    sendResult(syncClient, query.id, { error: 'ai_web_pilot_disabled' })
    return
  }

  let params: Record<string, unknown>
  try {
    params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
  } catch {
    sendResult(syncClient, query.id, {
      error: 'invalid_params',
      message: 'Failed to parse state query params as JSON'
    })
    return
  }
  const action = params.action as string

  try {
    let result: unknown

    switch (action) {
      case 'capture': {
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
        const firstTab = tabs[0]
        if (!firstTab?.id) {
          sendResult(syncClient, query.id, { error: 'no_active_tab' })
          return
        }
        result = await chrome.tabs.sendMessage(firstTab.id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: { action: 'capture' }
        })
        break
      }

      case 'save': {
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
        const firstTab = tabs[0]
        if (!firstTab?.id) {
          sendResult(syncClient, query.id, { error: 'no_active_tab' })
          return
        }
        const captureResult = (await chrome.tabs.sendMessage(firstTab.id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: { action: 'capture' }
        })) as { error?: string } & {
          url: string
          timestamp: number
          localStorage: Record<string, string>
          sessionStorage: Record<string, string>
          cookies: string
        }
        if (captureResult.error) {
          sendResult(syncClient, query.id, { error: captureResult.error })
          return
        }
        result = await saveStateSnapshot(params.name as string, captureResult)
        break
      }

      case 'load': {
        const snapshot = await loadStateSnapshot(params.name as string)
        if (!snapshot) {
          sendResult(syncClient, query.id, {
            error: `Snapshot '${params.name}' not found`
          })
          return
        }
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
        const firstTab = tabs[0]
        if (!firstTab?.id) {
          sendResult(syncClient, query.id, { error: 'no_active_tab' })
          return
        }
        result = await chrome.tabs.sendMessage(firstTab.id, {
          type: 'GASOLINE_MANAGE_STATE',
          params: {
            action: 'restore',
            state: snapshot,
            include_url: params.include_url !== false
          }
        })
        break
      }

      case 'list':
        result = { snapshots: await listStateSnapshots() }
        break

      case 'delete':
        result = await deleteStateSnapshot(params.name as string)
        break

      default:
        result = { error: `Unknown action: ${action}` }
    }

    sendResult(syncClient, query.id, result)
  } catch (err) {
    sendResult(syncClient, query.id, { error: (err as Error).message })
  }
}

// =============================================================================
// PILOT COMMAND
// =============================================================================

export async function handlePilotCommand(command: string, params: unknown): Promise<unknown> {
  if (!index.__aiWebPilotEnabledCache) {
    if (typeof chrome !== 'undefined' && chrome.storage) {
      const localResult = await new Promise<{ aiWebPilotEnabled?: boolean }>((resolve) => {
        chrome.storage.local.get(['aiWebPilotEnabled'], (result: { aiWebPilotEnabled?: boolean }) => {
          resolve(result)
        })
      })
      if (localResult.aiWebPilotEnabled === true) {
        // Update cache (note: this module imports from index.ts which has the state)
        // We can't directly update it, so we return the error
      }
    }
  }

  if (!index.__aiWebPilotEnabledCache) {
    return { error: 'ai_web_pilot_disabled' }
  }

  try {
    const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
    const firstTab = tabs[0]

    if (!firstTab?.id) {
      return { error: 'no_active_tab' }
    }

    const tabId = firstTab.id

    const result = await chrome.tabs.sendMessage(tabId, {
      type: command,
      params
    })

    return result || { success: true }
  } catch (err) {
    return { error: (err as Error).message || 'command_failed' }
  }
}
