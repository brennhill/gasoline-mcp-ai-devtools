// @ts-nocheck
/**
 * @fileoverview Exception and unhandled rejection capture.
 * Monkey-patches window.onerror and listens for unhandledrejection events,
 * enriching errors with AI context before posting via bridge.
 */

import { postLog } from './bridge.js'
import { enrichErrorWithAiContext } from './ai-context.js'

// Exception capture state
let originalOnerror = null
let unhandledrejectionHandler = null

/**
 * Install exception capture
 */
export function installExceptionCapture() {
  originalOnerror = window.onerror

  window.onerror = function (message, filename, lineno, colno, error) {
    const entry = {
      level: 'error',
      type: 'exception',
      message: String(message),
      source: filename ? `${filename}:${lineno || 0}` : '',
      filename: filename || '',
      lineno: lineno || 0,
      colno: colno || 0,
      stack: error?.stack || '',
    }

    // Enrich with AI context then post (async, fire-and-forget)
    ;(async () => {
      try {
        const enriched = await enrichErrorWithAiContext(entry)
        postLog(enriched)
      } catch {
        postLog(entry)
      }
    })().catch((err) => {
      console.error('[Gasoline] Exception enrichment error:', err)
      // Fallback: ensure entry is logged even if something fails
      try {
        postLog(entry)
      } catch (postErr) {
        console.error('[Gasoline] Failed to log entry:', postErr)
      }
    })

    // Call original if exists
    if (originalOnerror) {
      return originalOnerror(message, filename, lineno, colno, error)
    }
    return false
  }

  // Unhandled promise rejections
  unhandledrejectionHandler = function (event) {
    const error = event.reason
    let message = ''
    let stack = ''

    if (error instanceof Error) {
      message = error.message
      stack = error.stack || ''
    } else if (typeof error === 'string') {
      message = error
    } else {
      message = String(error)
    }

    const entry = {
      level: 'error',
      type: 'exception',
      message: `Unhandled Promise Rejection: ${message}`,
      stack,
    }

    // Enrich with AI context then post (async, fire-and-forget)
    ;(async () => {
      try {
        const enriched = await enrichErrorWithAiContext(entry)
        postLog(enriched)
      } catch {
        postLog(entry)
      }
    })().catch((err) => {
      console.error('[Gasoline] Exception enrichment error:', err)
      // Fallback: ensure entry is logged even if something fails
      try {
        postLog(entry)
      } catch (postErr) {
        console.error('[Gasoline] Failed to log entry:', postErr)
      }
    })
  }

  window.addEventListener('unhandledrejection', unhandledrejectionHandler)
}

/**
 * Uninstall exception capture
 */
export function uninstallExceptionCapture() {
  if (originalOnerror !== null) {
    window.onerror = originalOnerror
    originalOnerror = null
  }

  if (unhandledrejectionHandler) {
    window.removeEventListener('unhandledrejection', unhandledrejectionHandler)
    unhandledrejectionHandler = null
  }
}
