/**
 * Purpose: Handles extension background coordination and message routing.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

// browser-actions.ts — Browser navigation and action handlers.
// Handles navigate, refresh, back, forward actions with async timeout support.

import type { PendingQuery } from '../types'
import type { SyncClient } from './sync-client'
import * as eventListeners from './event-listeners'
import * as index from './index'
import { DebugCategory } from './debug'
import { broadcastTrackingState } from './message-handlers'
import { executeWithWorldRouting } from './query-execution'
import { ASYNC_COMMAND_TIMEOUT_MS } from '../lib/constants'
import type { SendAsyncResultFn, ActionToastFn } from './pending-queries'

const { debugLog } = index

// =============================================================================
// TIMEOUT CONFIGURATION
// =============================================================================

const ASYNC_EXECUTE_TIMEOUT_MS = ASYNC_COMMAND_TIMEOUT_MS
const ASYNC_BROWSER_ACTION_TIMEOUT_MS = ASYNC_COMMAND_TIMEOUT_MS

// =============================================================================
// BROWSER ACTION TYPES
// =============================================================================

export type BrowserActionResult = {
  success: boolean
  action?: string
  url?: string
  final_url?: string
  title?: string
  tab_id?: number
  content_script_status?: string
  message?: string
  error?: string
}

// =============================================================================
// NAVIGATION
// =============================================================================

// #lizard forgives
export async function handleNavigateAction(
  tabId: number,
  url: string,
  actionToast: ActionToastFn,
  reason?: string
): Promise<BrowserActionResult> {
  if (url.startsWith('chrome://') || url.startsWith('chrome-extension://')) {
    return { success: false, error: 'restricted_url', message: 'Cannot navigate to Chrome internal pages' }
  }

  actionToast(tabId, reason || 'navigate', reason ? undefined : url, 'trying', 10000)
  await chrome.tabs.update(tabId, { url })
  await eventListeners.waitForTabLoad(tabId)
  await new Promise((r) => setTimeout(r, 500))

  const tab = await chrome.tabs.get(tabId)

  if (await eventListeners.pingContentScript(tabId)) {
    broadcastTrackingState().catch(() => {})
    actionToast(tabId, reason || 'navigate', reason ? undefined : url, 'success')
    return { success: true, action: 'navigate', url, final_url: tab.url, title: tab.title, content_script_status: 'loaded', message: 'Content script ready' }
  }

  if (tab.url?.startsWith('file://')) {
    return {
      success: true,
      action: 'navigate',
      url,
      final_url: tab.url,
      title: tab.title,
      content_script_status: 'unavailable',
      message: 'Content script cannot load on file:// URLs. Enable "Allow access to file URLs" in extension settings.'
    }
  }

  debugLog(DebugCategory.CAPTURE, 'Content script not loaded after navigate, refreshing', { tabId, url })
  await chrome.tabs.reload(tabId)
  await eventListeners.waitForTabLoad(tabId)
  await new Promise((r) => setTimeout(r, 1000))

  const reloadedTab = await chrome.tabs.get(tabId)

  if (await eventListeners.pingContentScript(tabId)) {
    broadcastTrackingState().catch(() => {})
    return {
      success: true,
      action: 'navigate',
      url,
      final_url: reloadedTab.url,
      title: reloadedTab.title,
      content_script_status: 'refreshed',
      message: 'Page refreshed to load content script'
    }
  }

  return {
    success: true,
    action: 'navigate',
    url,
    final_url: reloadedTab.url,
    title: reloadedTab.title,
    content_script_status: 'failed',
    message: 'Navigation complete but content script could not be loaded. AI Web Pilot tools may not work.'
  }
}

// =============================================================================
// BROWSER ACTION DISPATCH
// =============================================================================

export async function handleBrowserAction(
  tabId: number,
  params: { action?: string; url?: string; reason?: string },
  actionToast: ActionToastFn
): Promise<BrowserActionResult> {
  const { action, url, reason } = params || {}

  if (!index.__aiWebPilotEnabledCache) {
    return { success: false, error: 'ai_web_pilot_disabled', message: 'AI Web Pilot is not enabled' }
  }

  try {
    switch (action) {
      case 'refresh': {
        actionToast(tabId, reason || 'refresh', reason ? undefined : 'reloading page', 'trying', 10000)
        await chrome.tabs.reload(tabId)
        await eventListeners.waitForTabLoad(tabId)
        actionToast(tabId, reason || 'refresh', undefined, 'success')
        const refreshedTab = await chrome.tabs.get(tabId)
        return { success: true, action: 'refresh', url: refreshedTab.url, title: refreshedTab.title }
      }
      case 'navigate':
        if (!url) return { success: false, error: 'missing_url', message: 'URL required for navigate action' }
        return handleNavigateAction(tabId, url, actionToast, reason)
      case 'back': {
        actionToast(tabId, reason || 'back', reason ? undefined : 'going back', 'trying', 10000)
        await chrome.tabs.goBack(tabId)
        await eventListeners.waitForTabLoad(tabId)
        actionToast(tabId, reason || 'back', undefined, 'success')
        const backTab = await chrome.tabs.get(tabId)
        return { success: true, action: 'back', url: backTab.url, title: backTab.title }
      }
      case 'forward': {
        actionToast(tabId, reason || 'forward', reason ? undefined : 'going forward', 'trying', 10000)
        await chrome.tabs.goForward(tabId)
        await eventListeners.waitForTabLoad(tabId)
        actionToast(tabId, reason || 'forward', undefined, 'success')
        const fwdTab = await chrome.tabs.get(tabId)
        return { success: true, action: 'forward', url: fwdTab.url, title: fwdTab.title }
      }
      case 'new_tab': {
        if (!url) return { success: false, error: 'missing_url', message: 'URL required for new_tab action' }
        actionToast(tabId, reason || 'new_tab', reason ? undefined : 'opening new tab', 'trying', 5000)
        const newTab = await chrome.tabs.create({ url, active: false })
        actionToast(tabId, reason || 'new_tab', undefined, 'success')
        return { success: true, action: 'new_tab', url, tab_id: newTab.id, title: newTab.title }
      }
      default:
        return { success: false, error: 'unknown_action', message: `Unknown action: ${action}` }
    }
  } catch (err) {
    return { success: false, error: 'browser_action_failed', message: (err as Error).message }
  }
}

// =============================================================================
// ASYNC EXECUTE COMMAND
// =============================================================================

export async function handleAsyncExecuteCommand(
  query: PendingQuery,
  tabId: number,
  world: string,
  syncClient: SyncClient,
  sendAsyncResult: SendAsyncResultFn,
  actionToast: ActionToastFn
): Promise<void> {
  const startTime = Date.now()

  // Extract reason for toast display
  let reason: string | undefined
  try {
    const p = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
    reason = (p as { reason?: string })?.reason
  } catch {
    /* ignore parse errors */
  }

  try {
    const result = await Promise.race([
      executeWithWorldRouting(tabId, query.params, world),
      new Promise<never>((_, reject) => {
        setTimeout(
          () =>
            reject(
              new Error(
                `Script execution timed out after ${ASYNC_EXECUTE_TIMEOUT_MS}ms. Script may be stuck in a loop or waiting for user input.`
              )
            ),
          ASYNC_EXECUTE_TIMEOUT_MS
        )
      })
    ])

    if (result.success) {
      actionToast(tabId, reason || 'execute_js', undefined, 'success')
    }

    let enrichedResult: unknown = result
    try {
      const tab = await chrome.tabs.get(tabId)
      enrichedResult = { ...result, effective_tab_id: tabId, effective_url: tab.url, effective_title: tab.title }
    } catch {
      /* tab may have closed */
    }

    const status = result.success ? 'complete' : 'error'
    const error = result.success ? undefined : result.error || result.message || 'execution_failed'
    sendAsyncResult(syncClient, query.id, query.correlation_id!, status, enrichedResult, error)

    debugLog(DebugCategory.CONNECTION, 'Completed async command', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
      success: result.success
    })
  } catch {
    const timeoutMessage = `JavaScript execution exceeded ${ASYNC_EXECUTE_TIMEOUT_MS / 1000}s timeout. RECOMMENDED ACTIONS:

1. Break your task into smaller discrete steps that execute in < ${ASYNC_EXECUTE_TIMEOUT_MS / 1000}s
2. Check your script for infinite loops or blocking operations
3. Simplify the operation or target a smaller DOM scope`

    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'timeout', null, timeoutMessage)

    debugLog(DebugCategory.CONNECTION, 'Async command timeout', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime
    })
  }
}

// =============================================================================
// ASYNC BROWSER ACTION
// =============================================================================

export async function handleAsyncBrowserAction(
  query: PendingQuery,
  tabId: number,
  params: { action?: string; url?: string },
  syncClient: SyncClient,
  sendAsyncResult: SendAsyncResultFn,
  actionToast: ActionToastFn
): Promise<void> {
  const startTime = Date.now()

  const executionPromise = handleBrowserAction(tabId, params, actionToast)
    .then((result) => {
      return result
    })
    .catch((err: Error) => {
      return {
        success: false as const,
        error: err.message || 'Browser action failed'
      }
    })

  try {
    const execResult = await Promise.race([
      executionPromise,
      new Promise<never>((_, reject) => {
        setTimeout(
          () =>
            reject(
              new Error(
                `Browser action execution timed out after ${ASYNC_BROWSER_ACTION_TIMEOUT_MS}ms. Action may be waiting for user interaction or network response.`
              )
            ),
          ASYNC_BROWSER_ACTION_TIMEOUT_MS
        )
      })
    ])

    if (execResult.success !== false) {
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', execResult)
    } else {
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'error', null, execResult.error)
    }

    debugLog(DebugCategory.CONNECTION, 'Completed async browser action', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
      success: execResult.success !== false
    })
  } catch {
    // nosemgrep: missing-template-string-indicator
    const timeoutMessage = `Browser action exceeded ${ASYNC_BROWSER_ACTION_TIMEOUT_MS / 1000}s timeout. DIAGNOSTIC STEPS:

1. Check page status: observe({what: 'page'})
2. Check for console errors: observe({what: 'errors'})
3. Check network requests: observe({what: 'network_waterfall', status_min: 400})`

    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'timeout', null, timeoutMessage)

    debugLog(DebugCategory.CONNECTION, 'Async browser action timeout', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime
    })
  }
}
