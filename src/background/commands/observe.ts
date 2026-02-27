/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

// observe.ts — Command handlers for the observe MCP tool.
// Handles: screenshot, waterfall, page_info, tabs.

import { debugLog } from '../index'
import { getServerUrl } from '../state'
import { DebugCategory } from '../debug'
import { recordScreenshot } from '../state-manager'
import { domPrimitiveListInteractive } from '../dom-primitives-list-interactive'
import { registerCommand } from './registry'

// =============================================================================
// SCREENSHOT
// =============================================================================

registerCommand('screenshot', async (ctx) => {
  try {
    const tab = await chrome.tabs.get(ctx.tabId)
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
      format: 'jpeg',
      quality: 80
    })
    recordScreenshot(ctx.tabId)

    // POST to /screenshots with query_id — server saves file and resolves query directly
    const response = await fetch(`${getServerUrl()}/screenshots`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Gasoline-Client': 'gasoline-extension' },
      body: JSON.stringify({
        data_url: dataUrl,
        url: tab.url,
        query_id: ctx.query.id
      })
    })
    if (!response.ok) {
      ctx.sendResult({ error: `Server returned ${response.status}` })
    }
    // No sendResult needed — server resolves the query via query_id
  } catch (err) {
    ctx.sendResult({
      error: 'screenshot_failed',
      message: (err as Error).message || 'Failed to capture screenshot'
    })
  }
})

// =============================================================================
// WATERFALL
// =============================================================================

registerCommand('waterfall', async (ctx) => {
  debugLog(DebugCategory.CAPTURE, 'Handling waterfall query', { queryId: ctx.query.id, tabId: ctx.tabId })
  try {
    const tab = await chrome.tabs.get(ctx.tabId)
    debugLog(DebugCategory.CAPTURE, 'Got tab for waterfall', { tabId: ctx.tabId, url: tab.url })
    const result = (await chrome.tabs.sendMessage(ctx.tabId, {
      type: 'GET_NETWORK_WATERFALL'
    })) as { entries?: unknown[] }
    debugLog(DebugCategory.CAPTURE, 'Waterfall result from content script', {
      entries: result?.entries?.length || 0
    })

    ctx.sendResult({
      entries: result?.entries || [],
      page_url: tab.url || '',
      count: result?.entries?.length || 0
    })
    debugLog(DebugCategory.CAPTURE, 'Posted waterfall result', { queryId: ctx.query.id })
  } catch (err) {
    debugLog(DebugCategory.CAPTURE, 'Waterfall query error', {
      queryId: ctx.query.id,
      error: (err as Error).message
    })
    ctx.sendResult({
      error: 'waterfall_query_failed',
      message: (err as Error).message || 'Failed to fetch network waterfall',
      entries: []
    })
  }
})

// =============================================================================
// PAGE INFO
// =============================================================================

registerCommand('page_info', async (ctx) => {
  try {
    const tab = await chrome.tabs.get(ctx.tabId)
    ctx.sendResult({
      url: tab.url,
      title: tab.title,
      favicon: tab.favIconUrl,
      status: tab.status,
      viewport: {
        width: tab.width,
        height: tab.height
      }
    })
  } catch (err) {
    ctx.sendResult({
      error: 'page_info_failed',
      message: (err as Error).message || `Failed to get tab ${ctx.tabId}`
    })
  }
})

// =============================================================================
// TABS
// =============================================================================

registerCommand('tabs', async (ctx) => {
  try {
    const allTabs = await chrome.tabs.query({})
    const tabsList = allTabs.map((tab) => ({
      id: tab.id,
      url: tab.url,
      title: tab.title,
      active: tab.active,
      windowId: tab.windowId,
      index: tab.index
    }))
    ctx.sendResult({ tabs: tabsList })
  } catch (err) {
    ctx.sendResult({
      error: 'tabs_query_failed',
      message: (err as Error).message || 'Failed to query tabs'
    })
  }
})

// =============================================================================
// PAGE INVENTORY (#318)
// =============================================================================

registerCommand('page_inventory', async (ctx) => {
  try {
    // 1. Get tab info (page metadata)
    const tab = await chrome.tabs.get(ctx.tabId)

    // 2. Run list_interactive via chrome.scripting in the page
    const scopeSelector = typeof ctx.params.scope_selector === 'string' ? ctx.params.scope_selector : ''
    const interactiveResults = await chrome.scripting.executeScript({
      target: { tabId: ctx.tabId, allFrames: true },
      world: 'MAIN',
      func: domPrimitiveListInteractive,
      args: [scopeSelector]
    })

    // Merge interactive elements from all frames (up to 100)
    const elements: unknown[] = []
    let firstError: string | undefined
    for (const r of interactiveResults) {
      const res = r.result as {
        success?: boolean
        elements?: unknown[]
        error?: string
        message?: string
      } | null
      if (res?.success === false) {
        if (!firstError) firstError = res.error || res.message
        continue
      }
      if (res?.elements) {
        elements.push(...res.elements)
        if (elements.length >= 100) break
      }
    }
    const cappedElements = elements.slice(0, 100)

    // Apply visible_only filter if requested
    let filteredElements = cappedElements
    if (ctx.params.visible_only === true) {
      filteredElements = cappedElements.filter((el) => {
        const elem = el as { visible?: boolean }
        return elem.visible !== false
      })
    }

    // Apply limit if specified
    const limit = typeof ctx.params.limit === 'number' && ctx.params.limit > 0
      ? ctx.params.limit
      : filteredElements.length
    const finalElements = filteredElements.slice(0, limit)

    const payload: Record<string, unknown> = {
      url: tab.url || '',
      title: tab.title || '',
      tab_status: tab.status || '',
      favicon: tab.favIconUrl || '',
      viewport: {
        width: tab.width,
        height: tab.height
      },
      interactive_elements: finalElements,
      interactive_count: finalElements.length,
      total_candidates: cappedElements.length
    }

    if (firstError && finalElements.length === 0) {
      payload.interactive_error = firstError
    }

    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'complete', payload)
    } else {
      ctx.sendResult(payload)
    }
  } catch (err) {
    const message = (err as Error).message || 'Page inventory failed'
    if (ctx.query.correlation_id) {
      ctx.sendAsyncResult(ctx.syncClient, ctx.query.id, ctx.query.correlation_id, 'error', null, message)
    } else {
      ctx.sendResult({
        error: 'page_inventory_failed',
        message
      })
    }
  }
})
