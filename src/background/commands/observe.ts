/**
 * Purpose: Handles extension background coordination and message routing.
 * Why: Centralizes extension coordination to reduce race conditions and split-brain state.
 * Docs: docs/features/feature/analyze-tool/index.md
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/observe/index.md
 */

// observe.ts — Command handlers for the observe MCP tool.
// Handles: screenshot, waterfall, page_info, tabs.

import { debugLog } from '../index.js'
import { getServerUrl } from '../state.js'
import { DebugCategory } from '../debug.js'
import { recordScreenshot } from '../state-manager.js'
import { domPrimitiveListInteractive } from '../dom-primitives-list-interactive.js'
import { registerCommand } from './registry.js'

// =============================================================================
// SCREENSHOT
// =============================================================================

const CDP_VERSION = '1.3'
const MAX_CAPTURE_HEIGHT = 16384 // Chrome max texture size

/**
 * Self-contained function injected via chrome.scripting.executeScript.
 * Temporarily expands scrollable containers so CDP captures full content.
 * Stores original styles in data attributes for restoration.
 */
function screenshotExpandContainers(): { expanded: number } {
  let count = 0
  function tryExpand(el: HTMLElement): void {
    const style = getComputedStyle(el)
    const oy = style.overflowY || ''
    if ((oy === 'auto' || oy === 'scroll' || oy === 'hidden') && el.scrollHeight > el.clientHeight + 1) {
      el.setAttribute('data-gasoline-fpx', JSON.stringify({
        o: el.style.overflow, h: el.style.height, m: el.style.maxHeight
      }))
      el.style.overflow = 'visible'
      el.style.height = 'auto'
      el.style.maxHeight = 'none'
      count++
    }
  }
  tryExpand(document.documentElement)
  tryExpand(document.body)
  const all = document.body.querySelectorAll('*')
  for (let i = 0; i < all.length; i++) {
    if (all[i] instanceof HTMLElement) tryExpand(all[i] as HTMLElement)
  }
  return { expanded: count }
}

/** Self-contained: restore containers after full-page capture. */
function screenshotRestoreContainers(): void {
  function tryRestore(el: HTMLElement): void {
    const raw = el.getAttribute('data-gasoline-fpx')
    if (!raw) return
    try {
      const s = JSON.parse(raw) as { o?: string; h?: string; m?: string }
      el.style.overflow = s.o || ''
      el.style.height = s.h || ''
      el.style.maxHeight = s.m || ''
    } catch { /* ignore parse errors */ }
    el.removeAttribute('data-gasoline-fpx')
  }
  tryRestore(document.documentElement)
  const all = document.querySelectorAll('[data-gasoline-fpx]')
  for (let i = 0; i < all.length; i++) {
    tryRestore(all[i] as HTMLElement)
  }
}

/** Post screenshot data to server for saving and query resolution. */
async function postScreenshot(
  dataUrl: string, pageUrl: string | undefined, queryId: string
): Promise<boolean> {
  try {
    const response = await fetch(`${getServerUrl()}/screenshots`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Gasoline-Client': 'gasoline-extension' },
      body: JSON.stringify({ data_url: dataUrl, url: pageUrl, query_id: queryId })
    })
    return response.ok
  } catch {
    return false
  }
}

registerCommand('screenshot', async (ctx) => {
  const format = ctx.params.format === 'png' ? 'png' : 'jpeg'
  const quality = typeof ctx.params.quality === 'number' ? ctx.params.quality : 80
  const fullPage = ctx.params.full_page === true

  try {
    const tab = await chrome.tabs.get(ctx.tabId)
    await chrome.tabs.update(ctx.tabId, { active: true })

    if (fullPage) {
      await captureFullPage(ctx, tab, format, quality)
      return
    }

    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
      format: format as 'jpeg' | 'png',
      quality
    })
    recordScreenshot(ctx.tabId)

    // POST to /screenshots with query_id — server saves file and resolves query directly
    const ok = await postScreenshot(dataUrl, tab.url, ctx.query.id)
    if (!ok) {
      ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
    }
    // No sendResult needed — server resolves the query via query_id
  } catch (err) {
    ctx.sendResult({
      error: 'screenshot_failed',
      message: (err as Error).message || 'Failed to capture screenshot'
    })
  }
})

/** Full-page screenshot via CDP with scrollable container expansion (#363). */
async function captureFullPage(
  ctx: { tabId: number; query: { id: string }; sendResult: (r: unknown) => void },
  tab: chrome.tabs.Tab,
  format: 'png' | 'jpeg',
  quality: number
): Promise<void> {
  // Step 1: Expand scrollable containers in the page
  await chrome.scripting.executeScript({
    target: { tabId: ctx.tabId },
    world: 'MAIN',
    func: screenshotExpandContainers
  })

  try {
    // Step 2: Attach CDP debugger
    await chrome.debugger.attach({ tabId: ctx.tabId }, CDP_VERSION)

    try {
      // Step 3: Get full content dimensions
      const metrics = await chrome.debugger.sendCommand(
        { tabId: ctx.tabId }, 'Page.getLayoutMetrics', {}
      ) as {
        cssContentSize?: { width: number; height: number }
        contentSize?: { width: number; height: number }
      }

      const contentSize = metrics.cssContentSize || metrics.contentSize || { width: 1280, height: 720 }
      const captureWidth = Math.ceil(contentSize.width)
      const captureHeight = Math.min(Math.ceil(contentSize.height), MAX_CAPTURE_HEIGHT)

      // Step 4: Override viewport to full content size
      await chrome.debugger.sendCommand(
        { tabId: ctx.tabId }, 'Emulation.setDeviceMetricsOverride',
        { width: captureWidth, height: captureHeight, deviceScaleFactor: 1, mobile: false }
      )

      // Brief pause for layout reflow after viewport resize
      await new Promise((r) => setTimeout(r, 150))

      // Step 5: Capture full-page screenshot via CDP
      const screenshotResult = await chrome.debugger.sendCommand(
        { tabId: ctx.tabId }, 'Page.captureScreenshot', {
          format,
          quality: format === 'jpeg' ? quality : undefined,
          clip: { x: 0, y: 0, width: captureWidth, height: captureHeight, scale: 1 }
        }
      ) as { data: string }

      // Step 6: Clear device metrics override
      await chrome.debugger.sendCommand(
        { tabId: ctx.tabId }, 'Emulation.clearDeviceMetricsOverride', {}
      )

      // Step 7: Build data URL and post to server
      const mimeType = format === 'png' ? 'image/png' : 'image/jpeg'
      const dataUrl = `data:${mimeType};base64,${screenshotResult.data}`
      recordScreenshot(ctx.tabId)

      const ok = await postScreenshot(dataUrl, tab.url, ctx.query.id)
      if (!ok) {
        ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
      }
    } finally {
      try { await chrome.debugger.detach({ tabId: ctx.tabId }) } catch { /* already detached */ }
    }
  } catch (err) {
    // CDP unavailable — fall back to regular captureVisibleTab with warning
    debugLog(DebugCategory.CAPTURE, 'Full-page CDP failed, falling back to viewport capture', {
      error: (err as Error).message
    })
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId!, {
      format: format as 'jpeg' | 'png',
      quality
    })
    recordScreenshot(ctx.tabId)
    const ok = await postScreenshot(dataUrl, tab.url, ctx.query.id)
    if (!ok) {
      ctx.sendResult({ error: 'screenshot_upload_failed', message: 'Server rejected screenshot' })
    }
  } finally {
    // Step 8: Always restore containers
    await chrome.scripting.executeScript({
      target: { tabId: ctx.tabId },
      world: 'MAIN',
      func: screenshotRestoreContainers
    }).catch(() => { /* best effort */ })
  }
}

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
    const interactiveResults = await chrome.scripting.executeScript({
      target: { tabId: ctx.tabId, allFrames: true },
      world: 'MAIN',
      func: domPrimitiveListInteractive,
      args: ['']
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
