// interact.ts — Command handlers for the interact MCP tool.
// Handles: subtitle, highlight, browser_action, dom_action, upload,
//          execute, record_start, record_stop, state_*.

import type { PendingQuery } from '../../types'
import type { SyncClient } from '../sync-client'
import { isAiWebPilotEnabled } from '../state'
import { executeDOMAction } from '../dom-dispatch'
import { executeUpload } from '../upload-handler'
import { startRecording, stopRecording } from '../recording'
import { executeWithWorldRouting } from '../query-execution'
import { handleBrowserAction, handleAsyncBrowserAction, handleAsyncExecuteCommand } from '../browser-actions'
import { saveStateSnapshot, loadStateSnapshot, listStateSnapshots, deleteStateSnapshot } from '../message-handlers'
import { registerCommand } from './registry'
import { sendResult, sendAsyncResult } from './helpers'

function statusFromError(error?: string): 'complete' | 'error' {
  return error ? 'error' : 'complete'
}

// =============================================================================
// SUBTITLE
// =============================================================================

registerCommand('subtitle', async (ctx) => {
  let params: { text?: string }
  try {
    params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params
  } catch {
    params = {}
  }
  chrome.tabs
    .sendMessage(ctx.tabId, {
      type: 'GASOLINE_SUBTITLE',
      text: params.text ?? ''
    })
    .catch(() => {})
  ctx.sendResult({ success: true, subtitle: params.text || 'cleared' })
})

// =============================================================================
// HIGHLIGHT
// =============================================================================

registerCommand('highlight', async (ctx) => {
  let params: unknown
  try {
    params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params
  } catch {
    ctx.sendResult({
      error: 'invalid_params',
      message: 'Failed to parse highlight params as JSON'
    })
    return
  }
  const result = await handlePilotCommand('GASOLINE_HIGHLIGHT', params)
  if (ctx.query.correlation_id) {
    const err =
      result && typeof result === 'object' && 'error' in result ? (result as { error: string }).error : undefined
    ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, statusFromError(err), result, err)
  } else {
    ctx.sendResult(result)
  }
})

// =============================================================================
// BROWSER ACTION
// =============================================================================

registerCommand('browser_action', async (ctx) => {
  let params: {
    action?: string
    what?: string
    url?: string
    reason?: string
    tab_id?: number
    tab_index?: number
    new_tab?: boolean
  }
  try {
    params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params
  } catch {
    ctx.sendResult({
      success: false,
      error: 'invalid_params',
      message: 'Failed to parse browser_action params as JSON'
    })
    return
  }
  if (ctx.query.correlation_id) {
    await handleAsyncBrowserAction(ctx.query, ctx.tabId, params, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast)
  } else {
    const result = await handleBrowserAction(ctx.tabId, params, ctx.actionToast)
    ctx.sendResult(result)
  }
})

// =============================================================================
// DOM ACTION
// =============================================================================

registerCommand('dom_action', async (ctx) => {
  if (!isAiWebPilotEnabled()) {
    ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id!, 'error', null, 'ai_web_pilot_disabled')
    return
  }
  await executeDOMAction(ctx.query, ctx.tabId, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast)
})

// =============================================================================
// UPLOAD
// =============================================================================

registerCommand('upload', async (ctx) => {
  if (!isAiWebPilotEnabled()) {
    ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id!, 'error', null, 'ai_web_pilot_disabled')
    return
  }
  await executeUpload(ctx.query, ctx.tabId, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast)
})

// =============================================================================
// RECORD START
// =============================================================================

registerCommand('record_start', async (ctx) => {
  if (!isAiWebPilotEnabled()) {
    ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id!, 'error', undefined, 'ai_web_pilot_disabled')
    return
  }
  let params: { name?: string; fps?: number; audio?: string }
  try {
    params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params
  } catch {
    params = {}
  }
  const result = await startRecording(
    params.name ?? 'recording',
    params.fps ?? 15,
    ctx.query.id,
    params.audio ?? '',
    false,
    ctx.tabId
  )
  const error = result.error || undefined
  ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id!, statusFromError(error), result, error)
})

// =============================================================================
// RECORD STOP
// =============================================================================

registerCommand('record_stop', async (ctx) => {
  if (!isAiWebPilotEnabled()) {
    sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id!, 'error', undefined, 'ai_web_pilot_disabled')
    return
  }
  const result = await stopRecording()
  const error = result.error || undefined
  sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id!, statusFromError(error), result, error)
})

// =============================================================================
// STATE QUERIES (state_capture, state_save, state_load, state_list, state_delete)
// =============================================================================

registerCommand('state_*', async (ctx) => {
  if (!isAiWebPilotEnabled()) {
    sendResult(ctx.syncClient, ctx.query.id, { error: 'ai_web_pilot_disabled' })
    return
  }

  let params: Record<string, unknown>
  try {
    params = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params
  } catch {
    sendResult(ctx.syncClient, ctx.query.id, {
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
          sendResult(ctx.syncClient, ctx.query.id, { error: 'no_active_tab' })
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
          sendResult(ctx.syncClient, ctx.query.id, { error: 'no_active_tab' })
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
          sendResult(ctx.syncClient, ctx.query.id, { error: captureResult.error })
          return
        }
        result = await saveStateSnapshot(params.name as string, captureResult)
        break
      }

      case 'load': {
        const snapshot = await loadStateSnapshot(params.name as string)
        if (!snapshot) {
          sendResult(ctx.syncClient, ctx.query.id, {
            error: `Snapshot '${params.name}' not found`
          })
          return
        }
        const tabs = await chrome.tabs.query({ active: true, currentWindow: true })
        const firstTab = tabs[0]
        if (!firstTab?.id) {
          sendResult(ctx.syncClient, ctx.query.id, { error: 'no_active_tab' })
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

    sendResult(ctx.syncClient, ctx.query.id, result)
  } catch (err) {
    sendResult(ctx.syncClient, ctx.query.id, { error: (err as Error).message })
  }
})

// =============================================================================
// EXECUTE
// =============================================================================

registerCommand('execute', async (ctx) => {
  if (!isAiWebPilotEnabled()) {
    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, 'ai_web_pilot_disabled')
    } else {
      ctx.sendResult({
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
    execParams = typeof ctx.query.params === 'string' ? JSON.parse(ctx.query.params) : ctx.query.params
  } catch {
    execParams = {}
  }
  const world = execParams.world || 'auto'

  if (ctx.query.correlation_id) {
    await handleAsyncExecuteCommand(ctx.query, ctx.tabId, world, ctx.syncClient, ctx.sendAsyncResult, ctx.actionToast)
  } else {
    try {
      const result = await executeWithWorldRouting(ctx.tabId, ctx.query.params, world)
      ctx.sendResult(result)
    } catch (err) {
      ctx.sendResult({
        success: false,
        error: 'execution_failed',
        message: (err as Error).message || 'Execution failed'
      })
    }
  }
})

// =============================================================================
// PILOT COMMAND (exported for use by index.ts re-export chain)
// =============================================================================

export async function handlePilotCommand(command: string, params: unknown): Promise<unknown> {
  if (!isAiWebPilotEnabled()) {
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
