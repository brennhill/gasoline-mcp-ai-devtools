/**
 * @fileoverview Runtime Message Listener Module
 * Handles chrome.runtime messages from background script
 */

import type { ContentMessage, WebSocketCaptureMode } from '../types'
import {
  isValidBackgroundSender,
  handlePing,
  handleToggleMessage,
  forwardHighlightMessage,
  handleStateCommand,
  handleExecuteJs,
  handleExecuteQuery,
  handleA11yQuery,
  handleDomQuery,
  handleGetNetworkWaterfall,
} from './message-handlers'

/** Color themes for each toast state */
const TOAST_THEMES: Record<string, { bg: string; shadow: string }> = {
  trying:  { bg: 'linear-gradient(135deg, #ff6b00 0%, #ff9500 100%)', shadow: 'rgba(255, 107, 0, 0.4)' },
  success: { bg: 'linear-gradient(135deg, #22c55e 0%, #16a34a 100%)', shadow: 'rgba(34, 197, 94, 0.4)' },
  warning: { bg: 'linear-gradient(135deg, #f59e0b 0%, #d97706 100%)', shadow: 'rgba(245, 158, 11, 0.4)' },
  error:   { bg: 'linear-gradient(135deg, #ef4444 0%, #dc2626 100%)', shadow: 'rgba(239, 68, 68, 0.4)' },
}

/** Truncate text to maxLen characters with ellipsis */
function truncateText(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen - 1) + '\u2026'
}

/**
 * Show a brief visual toast overlay for AI actions.
 * Supports color-coded states and structured content with truncation.
 */
function showActionToast(
  text: string,
  detail?: string,
  state: 'trying' | 'success' | 'warning' | 'error' = 'trying',
  durationMs = 3000,
): void {
  // Remove existing toast
  const existing = document.getElementById('gasoline-action-toast')
  if (existing) existing.remove()

  const theme = TOAST_THEMES[state] ?? TOAST_THEMES.trying!

  const toast = document.createElement('div')
  toast.id = 'gasoline-action-toast'

  // Build content: label + truncated detail
  const label = document.createElement('span')
  label.textContent = truncateText(text, 30)
  Object.assign(label.style, { fontWeight: '700' })
  toast.appendChild(label)

  if (detail) {
    const sep = document.createElement('span')
    sep.textContent = '  '
    Object.assign(sep.style, { opacity: '0.6', margin: '0 4px' })
    toast.appendChild(sep)

    const det = document.createElement('span')
    det.textContent = truncateText(detail, 50)
    Object.assign(det.style, { fontWeight: '400', opacity: '0.9' })
    toast.appendChild(det)
  }

  Object.assign(toast.style, {
    position: 'fixed',
    top: '16px',
    left: '50%',
    transform: 'translateX(-50%)',
    padding: '8px 20px',
    background: theme.bg,
    color: '#fff',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: '13px',
    borderRadius: '8px',
    boxShadow: `0 4px 20px ${theme.shadow}`,
    zIndex: '2147483647',
    pointerEvents: 'none',
    opacity: '0',
    transition: 'opacity 0.2s ease-in',
    maxWidth: '500px',
    whiteSpace: 'nowrap' as const,
    overflow: 'hidden',
    display: 'flex',
    alignItems: 'center',
    gap: '0',
  })

  const target = document.body || document.documentElement
  if (!target) return
  target.appendChild(toast)

  // Fade in
  requestAnimationFrame(() => {
    toast.style.opacity = '1'
  })

  // Fade out and remove
  setTimeout(() => {
    toast.style.opacity = '0'
    setTimeout(() => toast.remove(), 300)
  }, durationMs)
}

// Toggle state caches — updated by forwarded setting messages from background
let actionToastsEnabled = true
let subtitlesEnabled = true

/**
 * Show or update a persistent subtitle bar at the bottom of the viewport.
 * Empty text clears the subtitle.
 */
function showSubtitle(text: string): void {
  const ELEMENT_ID = 'gasoline-subtitle'

  if (!text) {
    // Clear: remove existing element
    const existing = document.getElementById(ELEMENT_ID)
    if (existing) {
      existing.style.opacity = '0'
      setTimeout(() => existing.remove(), 200)
    }
    return
  }

  let bar = document.getElementById(ELEMENT_ID)
  if (!bar) {
    bar = document.createElement('div')
    bar.id = ELEMENT_ID
    Object.assign(bar.style, {
      position: 'fixed',
      bottom: '0',
      left: '0',
      width: '100%',
      padding: '12px 24px',
      background: 'rgba(0, 0, 0, 0.85)',
      color: '#fff',
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      fontSize: '16px',
      lineHeight: '1.4',
      zIndex: '2147483646',
      pointerEvents: 'none',
      opacity: '0',
      transition: 'opacity 0.2s ease-in',
      maxHeight: '4.2em',      // ~3 lines
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      boxSizing: 'border-box',
    })

    const target = document.body || document.documentElement
    if (!target) return
    target.appendChild(bar)
  }

  bar.textContent = text
  requestAnimationFrame(() => {
    if (bar) bar.style.opacity = '1'
  })
}

/**
 * Initialize runtime message listener
 * Listens for messages from background (feature toggles and pilot commands)
 */
export function initRuntimeMessageListener(): void {
  // Load overlay toggle states from storage
  chrome.storage.local.get(['actionToastsEnabled', 'subtitlesEnabled'], (result: Record<string, boolean | undefined>) => {
    if (result.actionToastsEnabled !== undefined) actionToastsEnabled = result.actionToastsEnabled
    if (result.subtitlesEnabled !== undefined) subtitlesEnabled = result.subtitlesEnabled
  })

  chrome.runtime.onMessage.addListener(
    (
      message: ContentMessage & { enabled?: boolean; mode?: WebSocketCaptureMode; url?: string; params?: unknown },
      sender: chrome.runtime.MessageSender,
      sendResponse: (response?: unknown) => void,
    ): boolean | undefined => {
      // SECURITY: Validate sender is from the extension background, not from page context
      if (!isValidBackgroundSender(sender)) {
        console.warn('[Gasoline] Rejected message from untrusted sender:', sender.id)
        return false
      }

      // Handle ping to check if content script is loaded
      if (message.type === 'GASOLINE_PING') {
        return handlePing(sendResponse)
      }

      // Show AI action toast overlay (gated by toggle)
      if (message.type === 'GASOLINE_ACTION_TOAST') {
        if (!actionToastsEnabled) return false
        const msg = message as { type: string; text?: string; detail?: string; state?: 'trying' | 'success' | 'warning' | 'error'; duration_ms?: number }
        if (msg.text) showActionToast(msg.text, msg.detail, msg.state || 'trying', msg.duration_ms)
        return false
      }

      // Show subtitle overlay (gated by toggle)
      if (message.type === 'GASOLINE_SUBTITLE') {
        if (!subtitlesEnabled) return false
        const msg = message as { type: string; text?: string }
        showSubtitle(msg.text ?? '')
        return false
      }

      // Handle overlay toggle updates from background
      if (message.type === 'setActionToastsEnabled') {
        actionToastsEnabled = (message as { type: string; enabled: boolean }).enabled
        return false
      }
      if (message.type === 'setSubtitlesEnabled') {
        subtitlesEnabled = (message as { type: string; enabled: boolean }).enabled
        return false
      }

      // Handle toggle messages
      handleToggleMessage(message)

      // Handle GASOLINE_HIGHLIGHT from background
      if (message.type === 'GASOLINE_HIGHLIGHT') {
        forwardHighlightMessage(message)
          .then((result) => {
            sendResponse(result)
          })
          .catch((err: Error) => {
            sendResponse({ success: false, error: err.message })
          })
        return true // Will respond asynchronously
      }

      // Handle state management commands from background
      if (message.type === 'GASOLINE_MANAGE_STATE') {
        // message.params contains action, state, include_url from the manage_state tool
        // handleStateCommand accepts params with optional action (StateAction), name, state, include_url
        handleStateCommand(message.params)
          .then((result) => sendResponse(result))
          .catch((err: Error) => sendResponse({ error: err.message }))
        return true // Keep channel open for async response
      }

      // Handle GASOLINE_EXECUTE_JS from background (direct pilot command)
      if (message.type === 'GASOLINE_EXECUTE_JS') {
        const params = (message.params as { script?: string; timeout_ms?: number }) || {}
        return handleExecuteJs(params, sendResponse)
      }

      // Handle GASOLINE_EXECUTE_QUERY from background (polling system)
      if (message.type === 'GASOLINE_EXECUTE_QUERY') {
        return handleExecuteQuery(message.params || {}, sendResponse)
      }

      // Handle A11Y_QUERY from background (run accessibility audit in page context)
      if (message.type === 'A11Y_QUERY') {
        return handleA11yQuery(message.params || {}, sendResponse)
      }

      // Handle DOM_QUERY from background (execute CSS selector query in page context)
      if (message.type === 'DOM_QUERY') {
        return handleDomQuery(message.params || {}, sendResponse)
      }

      // Handle GET_NETWORK_WATERFALL from background (collect PerformanceResourceTiming data)
      if (message.type === 'GET_NETWORK_WATERFALL') {
        return handleGetNetworkWaterfall(sendResponse)
      }

      return undefined
    },
  )
}
