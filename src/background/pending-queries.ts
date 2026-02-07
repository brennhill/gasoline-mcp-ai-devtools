/**
 * @fileoverview Pending Query Handlers
 * Handles all query types from the server: DOM, accessibility, browser actions,
 * execute commands, and state management.
 *
 * All results are returned via syncClient.queueCommandResult() which routes them
 * through the unified /sync endpoint. No direct HTTP POSTs to legacy endpoints.
 */

import type { PendingQuery } from '../types'
import type { SyncClient } from './sync-client'
import * as eventListeners from './event-listeners'
import * as index from './index'
import { DebugCategory } from './debug'
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot, broadcastTrackingState } from './message-handlers'
import { executeDOMAction } from './dom-primitives'

// Extract values from index for easier reference (but NOT DebugCategory - imported directly above)
const { debugLog, diagnosticLog } = index

// =============================================================================
// TIMEOUT CONFIGURATION
// =============================================================================

/**
 * Timeout for async execute commands (JavaScript execution in page context)
 * Needs to accommodate:
 * - Axe accessibility audits on large pages (20-30s)
 * - Complex DOM queries
 * - Screenshot capture and encoding
 * - Custom JavaScript execution
 */
const ASYNC_EXECUTE_TIMEOUT_MS = 60000 // 60 seconds

/**
 * Timeout for async browser actions (navigation, refresh, etc.)
 * Needs to accommodate:
 * - Page navigation on slow networks
 * - Page load and rendering
 * - Resource fetching
 */
const ASYNC_BROWSER_ACTION_TIMEOUT_MS = 60000 // 60 seconds

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
  error?: string,
): void {
  debugLog(DebugCategory.CONNECTION, 'sendAsyncResult via /sync', { queryId, correlationId, status, hasResult: result != null, error: error || null })
  syncClient.queueCommandResult({
    id: queryId,
    correlation_id: correlationId,
    status,
    result,
    error,
  })
}

/** Show a visual action toast on the tracked tab */
function actionToast(
  tabId: number,
  text: string,
  detail?: string,
  state: 'trying' | 'success' | 'warning' | 'error' = 'success',
  durationMs = 3000,
): void {
  chrome.tabs.sendMessage(tabId, {
    type: 'GASOLINE_ACTION_TOAST',
    text,
    detail,
    state,
    duration_ms: durationMs,
  }).catch(() => {})
}

// =============================================================================
// ISOLATED WORLD EXECUTION (chrome.scripting API)
// =============================================================================

/**
 * Execute JavaScript via chrome.scripting.executeScript.
 * Used as fallback when MAIN world execution fails due to page CSP,
 * or when inject script is not loaded.
 * The func is injected natively by Chrome's extension system.
 */
async function executeViaScriptingAPI(
  tabId: number,
  script: string,
  timeoutMs: number,
): Promise<{ success: boolean; error?: string; message?: string; result?: unknown; stack?: string }> {
  const timeoutPromise = new Promise<never>((_, reject) => {
    setTimeout(() => reject(new Error(`Script exceeded ${timeoutMs}ms timeout`)), timeoutMs + 2000)
  })

  const executionPromise = chrome.scripting.executeScript({
    target: { tabId },
    world: 'MAIN',
    func: (code: string) => {
      try {
        const cleaned = code.trim()
        const hasMultiple = cleaned.includes(';')
        const hasReturn = /\breturn\b/.test(cleaned)
        const body = hasMultiple || hasReturn
          ? `"use strict"; ${cleaned}`
          : `"use strict"; return (${cleaned});`

        // eslint-disable-next-line no-new-func
        const fn = new Function(body) as () => unknown
        const result = fn()

        if (result !== null && result !== undefined && typeof (result as { then?: unknown }).then === 'function') {
          return (result as Promise<unknown>).then((v: unknown) => {
            return { success: true as const, result: serialize(v) }
          }).catch((err: unknown) => {
            const e = err as Error
            return { success: false as const, error: 'promise_rejected', message: e.message }
          })
        }

        return { success: true as const, result: serialize(result) }
      } catch (err) {
        const e = err as Error
        const msg = e.message || ''
        if (msg.includes('Content Security Policy') || msg.includes('Trusted Type') || msg.includes('unsafe-eval')) {
          return {
            success: false as const,
            error: 'csp_blocked_all_worlds',
            message: 'Page CSP blocks dynamic script execution. ' +
              'Use query_dom for DOM operations or navigate away from this CSP-restricted page.',
          }
        }
        return { success: false as const, error: 'execution_error', message: msg, stack: e.stack }
      }

      function serialize(value: unknown, depth = 0, seen = new WeakSet<object>()): unknown {
        if (depth > 10) return '[max depth]'
        if (value === null || value === undefined) return value
        const t = typeof value
        if (t === 'string' || t === 'number' || t === 'boolean') return value
        if (t === 'function') return '[Function]'
        if (t === 'symbol') return String(value)
        if (t === 'object') {
          const obj = value as object
          if (seen.has(obj)) return '[Circular]'
          seen.add(obj)
          if (Array.isArray(obj)) return obj.slice(0, 100).map(v => serialize(v, depth + 1, seen))
          if (obj instanceof Error) return { error: (obj as Error).message }
          if (obj instanceof Date) return (obj as Date).toISOString()
          if (obj instanceof RegExp) return String(obj)
          // DOM node duck-type check (works across worlds)
          if ('nodeType' in obj && 'nodeName' in obj) {
            const node = obj as { nodeName: string; id?: string }
            return `[${node.nodeName}${node.id ? '#' + node.id : ''}]`
          }
          const result: Record<string, unknown> = {}
          for (const key of Object.keys(obj).slice(0, 50)) {
            try { result[key] = serialize((obj as Record<string, unknown>)[key], depth + 1, seen) } catch { result[key] = '[unserializable]' }
          }
          return result
        }
        return String(value)
      }
    },
    args: [script],
  })

  try {
    const results = await Promise.race([executionPromise, timeoutPromise])
    const firstResult = results?.[0]?.result
    if (firstResult && typeof firstResult === 'object') {
      return firstResult as { success: boolean; error?: string; message?: string; result?: unknown; stack?: string }
    }
    return { success: false, error: 'no_result', message: 'chrome.scripting.executeScript produced no result' }
  } catch (err) {
    const msg = (err as Error).message || ''
    if (msg.includes('timeout')) {
      return { success: false, error: 'execution_timeout', message: msg }
    }
    return { success: false, error: 'scripting_api_error', message: msg }
  }
}

/**
 * Execute JS with world-aware routing.
 * - isolated: execute directly via chrome.scripting API
 * - main: send to content script (MAIN world via inject)
 * - auto: try content script, fallback to scripting API on CSP/inject errors
 */
async function executeWithWorldRouting(
  tabId: number,
  queryParams: string | Record<string, unknown>,
  world: string,
): Promise<{ success: boolean; error?: string; message?: string; result?: unknown; stack?: string }> {
  let parsedParams: { script?: string; timeout_ms?: number }
  try {
    parsedParams = typeof queryParams === 'string' ? JSON.parse(queryParams) : queryParams
  } catch {
    parsedParams = {}
  }
  const script = parsedParams.script || ''
  const timeoutMs = parsedParams.timeout_ms || 5000

  if (world === 'isolated') {
    return executeViaScriptingAPI(tabId, script, timeoutMs)
  }

  // MAIN or AUTO: try content script (MAIN world) first
  try {
    const result = await chrome.tabs.sendMessage(tabId, {
      type: 'GASOLINE_EXECUTE_QUERY',
      params: queryParams,
    }) as { success: boolean; error?: string; message?: string; result?: unknown; stack?: string }

    // Auto-fallback: retry via scripting API on CSP or inject issues
    if (world === 'auto' && result && !result.success &&
        (result.error === 'csp_blocked' || result.error === 'inject_not_loaded')) {
      debugLog(DebugCategory.CONNECTION, 'Auto-fallback to chrome.scripting API', {
        error: result.error, tabId,
      })
      return executeViaScriptingAPI(tabId, script, timeoutMs)
    }

    return result
  } catch (err) {
    let message = (err as Error).message || 'Tab communication failed'

    // Auto-fallback: content script not reachable
    if (world === 'auto' && message.includes('Receiving end does not exist')) {
      debugLog(DebugCategory.CONNECTION, 'Auto-fallback (content script unreachable)', { tabId })
      return executeViaScriptingAPI(tabId, script, timeoutMs)
    }

    if (message.includes('Receiving end does not exist')) {
      message =
        'Content script not loaded. REQUIRED ACTION: Refresh the page first using this command:\n\ninteract({action: "refresh"})\n\nThen retry your command.'
    }
    return { success: false, error: 'content_script_not_loaded', message }
  }
}

// =============================================================================
// PENDING QUERY HANDLING
// =============================================================================

export async function handlePendingQuery(query: PendingQuery, syncClient: SyncClient): Promise<void> {
  debugLog(DebugCategory.CONNECTION, 'handlePendingQuery ENTER', {
    id: query.id,
    type: query.type,
    correlation_id: query.correlation_id || null,
    hasSyncClient: !!syncClient,
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
        diagnosticLog(`[Diagnostic] Tracked tab ${storage.trackedTabId} no longer exists, clearing tracking`)
        eventListeners.clearTrackedTab()
        const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true })
        const firstActiveTab = activeTabs[0]
        if (!firstActiveTab?.id) return
        tabId = firstActiveTab.id
      }
    } else {
      const activeTabs = await chrome.tabs.query({ active: true, currentWindow: true })
      const firstActiveTab = activeTabs[0]
      if (!firstActiveTab?.id) return
      tabId = firstActiveTab.id
    }

    if (!tabId) return

    if (query.type === 'browser_action') {
      let params: { action?: string; url?: string; reason?: string }
      try {
        params = typeof query.params === 'string' ? JSON.parse(query.params) : query.params
      } catch {
        sendResult(syncClient, query.id, {
          success: false,
          error: 'invalid_params',
          message: 'Failed to parse browser_action params as JSON',
        })
        return
      }
      if (query.correlation_id) {
        await handleAsyncBrowserAction(query, tabId, params, syncClient)
      } else {
        const result = await handleBrowserAction(tabId, params)
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
          message: 'Failed to parse highlight params as JSON',
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
          height: tab.height,
        },
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
        index: tab.index,
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
          type: 'GET_NETWORK_WATERFALL',
        })) as { entries?: unknown[] }
        debugLog(DebugCategory.CAPTURE, 'Waterfall result from content script', {
          entries: result?.entries?.length || 0
        })

        sendResult(syncClient, query.id, {
          entries: result?.entries || [],
          pageURL: tab.url || '',
          count: result?.entries?.length || 0,
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
          entries: [],
        })
      }
      return
    }

    if (query.type === 'dom') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'DOM_QUERY',
          params: query.params,
        })
        sendResult(syncClient, query.id, result)
      } catch (err) {
        sendResult(syncClient, query.id, {
          error: 'dom_query_failed',
          message: (err as Error).message || 'Failed to execute DOM query',
        })
      }
      return
    }

    if (query.type === 'a11y') {
      try {
        const result = await chrome.tabs.sendMessage(tabId, {
          type: 'A11Y_QUERY',
          params: query.params,
        })
        sendResult(syncClient, query.id, result)
      } catch (err) {
        sendResult(syncClient, query.id, {
          error: 'a11y_audit_failed',
          message: (err as Error).message || 'Failed to execute accessibility audit',
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

    if (query.type === 'execute') {
      if (!index.__aiWebPilotEnabledCache) {
        if (query.correlation_id) {
          sendAsyncResult(syncClient, query.id, query.correlation_id, 'complete', null, 'ai_web_pilot_disabled')
        } else {
          sendResult(syncClient, query.id, {
            success: false,
            error: 'ai_web_pilot_disabled',
            message: 'AI Web Pilot is not enabled in the extension popup',
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
        await handleAsyncExecuteCommand(query, tabId, world, syncClient)
      } else {
        try {
          const result = await executeWithWorldRouting(tabId, query.params, world)
          sendResult(syncClient, query.id, result)
        } catch (err) {
          sendResult(syncClient, query.id, {
            success: false,
            error: 'execution_failed',
            message: (err as Error).message || 'Execution failed',
          })
        }
      }
      return
    }
  } catch (err) {
    debugLog(DebugCategory.CONNECTION, 'Error handling pending query', {
      type: query.type,
      id: query.id,
      error: (err as Error).message,
    })
  }
}

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
      message: 'Failed to parse state query params as JSON',
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
          params: { action: 'capture' },
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
          params: { action: 'capture' },
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
            error: `Snapshot '${params.name}' not found`,
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
            include_url: params.include_url !== false,
          },
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

async function handleBrowserAction(
  tabId: number,
  params: { action?: string; url?: string; reason?: string },
): Promise<{
  success: boolean
  action?: string
  url?: string
  content_script_status?: string
  message?: string
  error?: string
}> {
  const { action, url, reason } = params || {}

  if (!index.__aiWebPilotEnabledCache) {
    return { success: false, error: 'ai_web_pilot_disabled', message: 'AI Web Pilot is not enabled' }
  }

  try {
    switch (action) {
      case 'refresh':
        actionToast(tabId, 'refresh', reason || 'reloading page', 'trying', 10000)
        await chrome.tabs.reload(tabId)
        await eventListeners.waitForTabLoad(tabId)
        actionToast(tabId, 'refresh', reason || 'page reloaded', 'success')
        return { success: true, action: 'refresh' }

      case 'navigate': {
        if (!url) {
          return { success: false, error: 'missing_url', message: 'URL required for navigate action' }
        }

        if (url.startsWith('chrome://') || url.startsWith('chrome-extension://')) {
          return {
            success: false,
            error: 'restricted_url',
            message: 'Cannot navigate to Chrome internal pages',
          }
        }

        actionToast(tabId, 'navigate', reason || url, 'trying', 10000)
        await chrome.tabs.update(tabId, { url })
        await eventListeners.waitForTabLoad(tabId)
        await new Promise((r) => setTimeout(r, 500))

        const contentScriptLoaded = await eventListeners.pingContentScript(tabId)

        if (contentScriptLoaded) {
          broadcastTrackingState().catch(() => {})
          actionToast(tabId, 'navigate', reason || url, 'success')
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'loaded',
            message: 'Content script ready',
          }
        }

        const tab = await chrome.tabs.get(tabId)
        if (tab.url?.startsWith('file://')) {
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'unavailable',
            message:
              'Content script cannot load on file:// URLs. Enable "Allow access to file URLs" in extension settings.',
          }
        }

        debugLog(DebugCategory.CAPTURE, 'Content script not loaded after navigate, refreshing', { tabId, url })
        await chrome.tabs.reload(tabId)
        await eventListeners.waitForTabLoad(tabId)
        await new Promise((r) => setTimeout(r, 1000))

        const loadedAfterRefresh = await eventListeners.pingContentScript(tabId)

        if (loadedAfterRefresh) {
          broadcastTrackingState().catch(() => {})
          return {
            success: true,
            action: 'navigate',
            url,
            content_script_status: 'refreshed',
            message: 'Page refreshed to load content script',
          }
        }

        return {
          success: true,
          action: 'navigate',
          url,
          content_script_status: 'failed',
          message: 'Navigation complete but content script could not be loaded. AI Web Pilot tools may not work.',
        }
      }

      case 'back':
        await chrome.tabs.goBack(tabId)
        return { success: true, action: 'back' }

      case 'forward':
        await chrome.tabs.goForward(tabId)
        return { success: true, action: 'forward' }

      default:
        return { success: false, error: 'unknown_action', message: `Unknown action: ${action}` }
    }
  } catch (err) {
    return { success: false, error: 'browser_action_failed', message: (err as Error).message }
  }
}

async function handleAsyncExecuteCommand(query: PendingQuery, tabId: number, world: string, syncClient: SyncClient): Promise<void> {
  const startTime = Date.now()

  try {
    const result = await Promise.race([
      executeWithWorldRouting(tabId, query.params, world),
      new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('Execution timeout')), ASYNC_EXECUTE_TIMEOUT_MS)
      }),
    ])

    if (result.success) {
      actionToast(tabId, 'execute_js', 'script completed', 'success')
    }

    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', result)

    debugLog(DebugCategory.CONNECTION, 'Completed async command', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
      success: result.success,
    })
  } catch {
    const timeoutMessage = `JavaScript execution exceeded timeout. RECOMMENDED ACTIONS:

1. Break your task into smaller discrete steps that execute in < 2s for best results
2. Check your script for infinite loops or blocking operations
3. Simplify the operation or target a smaller DOM scope`

    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'timeout', null, timeoutMessage)

    debugLog(DebugCategory.CONNECTION, 'Async command timeout', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
    })
  }
}

async function handleAsyncBrowserAction(
  query: PendingQuery,
  tabId: number,
  params: { action?: string; url?: string },
  syncClient: SyncClient,
): Promise<void> {
  const startTime = Date.now()

  const executionPromise = handleBrowserAction(tabId, params)
    .then((result) => {
      return result
    })
    .catch((err: Error) => {
      return {
        success: false as const,
        error: err.message || 'Browser action failed',
      }
    })

  try {
    const execResult = await Promise.race([
      executionPromise,
      new Promise<never>((_, reject) => {
        setTimeout(() => reject(new Error('Execution timeout')), ASYNC_BROWSER_ACTION_TIMEOUT_MS)
      }),
    ])

    if (execResult.success !== false) {
      sendAsyncResult(syncClient, query.id, query.correlation_id!, 'complete', execResult)
    } else {
      sendAsyncResult(
        syncClient,
        query.id,
        query.correlation_id!,
        'complete',
        null,
        execResult.error,
      )
    }

    debugLog(DebugCategory.CONNECTION, 'Completed async browser action', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
      success: execResult.success !== false,
    })
  } catch {
    const timeoutMessage = `Browser action exceeded 10s timeout. DIAGNOSTIC STEPS:

1. Check page status: observe({what: 'page'})
2. Check for console errors: observe({what: 'errors'})
3. Check network requests: observe({what: 'network', status_min: 400})`

    sendAsyncResult(syncClient, query.id, query.correlation_id!, 'timeout', null, timeoutMessage)

    debugLog(DebugCategory.CONNECTION, 'Async browser action timeout', {
      correlationId: query.correlation_id,
      elapsed: Date.now() - startTime,
    })
  }
}

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
      params,
    })

    return result || { success: true }
  } catch (err) {
    return { error: (err as Error).message || 'command_failed' }
  }
}
