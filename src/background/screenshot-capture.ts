/**
 * Purpose: Captures visible tab screenshots with rate limiting and uploads to the daemon server.
 * Why: Isolates screenshot capture (rate-check, tab capture, server upload) from unrelated log/badge logic.
 * Docs: docs/features/feature/browser-extension-enhancement/index.md
 */

import type { LogEntry } from '../types/index.js'
import { getRequestHeaders } from './server.js'
import { errorMessage } from '../lib/error-utils.js'
import { captureVisibleTabSafe } from './tab-state.js'

/**
 * Capture a screenshot of the visible tab area
 */
export async function captureScreenshot(
  tabId: number,
  serverUrl: string,
  relatedErrorId: string | null,
  errorType: string | null,
  canTakeScreenshotFn: (tabId: number) => { allowed: boolean; reason?: string; nextAllowedIn?: number | null },
  recordScreenshotFn: (tabId: number) => void,
  debugLogFn?: (category: string, message: string, data?: unknown) => void
): Promise<{
  success: boolean
  entry?: LogEntry
  error?: string
  nextAllowedIn?: number | null
}> {
  const rateCheck = canTakeScreenshotFn(tabId)
  if (!rateCheck.allowed) {
    if (debugLogFn) {
      debugLogFn('capture', `Screenshot rate limited: ${rateCheck.reason}`, {
        tabId,
        nextAllowedIn: rateCheck.nextAllowedIn
      })
    }
    return {
      success: false,
      error: `Rate limited: ${rateCheck.reason}`,
      nextAllowedIn: rateCheck.nextAllowedIn
    }
  }

  try {
    const tab = await chrome.tabs.get(tabId)
    const dataUrl = await captureVisibleTabSafe(tabId, tab.windowId, {
      format: 'jpeg',
      quality: 80
    })

    recordScreenshotFn(tabId)

    const response = await fetch(`${serverUrl}/screenshots`, {
      method: 'POST',
      headers: getRequestHeaders(),
      body: JSON.stringify({
        data_url: dataUrl,
        url: tab.url,
        correlation_id: relatedErrorId || ''
      })
    })

    if (!response.ok) {
      throw new Error(`Failed to upload screenshot: server returned HTTP ${response.status} ${response.statusText}`)
    }

    const result = (await response.json()) as { filename: string }

    const screenshotEntry: LogEntry = {
      ts: new Date().toISOString(),
      type: 'screenshot',
      level: 'info',
      url: tab.url,
      _enrichments: ['screenshot'],
      screenshotFile: result.filename,
      trigger: relatedErrorId ? 'error' : 'manual',
      ...(relatedErrorId ? { relatedErrorId } : {})
    } as LogEntry

    if (debugLogFn) {
      debugLogFn('capture', `Screenshot saved: ${result.filename}`, {
        trigger: relatedErrorId ? 'error' : 'manual',
        relatedErrorId
      })
    }

    return { success: true, entry: screenshotEntry }
  } catch (error) {
    if (debugLogFn) {
      debugLogFn('error', 'Screenshot capture failed', { error: errorMessage(error) })
    }
    return { success: false, error: errorMessage(error) }
  }
}
