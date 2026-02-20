// observe.ts — Command handlers for the observe MCP tool.
// Handles: screenshot, waterfall, page_info, tabs.

import * as index from '../index'
import { DebugCategory } from '../debug'
import { canTakeScreenshot, recordScreenshot } from '../state-manager'
import { registerCommand } from './registry'

const { debugLog } = index

// =============================================================================
// SCREENSHOT
// =============================================================================

registerCommand('screenshot', async (ctx) => {
  try {
    const rateCheck = canTakeScreenshot(ctx.tabId)
    if (!rateCheck.allowed) {
      ctx.sendResult({
        error: `Rate limited: ${rateCheck.reason}`,
        ...(rateCheck.nextAllowedIn != null ? { next_allowed_in: rateCheck.nextAllowedIn } : {})
      })
      return
    }

    const tab = await chrome.tabs.get(ctx.tabId)
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
      format: 'jpeg',
      quality: 80
    })
    recordScreenshot(ctx.tabId)

    // POST to /screenshots with query_id — server saves file and resolves query directly
    const response = await fetch(`${index.serverUrl}/screenshots`, {
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
