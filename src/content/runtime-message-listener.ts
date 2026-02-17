/**
 * Purpose: Handles content-script message relay between background and inject contexts.
 * Docs: docs/features/feature/interact-explore/index.md
 * Docs: docs/features/feature/query-dom/index.md
 */

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
  handleLinkHealthQuery
} from './message-handlers'

/** Color themes for each toast state */
const TOAST_THEMES: Record<string, { bg: string; shadow: string }> = {
  trying: { bg: 'linear-gradient(135deg, #3b82f6 0%, #2563eb 100%)', shadow: 'rgba(59, 130, 246, 0.4)' },
  success: { bg: 'linear-gradient(135deg, #22c55e 0%, #16a34a 100%)', shadow: 'rgba(34, 197, 94, 0.4)' },
  warning: { bg: 'linear-gradient(135deg, #f59e0b 0%, #d97706 100%)', shadow: 'rgba(245, 158, 11, 0.4)' },
  error: { bg: 'linear-gradient(135deg, #ef4444 0%, #dc2626 100%)', shadow: 'rgba(239, 68, 68, 0.4)' },
  audio: { bg: 'linear-gradient(135deg, #f97316 0%, #ea580c 100%)', shadow: 'rgba(249, 115, 22, 0.5)' }
}

/** Pre-built CSS for toast animations — extracted to reduce function complexity */
// nosemgrep: missing-template-string-indicator
const TOAST_ANIMATION_CSS = [
  '@keyframes gasolineArrowBounceUp {',
  '  0%, 100% { transform: translateY(0); opacity: 1; }',
  '  50% { transform: translateY(-6px); opacity: 0.7; }',
  '}',
  '@keyframes gasolineToastPulse {',
  '  0%, 100% { box-shadow: 0 4px 20px var(--toast-shadow); }',
  '  50% { box-shadow: 0 8px 32px var(--toast-shadow-intense); }',
  '}',
  '.gasoline-toast-arrow {',
  '  display: inline-block; margin-left: 8px;',
  '  animation: gasolineArrowBounceUp 1.5s ease-in-out infinite;',
  '}',
  '.gasoline-toast-pulse { animation: gasolineToastPulse 2s ease-in-out infinite; }'
].join('\n')

/** Add animation keyframes to document */
function injectToastAnimationStyles(): void {
  if (document.getElementById('gasoline-toast-animations')) return
  const style = document.createElement('style')
  style.id = 'gasoline-toast-animations'
  style.textContent = TOAST_ANIMATION_CSS
  document.head.appendChild(style)
}

/** Truncate text to maxLen characters with ellipsis */
function truncateText(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text
  return text.slice(0, maxLen - 1) + '\u2026'
}

/**
 * Show a brief visual toast overlay for AI actions.
 * Supports color-coded states and structured content with truncation.
 * For audio-related toasts, adds animated arrow pointing to extension icon.
 */
// #lizard forgives
function showActionToast(
  text: string,
  detail?: string,
  state: 'trying' | 'success' | 'warning' | 'error' | 'audio' = 'trying',
  durationMs = 3000
): void {
  // Remove existing toast
  const existing = document.getElementById('gasoline-action-toast')
  if (existing) existing.remove()

  // Inject animation styles once
  injectToastAnimationStyles()

  const theme = TOAST_THEMES[state] ?? TOAST_THEMES.trying!
  const isAudioPrompt =
    state === 'audio' || (detail && detail.toLowerCase().includes('audio') && detail.toLowerCase().includes('click'))

  const arrowChar = '↑'

  const toast = document.createElement('div')
  toast.id = 'gasoline-action-toast'
  if (isAudioPrompt) {
    toast.className = 'gasoline-toast-pulse'
  }

  // Add gasoline icon for audio/extension-click prompts
  if (isAudioPrompt) {
    const icon = document.createElement('img')
    icon.src = chrome.runtime.getURL('icons/icon-48.png')
    Object.assign(icon.style, {
      width: '20px',
      height: '20px',
      marginRight: '8px',
      flexShrink: '0'
    })
    toast.appendChild(icon)
  }

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

  // Add animated arrow for audio prompts (↑ pointing to extension toolbar)
  if (isAudioPrompt) {
    const arrow = document.createElement('span')
    arrow.className = 'gasoline-toast-arrow'
    arrow.textContent = arrowChar
    Object.assign(arrow.style, {
      fontSize: '16px',
      fontWeight: '700',
      marginLeft: '12px',
      display: 'inline-block'
    })
    toast.appendChild(arrow)
  }

  Object.assign(toast.style, {
    position: 'fixed',
    top: '16px',
    right: isAudioPrompt ? '80px' : 'auto',
    left: isAudioPrompt ? 'auto' : '50%',
    transform: isAudioPrompt ? 'none' : 'translateX(-50%)',
    padding: isAudioPrompt ? '12px 24px' : '8px 20px',
    background: theme.bg,
    color: '#fff',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    fontSize: isAudioPrompt ? '14px' : '13px',
    fontWeight: isAudioPrompt ? '600' : '400',
    borderRadius: '8px',
    boxShadow: `0 4px 20px ${theme.shadow}`,
    zIndex: '2147483647',
    pointerEvents: 'none',
    opacity: '0',
    transition: 'opacity 0.2s ease-in',
    maxWidth: isAudioPrompt ? '320px' : '500px',
    whiteSpace: isAudioPrompt ? 'normal' : ('nowrap' as const),
    overflow: isAudioPrompt ? 'visible' : 'hidden',
    display: 'flex',
    alignItems: 'center',
    gap: '0',
    '--toast-shadow': theme.shadow,
    '--toast-shadow-intense': theme.shadow.replace('0.4)', '0.7)')
  } as Record<string, string>)

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

/** Active Escape key listener reference for subtitle dismiss */
let subtitleEscapeHandler: ((e: KeyboardEvent) => void) | null = null

/** Fade out a DOM element and remove it after transition completes */
// #lizard forgives
function fadeOutAndRemove(elementId: string, delayMs: number): void {
  const el = document.getElementById(elementId)
  if (!el) return
  el.style.opacity = '0'
  setTimeout(() => el.remove(), delayMs)
}

/** Detach the active Escape key listener if one exists */
function detachEscapeListener(): void {
  if (!subtitleEscapeHandler) return
  document.removeEventListener('keydown', subtitleEscapeHandler)
  subtitleEscapeHandler = null
}

/**
 * Remove the subtitle element, clean up Escape listener.
 */
function clearSubtitle(): void {
  fadeOutAndRemove('gasoline-subtitle', 200)
  detachEscapeListener()
}

/**
 * Show or update a persistent subtitle bar at the bottom of the viewport.
 * Empty text clears the subtitle. Includes a hover close button and
 * Escape key listener for dismissal.
 */
function showSubtitle(text: string): void {
  const ELEMENT_ID = 'gasoline-subtitle'
  const CLOSE_BTN_ID = 'gasoline-subtitle-close'

  if (!text) {
    clearSubtitle()
    return
  }

  let bar = document.getElementById(ELEMENT_ID)
  if (!bar) {
    bar = document.createElement('div')
    bar.id = ELEMENT_ID
    Object.assign(bar.style, {
      position: 'fixed',
      bottom: '24px',
      left: '50%',
      transform: 'translateX(-50%)',
      width: 'auto',
      maxWidth: '80%',
      padding: '12px 20px',
      background: 'rgba(0, 0, 0, 0.85)',
      color: '#fff',
      fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
      fontSize: '16px',
      lineHeight: '1.4',
      textAlign: 'center',
      borderRadius: '4px',
      zIndex: '2147483646',
      pointerEvents: 'auto',
      opacity: '0',
      transition: 'opacity 0.2s ease-in',
      maxHeight: '4.2em', // ~3 lines
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      boxSizing: 'border-box'
    })

    // Close button — visible only on hover over the subtitle bar
    const closeBtn = document.createElement('button')
    closeBtn.id = CLOSE_BTN_ID
    closeBtn.textContent = '\u00d7' // multiplication sign (x)
    Object.assign(closeBtn.style, {
      position: 'absolute',
      top: '-6px',
      right: '-6px',
      width: '16px',
      height: '16px',
      padding: '0',
      margin: '0',
      border: 'none',
      borderRadius: '50%',
      background: 'rgba(255, 255, 255, 0.25)',
      color: '#fff',
      fontSize: '12px',
      lineHeight: '16px',
      textAlign: 'center',
      cursor: 'pointer',
      pointerEvents: 'auto',
      opacity: '0',
      transition: 'opacity 0.15s ease-in',
      fontFamily: 'sans-serif'
    })
    closeBtn.addEventListener('click', (e: MouseEvent) => {
      e.stopPropagation()
      clearSubtitle()
    })
    bar.appendChild(closeBtn)

    // Show close button on hover
    bar.addEventListener('mouseenter', () => {
      const btn = document.getElementById(CLOSE_BTN_ID)
      if (btn) btn.style.opacity = '1'
    })
    bar.addEventListener('mouseleave', () => {
      const btn = document.getElementById(CLOSE_BTN_ID)
      if (btn) btn.style.opacity = '0'
    })

    const target = document.body || document.documentElement
    if (!target) return
    target.appendChild(bar)
  }

  // Update text content while preserving the close button
  const closeBtn = document.getElementById(CLOSE_BTN_ID)
  // Set text on bar, then re-append close button so it stays on top
  bar.textContent = text
  if (closeBtn) {
    bar.appendChild(closeBtn)
  }

  // Register Escape key listener (replace any existing one)
  if (subtitleEscapeHandler) {
    document.removeEventListener('keydown', subtitleEscapeHandler)
  }
  subtitleEscapeHandler = (e: KeyboardEvent) => {
    if (e.key === 'Escape') {
      clearSubtitle()
    }
  }
  document.addEventListener('keydown', subtitleEscapeHandler)

  // Force reflow so the browser registers opacity:0, then set to 1
  // for the CSS transition. No timer needed — avoids rAF (paused in
  // background tabs) and setTimeout (throttled to 1s in background tabs).
  void bar.offsetHeight
  bar.style.opacity = '1'
}

/**
 * Show or hide a recording watermark (Gasoline flame icon) in the bottom-right corner.
 * The icon renders at 64x64px with 50% opacity, captured in the tab video.
 */
function toggleRecordingWatermark(visible: boolean): void {
  const ELEMENT_ID = 'gasoline-recording-watermark'

  if (!visible) {
    const existing = document.getElementById(ELEMENT_ID)
    if (existing) {
      existing.style.opacity = '0'
      setTimeout(() => existing.remove(), 300)
    }
    return
  }

  // Don't create a duplicate
  if (document.getElementById(ELEMENT_ID)) return

  const container = document.createElement('div')
  container.id = ELEMENT_ID
  Object.assign(container.style, {
    position: 'fixed',
    bottom: '16px',
    right: '16px',
    width: '64px',
    height: '64px',
    opacity: '0',
    transition: 'opacity 0.3s ease-in',
    zIndex: '2147483645',
    pointerEvents: 'none'
  })

  const img = document.createElement('img')
  img.src = chrome.runtime.getURL('icons/icon.svg')
  Object.assign(img.style, { width: '100%', height: '100%', opacity: '0.5' })
  container.appendChild(img)

  const target = document.body || document.documentElement
  if (!target) return
  target.appendChild(container)

  // Trigger reflow then fade in
  void container.offsetHeight
  container.style.opacity = '1'
}

/**
 * Initialize runtime message listener
 * Listens for messages from background (feature toggles and pilot commands)
 */
export function initRuntimeMessageListener(): void {
  // Load overlay toggle states from storage
  chrome.storage.local.get(
    ['actionToastsEnabled', 'subtitlesEnabled'],
    (result: Record<string, boolean | undefined>) => {
      if (result.actionToastsEnabled !== undefined) actionToastsEnabled = result.actionToastsEnabled
      if (result.subtitlesEnabled !== undefined) subtitlesEnabled = result.subtitlesEnabled
    }
  )

  /** Sync message handlers — return false (no async response needed) */
  type SyncMsg = ContentMessage & { enabled?: boolean; mode?: WebSocketCaptureMode; url?: string; params?: unknown }
  const syncHandlers: Record<string, (msg: SyncMsg) => false | void> = {
    GASOLINE_PING: () => {
      /* handled below via sendResponse */
    },
    GASOLINE_ACTION_TOAST: (msg) => {
      if (!actionToastsEnabled) return false
      const m = msg as {
        text?: string
        detail?: string
        state?: 'trying' | 'success' | 'warning' | 'error'
        duration_ms?: number
      }
      if (m.text) showActionToast(m.text, m.detail, m.state || 'trying', m.duration_ms)
      return false
    },
    GASOLINE_RECORDING_WATERMARK: (msg) => {
      toggleRecordingWatermark((msg as { visible?: boolean }).visible ?? false)
      return false
    },
    GASOLINE_SUBTITLE: (msg) => {
      if (!subtitlesEnabled) return false
      showSubtitle((msg as { text?: string }).text ?? '')
      return false
    },
    setActionToastsEnabled: (msg) => {
      actionToastsEnabled = (msg as { enabled: boolean }).enabled
      return false
    },
    setSubtitlesEnabled: (msg) => {
      subtitlesEnabled = (msg as { enabled: boolean }).enabled
      return false
    }
  }

  /** Delegated handlers — return boolean | undefined (some are async, returning true) */
  type DelegatedHandler = (msg: SyncMsg, sendResponse: (r?: unknown) => void) => boolean | undefined
  const delegatedHandlers: Record<string, DelegatedHandler> = {
    GASOLINE_DRAW_MODE_START: (msg, sr) => {
      const m = msg as { started_by?: string; session_name?: string; correlation_id?: string }
      import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
        .then((mod) => {
          const result = mod.activateDrawMode(m.started_by || 'user', m.session_name || '', m.correlation_id || '')
          sr(result)
        })
        .catch((e: Error) => sr({ error: 'draw_mode_load_failed', message: e.message }))
      return true
    },
    GASOLINE_DRAW_MODE_STOP: (_msg, sr) => {
      import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
        .then((mod) => {
          const result = mod.deactivateAndSendResults?.() || mod.deactivateDrawMode?.()
          sr(result || { status: 'stopped' })
        })
        .catch((e: Error) => sr({ error: 'draw_mode_load_failed', message: e.message }))
      return true
    },
    GASOLINE_GET_ANNOTATIONS: (_msg, sr) => {
      import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
        .then((mod) => {
          sr({ draw_mode_active: mod.isDrawModeActive?.() ?? false })
        })
        .catch(() => sr({ draw_mode_active: false }))
      return true
    },
    GASOLINE_HIGHLIGHT: (msg, sr) => {
      forwardHighlightMessage(msg as unknown as { params: { selector: string; duration_ms?: number } })
        .then((r) => sr(r))
        .catch((e: Error) => sr({ success: false, error: e.message }))
      return true
    },
    GASOLINE_MANAGE_STATE: (msg, sr) => {
      handleStateCommand(msg.params as Record<string, unknown>)
        .then((r) => sr(r))
        .catch((e: Error) => sr({ error: e.message }))
      return true
    },
    GASOLINE_EXECUTE_JS: (msg, sr) =>
      handleExecuteJs((msg.params as { script?: string; timeout_ms?: number }) || {}, sr),
    GASOLINE_EXECUTE_QUERY: (msg, sr) => handleExecuteQuery((msg.params || {}) as Record<string, unknown>, sr),
    A11Y_QUERY: (msg, sr) => handleA11yQuery((msg.params || {}) as Record<string, unknown>, sr),
    DOM_QUERY: (msg, sr) => handleDomQuery((msg.params || {}) as Record<string, unknown>, sr),
    GET_NETWORK_WATERFALL: (_msg, sr) => handleGetNetworkWaterfall(sr),
    LINK_HEALTH_QUERY: (msg, sr) => handleLinkHealthQuery((msg as any).params || {}, sr)
  }

  chrome.runtime.onMessage.addListener(
    (
      message: SyncMsg,
      sender: chrome.runtime.MessageSender,
      sendResponse: (response?: unknown) => void
    ): boolean | undefined => {
      if (!isValidBackgroundSender(sender)) {
        console.warn('[Gasoline] Rejected message from untrusted sender:', sender.id)
        return false
      }

      // Ping is special: sync handler that needs sendResponse
      if (message.type === 'GASOLINE_PING') return handlePing(sendResponse)

      // Try sync handlers first
      const syncHandler = syncHandlers[message.type] // nosemgrep: unsafe-dynamic-method
      if (syncHandler) {
        syncHandler(message)
        return false
      }

      // Handle toggle messages (no dispatch needed, always runs)
      handleToggleMessage(message)

      // Try delegated handlers
      const delegated = delegatedHandlers[message.type] // nosemgrep: unsafe-dynamic-method
      if (delegated) return delegated(message, sendResponse)

      return undefined
    }
  )
}
