// runtime-message-listener.ts — Message routing between background and content contexts.

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
import { SettingName } from '../lib/constants'
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
import { showActionToast } from './ui/toast'
import { showSubtitle, toggleRecordingWatermark } from './ui/subtitle'

// Toggle state caches — updated by forwarded setting messages from background
let actionToastsEnabled = true
let subtitlesEnabled = true

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
    [SettingName.ACTION_TOASTS]: (msg) => {
      actionToastsEnabled = (msg as { enabled: boolean }).enabled
      return false
    },
    [SettingName.SUBTITLES]: (msg) => {
      subtitlesEnabled = (msg as { enabled: boolean }).enabled
      return false
    }
  }

  /** Delegated handlers — return boolean | undefined (some are async, returning true) */
  type DelegatedHandler = (msg: SyncMsg, sendResponse: (r?: unknown) => void) => boolean | undefined
  const delegatedHandlers: Record<string, DelegatedHandler> = {
    GASOLINE_DRAW_MODE_START: (msg, sr) => {
      const m = msg as { started_by?: string; annot_session_name?: string; correlation_id?: string }
      import(/* webpackIgnore: true */ chrome.runtime.getURL('content/draw-mode.js'))
        .then((mod) => {
          const result = mod.activateDrawMode(m.started_by || 'user', m.annot_session_name || '', m.correlation_id || '')
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
